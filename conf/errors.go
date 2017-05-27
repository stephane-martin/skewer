package conf

import "fmt"

type ConfigurationError interface {
	Error() string
	ConfigError() string
}

//
type ConfigurationReadError struct {
	Err error
}

func (e ConfigurationReadError) Error() string {
	return fmt.Sprintf("Error reading configuration file: %s", e.Err.Error())
}

func (e ConfigurationReadError) WrappedErrors() []error {
	return []error{e.Err}
}

func (e ConfigurationReadError) ConfigError() string {
	return e.Error()
}

//
type ConfigurationSyntaxError struct {
	Err      error
	Filename string
}

func (e ConfigurationSyntaxError) Error() string {
	return fmt.Sprintf("Syntax error in configuration file '%s': %s", e.Filename, e.Err.Error)
}

func (e ConfigurationSyntaxError) WrappedErrors() []error {
	return []error{e.Err}
}

func (e ConfigurationSyntaxError) ConfigError() string {
	return e.Error()
}

//
type ConfigurationCheckError struct {
	ErrString string
	Err       error
}

func (e ConfigurationCheckError) Error() string {
	if e.Err == nil {
		return e.ErrString
	}
	return fmt.Sprintf("%s: %s", e.ErrString, e.Err.Error())
}

func (e ConfigurationCheckError) WrappedErrors() []error {
	if e.Err == nil {
		return nil
	}
	return []error{e.Err}
}

func (e ConfigurationCheckError) ConfigError() string {
	return e.Error()
}

//
type KafkaError struct {
	Err error
}

func (e KafkaError) Error() string {
	return fmt.Sprintf("Kafka error: %s", e.Err.Error())
}

func (e KafkaError) WrappedErrors() []error {
	return []error{e.Err}
}
