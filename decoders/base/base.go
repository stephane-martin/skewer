package base

import (
	"fmt"
	"strings"
)

type Format int

const (
	RFC5424 Format = iota
	RFC3164
	JSON
	RsyslogJSON
	GELF
	InfluxDB
	Protobuf
	Collectd
	W3C
)

var Formats = map[string]Format{
	"rfc5424":     RFC5424,
	"rfc3164":     RFC3164,
	"json":        JSON,
	"rsyslogjson": RsyslogJSON,
	"gelf":        GELF,
	"influxdb":    InfluxDB,
	"protobuf":    Protobuf,
	"collectd":    Collectd,
	"w3c":         W3C,
}

func ParseFormat(format string) Format {
	format = strings.ToLower(strings.TrimSpace(format))
	if f, ok := Formats[format]; ok {
		return f
	}
	return -1
}

//type Parser func(m []byte) ([]*model.SyslogMessage, error)

type ParsingError interface {
	Error() string
	Parsing()
}

type UnknownFormatError struct {
	format string
}

func (e *UnknownFormatError) Error() string {
	return fmt.Sprintf("Unknown parsing format: '%s'", e.format)
}

func (e *UnknownFormatError) Parsing() {}

type EmptyMessageError struct{}

func (e *EmptyMessageError) Error() string {
	return "Empty message"
}
func (e *EmptyMessageError) Parsing() {}

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

type InvalidEncodingError struct {
	Err error
}

func (e *InvalidEncodingError) Error() string {
	return fmt.Sprintf("The input message was not properly encoded: %s", e.Err.Error())
}

func (e *InvalidEncodingError) Parsing() {}

type InfluxDecodingError struct {
	Err error
}

func (e *InfluxDecodingError) Parsing() {}

func (e *InfluxDecodingError) Error() string {
	return fmt.Sprintf("Error decoding InfluxDB line protocol: %s", e.Err.Error())
}

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

type InvalidTopic struct {
	Topic string
}

func (e *InvalidTopic) Error() string {
	return fmt.Sprintf("The topic name is invalid: '%s'", e.Topic)
}
