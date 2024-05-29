package fseventserver

import (
	"context"
	"fmt"
	"path/filepath"
	"reflect"

	"github.com/gobwas/glob"
)

type HandlerFunc func(ctx context.Context) error

type ServeMux struct {
	store []storeItem
}

// glob.Compile result is not hashable
// so we cannot used as builtin map keys
// if we don't made them hashable
type storeItem struct {
	key   glob.Glob
	value Handler
}

func (self HandlerFunc) ServeFSEvent(ctx context.Context) error {
	return self(ctx)
}

func Handle(path string, handler Handler) {
	if err := defaultServeMux.register(path, handler); err != nil {
		panic(err)
	}
}

func HandleFunc(path string, handler HandlerFunc) {
	if err := defaultServeMux.register(path, handler); err != nil {
		panic(err)
	}
}

func (self *ServeMux) ServeFSEvent(ctx context.Context) error {
	if handler := self.findHandler(ctx); handler != nil {
		return handler.ServeFSEvent(ctx)
	}
	req := ctx.Value("request")
	return fmt.Errorf("%w cannot find a handler associated with the request %+v", ErrHandlingRequest, req)
}

func (self *ServeMux) register(path string, handler Handler) error {
	var err error
	if path == "" {
		return fmt.Errorf("%w the path should not be empty", ErrRegisteringPath)
	}

	if !filepath.IsAbs(path) {
		path, err = expandUser(path)
		if err != nil {
			return fmt.Errorf("%w cannot expand the path, you have to provide an absolute path", ErrRegisteringPath)
		}
	}

	if fun, ok := handler.(HandlerFunc); ok && fun == nil {
		return fmt.Errorf("%w the handler cannot be nil, you have to provide a non nil handler", ErrRegisteringPath)
	}

	if len(self.store) == 0 {
		self.store = make([]storeItem, 0)
	}

	pattern, err := glob.Compile(path)
	if err != nil {
		return fmt.Errorf("%w the provided path is not glob compatible, make sure you provide a glob like path %w", ErrRegisteringPath, err)
	}

	for _, item := range self.store {
		if reflect.DeepEqual(item.key, pattern) {
			return fmt.Errorf("%w the path is already registered", ErrRegisteringPath)
		}
	}

	self.store = append(self.store, storeItem{key: pattern, value: handler})

	return nil
}

func (self *ServeMux) findHandler(ctx context.Context) Handler {
	value := ctx.Value("request")
	req, ok := value.(*Request)

	if !ok {
		return nil
	}

	for _, item := range self.store {
		if item.key.Match(req.Path) {
			return item.value
		}
	}
	return nil
}

func NewServerMux() *ServeMux {
	return &ServeMux{}
}

var defaultServeMux = NewServerMux()
