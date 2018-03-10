package decoders

import (
	"strings"
	"sync"

	"github.com/inconshreveable/log15"
	"github.com/stephane-martin/skewer/decoders/base"
	"github.com/stephane-martin/skewer/javascript"
	"github.com/stephane-martin/skewer/model"
	"github.com/stephane-martin/skewer/utils"
	"github.com/zond/gotomic"
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
	sync.Mutex
	exists *gotomic.Hash
	jsenv  *javascript.Environment
	logger log15.Logger
}

func NewParsersEnv(logger log15.Logger) *ParsersEnv {
	return &ParsersEnv{
		jsenv:  javascript.NewParsersEnvironment(logger),
		logger: logger,
		exists: gotomic.NewHash(),
	}
}

func (e *ParsersEnv) AddJSParser(name, funct string) {
	err := e.jsenv.AddParser(name, funct)
	if err != nil {
		e.logger.Warn("Error initializing parser", "name", name, "error", err)
	}
}

func (e *ParsersEnv) GetParser(c *model.DecoderConfig) (p base.Parser, err error) {
	// some parsers may be heavy to build, so we cache them
	if pi, have := e.exists.Get(c); have {
		return pi.(base.Parser), nil
	}
	e.Lock()
	defer e.Unlock()
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
	p = parserWithEncoding(frmt, c.Charset, p)
	e.exists.Put(c, p)
	return p, nil
}

func parserWithEncoding(frmt Format, charset string, p base.Parser) base.Parser {
	switch frmt {
	case RFC3164, RFC5424, W3C:
		return func(m []byte) ([]*model.SyslogMessage, error) {
			var err error
			m, err = utils.SelectDecoder(charset).Bytes(m)
			if err != nil {
				return nil, &base.InvalidEncodingError{Err: err}
			}
			return p(m)
		}
	case JSON, RsyslogJSON, GELF, InfluxDB, -1:
		return func(m []byte) ([]*model.SyslogMessage, error) {
			var err error
			m, err = unicode.UTF8.NewDecoder().Bytes(m)
			if err != nil {
				return nil, &base.InvalidEncodingError{Err: err}
			}
			return p(m)
		}
	case Protobuf, Collectd:
		return p
	default:
		return p
	}
}
