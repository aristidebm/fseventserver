package fseventserver

import (
	"errors"
)

var ErrListerningDirectory = errors.New("cannot listen to change from directory")

var ErrHandlingRequest = errors.New("cannot handle the request")

var ErrWatchingFile = errors.New("cannot watch the file")

var ErrInternalError = errors.New("cannot serve request anymore")

var ErrRegisteringPath = errors.New("cannot register the path")
