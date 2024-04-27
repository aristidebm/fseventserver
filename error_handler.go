package fseventserver

import (
	//    "errors"
	"github.com/fsnotify/fsnotify"
)

type DefaultErrorHandler struct{}

func (self *DefaultErrorHandler) HandleError(err error) {

	switch err.Error() {
	case fsnotify.ErrClosed.Error():
		//
	case fsnotify.ErrNonExistentWatch.Error():
		//
	case fsnotify.ErrEventOverflow.Error():
		//
	default:
		//
	}

}

func NewErrorHandler() *DefaultErrorHandler {
	return &DefaultErrorHandler{}
}

var defaultErrorHandler = NewErrorHandler()
