package fseventserver

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log"
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
	// whether to skip watching errors
	// if set true, files that cannot be watched are skipped
	// otherwise the server will stop
	Skip         bool
	ErrorHandler ErrorHandler
	// glog pattern can be provided
	IgnoreList         []string
	Logger             Logger
	compiledIgnoreList []glob.Glob
	watcher            *fsnotify.Watcher
}

type Handler interface {
	ServeFSEvent(ctx context.Context) error
}

type Logger interface {
	Trace(msg string)
	Debug(msg string)
	Info(msg string)
	Warn(msg string)
	Error(msg string)
	SetOuput(out io.Writer)
}

type ErrorHandler interface {
	HandleError(err error)
}

type Request struct {
	Path         string
	Size         int64
	IsDir        bool
	Mode         fs.FileMode
	Mimetype     *mimetype.MIME
	Action       fsnotify.Op
	LastModified time.Time
	Date         time.Time
	Hostname     string
	Timeout      time.Duration
}

func ListenAndServe(root string, handler Handler) error {
	var err error

	server, err := NewServer(root, handler, -1, nil, nil, nil)
	if err != nil {
		return err
	}

	if err = server.ListenAndServe(); err != nil {
		return err
	}

	return nil
}

func (self *Server) ListenAndServe() error {
	var err error
	var root = self.Root

	if root == "" {
		root, err = os.Getwd()
		if err != nil {
			return err
		}
	}

	tilde := "~"
	if strings.HasPrefix(root, tilde) {
		root, err = expandUser(root)
		if err != nil {
			return err
		}
	}

	var buf bytes.Buffer

	if err := self.walk(root, &buf); err != nil {
		return err
	}

	if err = self.watch(&buf); err != nil {
		return err
	}

	return nil
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

		if self.MaxDepth > 0 {
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
	depth := 0
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
		return -1
	}

	return depth
}

func (self *Server) shouldIgnore(path string) (bool, error) {
	if len(self.compiledIgnoreList) == 0 {
		for _, item := range self.IgnoreList[:] {
			pattern, err := glob.Compile(item)
			if err != nil {
				return false, err
			}
			self.compiledIgnoreList = append(self.compiledIgnoreList, pattern)
		}
	}

	for _, pattern := range self.compiledIgnoreList[:] {
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
		return err
	}
	defer self.watcher.Close()

	sc := bufio.NewScanner(red)
	for sc.Scan() {
		if err = self.watcher.Add(sc.Text()); err != nil {
			if self.Skip {
				continue
			}
			return err
		}
	}

	log.Print(self.watcher.WatchList())

	for {
		select {
		case event, ok := <-self.watcher.Events:
			// the events channel is closed, we cannot receive
			// the event anymore, end the goroutine
			if !ok {
				return errors.New("")
			}

			ctx, cancel, err := self.makeContext(event)
			if err != nil {
				self.watcher.Errors <- err
				continue
			}

			handle := func() {
				defer cancel()
				if err := self.Handler.ServeFSEvent(ctx); err != nil {
					self.watcher.Errors <- err
				}
			}
			go handle()
		case err, ok := <-self.watcher.Errors:
			// the error channel is closed, we cannot receive
			// the errors anymore, end the goroutine
			if !ok {
				return errors.New("")
			}
			self.ErrorHandler.HandleError(err)
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

	fileStat, err := os.Stat(evt.Name)
	if err != nil {
		return nil, err
	}

	hostname, err := os.Hostname()
	if err != nil {
		return nil, err
	}

	var mType *mimetype.MIME
	if !fileStat.IsDir() {
		mType, err = mimetype.DetectFile(evt.Name)
		if err != nil {
			return nil, err
		}
	}

	return &Request{
		Path:         evt.Name,
		Size:         fileStat.Size(),
		IsDir:        fileStat.IsDir(),
		Mode:         fileStat.Mode(),
		Mimetype:     mType,
		Action:       evt.Op,
		LastModified: fileStat.ModTime(),
		Date:         time.Now(),
		Hostname:     hostname,
	}, nil
}

func NewServer(root string, handler Handler, maxDepth int, ignoreList []string, errorHandler ErrorHandler, logger Logger) (*Server, error) {
	var err error

	if root == "" {
		root, err = os.Getwd()
		if err != nil {
			return nil, err
		}
	}

	if handler == nil {
		handler = defaultServeMux
	}

	if errorHandler == nil {
		errorHandler = defaultErrorHandler
	}

	if logger == nil {
		logger = defaultLogger
	}

	ignoreList = append(ignoreList, ".git/")

	return &Server{
		Root:         root,
		Handler:      handler,
		ErrorHandler: errorHandler,
		MaxDepth:     maxDepth,
		IgnoreList:   ignoreList,
	}, nil
}
