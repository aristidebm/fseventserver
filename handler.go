package fseventserver

import (
	"context"
	"errors"

	"github.com/gobwas/glob"
)

type HandleFunc func(ctx context.Context) error

func (self HandleFunc) ServeFSEvent(ctx context.Context) error {
	return self(ctx)
}

type ServeMux struct {
	patterns map[glob.Glob]Handler
}

func (self *ServeMux) ServeFSEvent(ctx context.Context) error {
	handler := self.findHandler(ctx)
	return handler.ServeFSEvent(ctx)
}

func (self *ServeMux) register(path string, handler Handler) error {

	if path == "" {
		return errors.New("")
	}

	if fun, ok := handler.(HandleFunc); ok && fun == nil {
		return errors.New("")
	}

	if len(self.patterns) == 0 {
		self.patterns = make(map[glob.Glob]Handler)
	}

	pattern, err := glob.Compile(path)
	if err != nil {
		return err
	}

	if _, ok := self.patterns[pattern]; ok {
		return errors.New("")
	}

	self.patterns[pattern] = handler

	return nil
}

func (self *ServeMux) findHandler(ctx context.Context) Handler {
	value := ctx.Value("request")
	req, ok := value.(request)

	if !ok {
		return nil
	}

	for pattern, handler := range self.patterns {
		if pattern.Match(req.path) {
			return handler
		}
	}
	return nil
}

func NewServerMux() *ServeMux {
	return &ServeMux{}
}

var defaultServeMux = NewServerMux()
