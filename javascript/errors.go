package javascript

import "fmt"
import "github.com/dop251/goja"

type JavascriptError interface {
	Error() string
	Javascript()
}

type ObjectNotFoundError struct {
	Object string
}

func (e *ObjectNotFoundError) Error() string {
	return fmt.Sprintf("Object was not found in the Javascript VM: '%s'", e.Object)
}

func (e *ObjectNotFoundError) Javascript() {}

type NotAFunctionError struct {
	Object string
}

func (e *NotAFunctionError) Error() string {
	return fmt.Sprintf("The object is not a JS function: '%s'", e.Object)
}

func (e *NotAFunctionError) Javascript() {}

type ExecutingError interface {
	Error() string
	Javascript()
	Executing()
}

type ExceptionExecutingJSError struct {
	Exc      *goja.Exception
	FuncName string
}

func (e *ExceptionExecutingJSError) Error() string {
	return fmt.Sprintf("A JS exception happened when executing the JS function '%s'", e.FuncName)
}

func (e *ExceptionExecutingJSError) GetValue() goja.Value {
	return e.Exc.Value()
}

func (e *ExceptionExecutingJSError) Javascript() {}
func (e *ExceptionExecutingJSError) Executing()  {}

type ExecutingJSError struct {
	Err      error
	FuncName string
}

func (e *ExecutingJSError) Error() string {
	return fmt.Sprintf("An unexpected error happened when executing the JS function '%s': %s", e.FuncName, e.Err.Error())
}

func (e *ExecutingJSError) Javascript() {}
func (e *ExecutingJSError) Executing()  {}

func ExecutingJSErrorFactory(err error, funcname string) ExecutingError {
	if jsexc, ok := err.(*goja.Exception); ok {
		return &ExceptionExecutingJSError{Exc: jsexc, FuncName: funcname}
	} else {
		return &ExecutingJSError{Err: err, FuncName: funcname}
	}
}

// go-js object conversions error (with wrapped)
type ConversionGoJsError struct {
	Err ExecutingError
}

func (e *ConversionGoJsError) Error() string {
	return fmt.Sprintf("Error converting a Go variable to Javascript: %s", e.Err.Error())
}

func (e *ConversionGoJsError) Javascript() {}

type ConversionJsGoError struct {
	Err error
}

func (e *ConversionJsGoError) Error() string {
	return fmt.Sprintf("Error converting a Javascript variable to Go: %s", e.Err.Error())
}

func (e *ConversionJsGoError) Javascript() {}

// filter error (with wrapped)
