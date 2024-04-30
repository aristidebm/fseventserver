package fseventserver

import (
	"io"
)

type DefaulLogger struct{}

func (self *DefaulLogger) Trace(msg string)       {}
func (self *DefaulLogger) Debug(msg string)       {}
func (self *DefaulLogger) Info(msg string)        {}
func (self *DefaulLogger) Warn(msg string)        {}
func (self *DefaulLogger) Error(msg string)       {}
func (self *DefaulLogger) SetOuput(out io.Writer) {}

func NewLogger() *DefaulLogger {
	return &DefaulLogger{}
}

var defaultLogger = NewLogger()
