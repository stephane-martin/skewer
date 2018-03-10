package decoders

import (
	"strings"

	"github.com/inconshreveable/log15"
	"github.com/stephane-martin/skewer/decoders/base"
	"github.com/stephane-martin/skewer/javascript"
	"github.com/stephane-martin/skewer/model"
	"github.com/stephane-martin/skewer/utils"
	"golang.org/x/text/encoding/unicode"
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

var parsers = map[Format]base.Parser{
	RFC5424:     p5424,
	RFC3164:     p3164,
	JSON:        pJSON,
	RsyslogJSON: pRsyslogJSON,
	GELF:        pGELF,
	InfluxDB:    pInflux,
	Protobuf:    pProtobuf,
	Collectd:    pCollectd,
	W3C:         nil,
}

func ParseFormat(format string) Format {
	format = strings.ToLower(strings.TrimSpace(format))
	if f, ok := Formats[format]; ok {
		return f
	}
	return -1
}

type ParsersEnv struct {
	jsenv  javascript.ParsersEnvironment
	logger log15.Logger
}

func NewParsersEnv(logger log15.Logger) *ParsersEnv {
	jsenv := javascript.NewParsersEnvironment(logger)
	return &ParsersEnv{jsenv: jsenv, logger: logger}
}

func (e *ParsersEnv) AddJSParser(name, funct string) {
	err := e.jsenv.AddParser(name, funct)
	if err != nil {
		e.logger.Warn("Error initializing parser", "name", name, "error", err)
	}
}

func (e *ParsersEnv) GetParser(c *model.DecoderConfig) (p base.Parser, err error) {
	frmt := ParseFormat(c.Format)
	switch frmt {
	case W3C:
		p = W3CDecoder(c.W3CFields)
	case -1:
		p, err = e.jsenv.GetParser(c.Format)
	default:
		p = parsers[frmt]
	}
	if err != nil {
		return nil, err
	}

	switch frmt {
	case RFC3164, RFC5424, W3C:
		return func(m []byte) ([]*model.SyslogMessage, error) {
			var err error
			m, err = utils.SelectDecoder(c.Charset).Bytes(m)
			if err != nil {
				return nil, &base.InvalidEncodingError{Err: err}
			}
			return p(m)
		}, nil
	case JSON, RsyslogJSON, GELF, InfluxDB:
		return func(m []byte) ([]*model.SyslogMessage, error) {
			var err error
			m, err = unicode.UTF8.NewDecoder().Bytes(m)
			if err != nil {
				return nil, &base.InvalidEncodingError{Err: err}
			}
			return p(m)
		}, nil
	case Protobuf, Collectd:
		return p, nil
	default:
		return p, nil
	}

}
