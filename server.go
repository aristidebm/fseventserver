package fseventserver

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/gobwas/glob"
)

type Server struct {
	Root         string
	Handler      Handler
	MaxDepth     int
	ErrorHandler ErrorHandler
	// glog pattern can be provided
	IgnoreList         []string
	compiledIgnoreList []glob.Glob
	watcher            *fsnotify.Watcher
}

type Handler interface {
	ServeFSEvent(ctx context.Context) error
}

type ErrorHandler interface {
	HandleError(err error)
}

type request struct {
	path     string
	size     int
	fileType string
	mimetype string
	action   string
	date     time.Time
	hostname string
}

func (self *Server) ListenAndServe() error {
	var err error

	root := self.Root
	if root == "" {
		root, err = os.Getwd()
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

		// We are only interested in Dirs
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
			return err
		}
	}
	var wg sync.WaitGroup

	// use a goroutine to listen to changes
	// and process them
	wg.Add(1)
	go func() {

		defer wg.Done()
		for {
			select {
			case event, ok := <-self.watcher.Events:
				// the events channel is closed, we cannot receive
				// the event anymore, end the goroutine
				if !ok {
					return
				}
				go func() {
					ctx, err := self.makeContext(event)
					if err != nil {
						self.watcher.Errors <- err
						return
					}
					if err := self.Handler.ServeFSEvent(ctx); err != nil {
						self.watcher.Errors <- err
					}
				}()
			case err, ok := <-self.watcher.Errors:
				// the error channel is closed, we cannot receive
				// the errors anymore, end the goroutine
				if !ok {
					return
				}
				self.ErrorHandler.HandleError(err)
			}
		}
	}()

	wg.Wait()

	return nil
}

func (self *Server) makeContext(evt fsnotify.Event) (context.Context, error) {
	req, err := self.makeRequest(evt)
	if err != nil {
		return nil, err
	}
	ctx := context.Background()
	ctx = context.WithValue(ctx, "request", req)
	return ctx, nil
}

func (self *Server) makeRequest(evt fsnotify.Event) (request, error) {
	return request{}, nil
}

func NewServer(root string, depth int, ignoreList []string, handler Handler) (*Server, error) {
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

	ignoreList = append(ignoreList, ".git/")

	return &Server{
		Root:       root,
		Handler:    handler,
		MaxDepth:   depth,
		IgnoreList: ignoreList,
	}, nil
}
