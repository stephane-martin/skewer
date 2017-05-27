package server

import "errors"

var ServerDefinitelyStopped = errors.New("Server is definitely stopped")
var ServerNotStopped = errors.New("Server is not stopped")
