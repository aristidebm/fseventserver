package fseventserver

import (
	//"errors"
	// "github.com/fsnotify/fsnotify"
	"log/slog"
)

type DefaultErrorHandler struct{}

func (self *DefaultErrorHandler) HandleError(err error) {
	slog.Error(err.Error())
}

func NewErrorHandler() *DefaultErrorHandler {
	return &DefaultErrorHandler{}
}

var defaultErrorHandler = NewErrorHandler()
