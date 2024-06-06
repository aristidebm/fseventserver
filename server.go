package fseventserver

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"io/fs"
	"math"

	"log/slog"
	"mime"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/gabriel-vasile/mimetype"
	"github.com/gobwas/glob"
)

type Server struct {
	Root     string
	Handler  Handler
	MaxDepth int
	// whether to stop watching on errors
	// if set true, files that cannot be watched are skipped
	// otherwise the server will stop
	Skip         bool
	ErrorHandler ErrorHandler
	// glog pattern can be provided
	IgnoreList         []string
	Logger             *slog.Logger
	compiledIgnoreList []glob.Glob
	watcher            *fsnotify.Watcher
}

type Handler interface {
	ServeFSEvent(ctx context.Context) error
}

type Middleware func(Handler) Handler

type ErrorHandler interface {
	HandleError(err error)
}

type Request struct {
	ID           int64
	Path         string
	Size         int64
	IsDir        bool
	Mode         fs.FileMode
	Mimetype     MIME
	Action       fsnotify.Op
	LastModified int64
	Date         time.Time
	Timeout      time.Duration
}

type MIME struct {
	mime      string
	extension string
}

func (self MIME) Extension() string {
	return self.extension
}

func (self MIME) String() string {
	return self.mime
}

func (self MIME) Is(mType string) bool {
	// Parsing is needed because some detected MIME types contain parameters
	// that need to be stripped for the comparison.
	mType, _, _ = mime.ParseMediaType(mType)
	found, _, _ := mime.ParseMediaType(self.mime)

	if mType == found {
		return true
	}

	return false
}

func ListenAndServe(root string, handler Handler) error {
	server := &Server{Root: root, Handler: handler, Logger: makeLogger()}
	if err := server.ListenAndServe(); err != nil {
		return err
	}
	return nil
}

func (self *Server) ListenAndServe() error {
	var err error
	var root = self.Root

	if root == "" {
		if root, err = defaultRoot(); err != nil {
			return fmt.Errorf("%w %s %w", ErrListerningDirectory, root, err)
		}
	}

	if root, err = cleanPath(root); err != nil {
		return fmt.Errorf("%w %s %w", ErrListerningDirectory, root, err)
	}

	var buf bytes.Buffer

	if err := self.walk(root, &buf); err != nil {
		return fmt.Errorf("%w %s %w", ErrListerningDirectory, root, err)
	}

	return self.watch(&buf)
}

func defaultRoot() (string, error) {
	root, err := os.Getwd()
	if err != nil {
		return "", err
	}
	return root, nil
}

func cleanPath(path string) (string, error) {
	var err error
	if strings.HasPrefix(path, "~") {
		path, err = expandUser(path)
		if err != nil {
			return "", err
		}
	}
	return path, nil
}

func (self *Server) Close() error {
	if self.watcher != nil {
		return self.watcher.Close()
	}
	return nil
}

func (self *Server) walk(root string, wrt io.Writer) error {
	err := filepath.WalkDir(root, func(path string, info fs.DirEntry, err error) error {

		if err != nil {
			return err
		}

		// currently we are only interested in events
		// occured in a directory
		if !info.IsDir() {
			return nil
		}

		value, err := self.shouldIgnore(path)
		if err != nil {
			return err
		}

		if value {
			return nil
		}

		maxDepth := self.MaxDepth
		if maxDepth == 0 {
			// use recursive watch by default
			maxDepth = -1
		}

		if maxDepth > 0 {
			depth := self.computeDepth(path, root)
			if depth < 0 || depth > self.MaxDepth {
				return nil
			}
		}

		fmt.Fprintln(wrt, path)
		return nil
	})

	if err != nil {
		return err
	}

	return nil
}

func (self *Server) computeDepth(path string, root string) int {
	depth := 1
	current := path

	for {
		// we reach the top of the hierarchy
		if current == "/" || current == root {
			break
		}
		current = filepath.Dir(current)
		depth++
	}

	if current != root {
		return int(math.Inf(1))
	}

	return depth
}

func (self *Server) shouldIgnore(path string) (bool, error) {
	ignoreList := self.IgnoreList
	ignoreList = append(ignoreList, ".git")

	if len(self.compiledIgnoreList) == 0 {
		for _, item := range ignoreList {
			pattern, err := glob.Compile(item)
			if err != nil {
				return false, err
			}
			self.compiledIgnoreList = append(self.compiledIgnoreList, pattern)
		}
	}

	for _, pattern := range self.compiledIgnoreList {
		if pattern.Match(path) {
			return true, nil
		}
	}
	return false, nil
}

func (self *Server) watch(red io.Reader) error {
	var err error

	// defers wather initialization here because
	// fsnotify.NewWatcher() combines watcher creation
	// and listening
	self.watcher, err = fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("%w %w", ErrWatchingFile, err)
	}
	defer self.watcher.Close()

	sc := bufio.NewScanner(red)
	for sc.Scan() {
		filename := sc.Text()
		if err = self.watcher.Add(filename); err != nil {
			if self.Skip {
				continue
			}
			return fmt.Errorf("%w %s %w", ErrWatchingFile, filename, err)
		}
	}

	if err := sc.Err(); err != nil {
		return fmt.Errorf("%w %w", ErrWatchingFile, err)
	}

	self.printWatchList(self.watcher.WatchList())

	for {
		select {
		case event, ok := <-self.watcher.Events:
			// the events channel is closed, we cannot receive
			// the event anymore, end the goroutine
			if !ok {
				return nil
			}

			// NOTE: file creating is followed by multiple writes events
			// we need to ignore them, since we are not interested in them
			if event.Op.Has(fsnotify.Write) {
				continue
			}

			ctx, cancel, err := self.makeContext(event)

			// FIXME: don't do that, otherwise the the main goroutine will deadlock itself
			// by trying to write in an unbuffered channel that reader counter part in the same goroutine
			// i am thinking of using a dedicated channel for error handling, is it good idea ?
			if err != nil {
				self.watcher.Errors <- err
				continue
			}

			req := ctx.Value("request")
			self.Logger.Info(fmt.Sprintf("sending request %+v", req))

			handle := func() {

				defer cancel()

				handler := self.Handler
				if handler == nil {
					handler = defaultServeMux
				}

				if err := handler.ServeFSEvent(ctx); err != nil {
					self.watcher.Errors <- err
				}
			}
			go handle()
		case err, ok := <-self.watcher.Errors:
			// the error channel is closed, we cannot receive
			// the errors anymore, end the goroutine
			if !ok {
				return nil
			}
			errHandler := self.ErrorHandler
			if errHandler == nil {
				errHandler = defaultErrorHandler
			}
			errHandler.HandleError(err)
		}
	}
}

func (self *Server) makeContext(evt fsnotify.Event) (context.Context, context.CancelFunc, error) {
	req, err := self.makeRequest(evt)
	if err != nil {
		return nil, nil, err
	}
	ctx := context.Background()
	ctx = context.WithValue(ctx, "request", req)
	ctx, cancel := context.WithCancel(ctx)
	return ctx, cancel, nil
}

func (self *Server) makeRequest(evt fsnotify.Event) (*Request, error) {
	var err error
	var fileStat fs.FileInfo

	if !evt.Op.Has(fsnotify.Rename) && !evt.Op.Has(fsnotify.Remove) {
		fileStat, err = os.Stat(evt.Name)
		if err != nil {
			return nil, err
		}
	}

	ext := filepath.Ext(evt.Name)
	mType := MIME{extension: ext, mime: mime.TypeByExtension(ext)}
	if fileStat != nil && !fileStat.IsDir() {
		// perform mimetype negociation using `magic number`(https://en.wikipedia.org/wiki/Magic_number_(programming)#Magic_numbers_in_files)
		// since it is more accurate
		if value, err := mimetype.DetectFile(evt.Name); err == nil {
			mType.extension = value.Extension()
			mType.mime = value.String()
		}
		// text/plain is a broad mimetype, sometimes markdown, html, ...
		// are all referenced as text files by magic number based mimetype
		// negociator, because usually text files do not use a magic number
		// so that they can be indentified uniquely so fallback do detection by extension
		if mType.Is("text/plain") && mType.extension != "" {
			mType.mime = mime.TypeByExtension(mType.extension)
		}
	}

	req := &Request{
		Path:         evt.Name,
		Mimetype:     mType,
		Action:       evt.Op,
		Date:         time.Now(),
		LastModified: time.Now().UnixNano(),
	}

	if fileStat != nil {
		req.Size = fileStat.Size()
		req.IsDir = fileStat.IsDir()
		req.Mode = fileStat.Mode()
		req.LastModified = fileStat.ModTime().UnixNano()
	}

	return req, nil
}

func (self *Server) printWatchList(items []string) {
	msg := make([]string, 0)
	for _, item := range items {
		msg = append(msg, fmt.Sprintf("-> %s", item))
	}
	self.Logger.Info(strings.Join(msg, "\n"))
}

func makeLogger() *slog.Logger {
	return slog.Default()
}
