package fsserver

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

	"github.com/fsnotify/fsnotify"
)

type Server struct {
	Root       string
	Handler    Handler
	MaxDepth   int
	IgnoreList []string
	watcher    *fsnotify.Watcher
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

		for _, item := range self.IgnoreList[:] {
			if item == path {
				return nil
			}
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
			case _, ok := <-self.watcher.Events:
				// the events channel is closed, we cannot receive
				// the event anymore, end the goroutine
				if !ok {
					return
				}

				// if event.Has(fsnotify.Create) {
				// 	go func() {
				// 		if err := self.mux.Route(event.Name); err != nil {
				// 			watcher.Errors <- err
				// 		}
				// 	}()
				//}
			case err, ok := <-self.watcher.Errors:
				// the error channel is closed, we cannot receive
				// the errors anymore, end the goroutine
				if !ok {
					return
				}
				log.Print(err)
			}
		}
	}()

	wg.Wait()

	return nil
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
		handler = nil
	}

	ignoreList = append(ignoreList, ".git/")

	return &Server{
		Root:       root,
		Handler:    handler,
		MaxDepth:   depth,
		IgnoreList: ignoreList,
	}, nil
}

type Handler interface {
	Handle(path string, fun HandleFunc)
}

type HandleFunc func(ctx context.Context) error
