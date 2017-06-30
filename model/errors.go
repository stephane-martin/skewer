package model

import "fmt"

type ParsingError interface {
	Error() string
	Parsing()
}

type UnknownFormatError struct {
	Format string
}

func (e *UnknownFormatError) Error() string {
	return fmt.Sprintf("Unknown parsing format: '%s'", e.Format)
}

func (e *UnknownFormatError) Parsing() {}

type ParsingErrorRfc5424 interface {
	Error() string
	Parsing()
	Rfc5424()
}

type ParsingErrorJson interface {
	Error() string
	Parsing()
	Json()
}

type UnmarshalingJsonError struct {
	Err error
}

func (e *UnmarshalingJsonError) Error() string {
	return fmt.Sprintf("Error happend when trying to unmarshal the syslog message from JSON: %s", e.Err.Error())
}

func (e *UnmarshalingJsonError) Json()    {}
func (e *UnmarshalingJsonError) Parsing() {}

type TimeError struct{}

func (e *TimeError) Error() string {
	return "TimeReported and TimeGenerated should be formatted in RFC3339 format, but parsing them failed"
}

func (e *TimeError) Json()    {}
func (e *TimeError) Parsing() {}

type InvalidStructuredDataError struct {
	Message string
}

func (e *InvalidStructuredDataError) Error() string {
	if len(e.Message) > 0 {
		return fmt.Sprintf("Invalid structured data: %s", e.Message)
	} else {
		return "Invalid structure data"
	}
}

func (e *InvalidStructuredDataError) Rfc5424() {}
func (e *InvalidStructuredDataError) Parsing() {}

type InvalidPriorityError struct{}

func (e *InvalidPriorityError) Error() string {
	return "Invalid priority"
}

func (e *InvalidPriorityError) Rfc5424() {}
func (e *InvalidPriorityError) Json()    {}
func (e *InvalidPriorityError) Parsing() {}

type NotEnoughPartsError struct {
	NbParts int
}

func (e *NotEnoughPartsError) Error() string {
	return fmt.Sprintf("The message does not have enough parts: %d, but minimum is 7", e.NbParts)
}

func (e *NotEnoughPartsError) Rfc5424() {}
func (e *NotEnoughPartsError) Parsing() {}

type JSParsingError struct {
	Message    string
	ParserName string
}

func (e *JSParsingError) Error() string {
	return fmt.Sprintf("The provided JS parser '%s' could not parse the raw message: %s", e.ParserName, e.Message)
}

func (e *JSParsingError) Parsing()    {}
func (e *JSParsingError) Javascript() {}

type InvalidTopic struct {
	Topic string
}

func (e *InvalidTopic) Error() string {
	return fmt.Sprintf("The topic name is invalid: '%s'", e.Topic)
}
