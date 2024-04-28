package fseventserver

import (
	"context"
	"errors"
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
	handler := self.findHandler(ctx)
	return handler.ServeFSEvent(ctx)
}

func (self *ServeMux) register(path string, handler Handler) error {
	if path == "" {
		return errors.New("")
	}

	if fun, ok := handler.(HandlerFunc); ok && fun == nil {
		return errors.New("")
	}

	if len(self.store) == 0 {
		self.store = make([]storeItem, 0)
	}

	pattern, err := glob.Compile(path)
	if err != nil {
		return err
	}

	for _, item := range self.store {
		if reflect.DeepEqual(item.key, pattern) {
			return errors.New("")
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
