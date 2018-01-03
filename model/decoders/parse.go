package decoders

import (
	"bytes"
	"strings"
	"unicode/utf8"

	"github.com/stephane-martin/skewer/model"
	"golang.org/x/text/encoding"
)

type Format int

const (
	RFC5424 Format = iota
	RFC3164
	JSON
	FullJSON
	GELF
	InfluxDB
	Auto
)

var Formats = map[string]Format{
	"rfc5424":  RFC5424,
	"rfc3164":  RFC3164,
	"json":     JSON,
	"fulljson": FullJSON,
	"gelf":     GELF,
	"influxdb": InfluxDB,
	"auto":     Auto,
}

func ParseFormat(format string) Format {
	format = strings.ToLower(strings.TrimSpace(format))
	if f, ok := Formats[format]; ok {
		return f
	}
	return -1
}

type Parser struct {
	format Format
}

func (p *Parser) Parse(m []byte, decoder *encoding.Decoder, dont_parse_sd bool) (*model.SyslogMessage, error) {
	return Parse(m, p.format, decoder, dont_parse_sd)
}

func Fuzz(m []byte) int {
	msg, err := Parse(m, Auto, nil, false)
	if err != nil {
		return 0
	}
	b, err := msg.Marshal()
	if err != nil {
		panic(err)
	}
	msg2 := &model.SyslogMessage{}
	err = msg2.Unmarshal(b)
	if err != nil {
		panic("Unmarshaling failed")
	}
	if !msg.Equal(msg2) {
		panic("msg and msg2 are not equal")
	}
	return 1
}

func GetParser(format Format) *Parser {
	return &Parser{format: format}
}

func Parse(m []byte, format Format, decoder *encoding.Decoder, dont_parse_sd bool) (sm *model.SyslogMessage, err error) {

	switch format {
	case RFC5424:
		sm, err = ParseRfc5424Format(m, decoder, dont_parse_sd)
	case RFC3164:
		sm, err = ParseRfc3164Format(m, decoder)
	case JSON:
		sm, err = ParseJsonFormat(m, decoder)
	case FullJSON:
		sm, err = ParseFullJsonFormat(m, decoder)
	case GELF:
		sm, err = ParseGelfFormat(m, decoder)
	case InfluxDB:
		sm, err = ParseInfluxFormat(m, decoder)
	case Auto:
		if len(m) == 0 {
			return sm, &EmptyMessageError{}
		}
		if m[0] == byte('{') {
			sm, err = ParseJsonFormat(m, decoder)
			if err != nil {
				sm, err = ParseFullJsonFormat(m, decoder)
			}
		} else if m[0] != byte('<') {
			sm, err = ParseRfc3164Format(m, decoder)
		} else {
			i := bytes.Index(m, []byte(">"))
			if i < 2 {
				sm, err = ParseRfc3164Format(m, decoder)
			} else if len(m) == (i + 1) {
				sm, err = ParseRfc3164Format(m, decoder)
			} else if m[i+1] == byte('1') {
				sm, err = ParseRfc5424Format(m, decoder, dont_parse_sd)
			} else {
				sm, err = ParseRfc3164Format(m, decoder)
			}
		}

	default:
		return sm, &UnknownFormatError{format}
	}
	return sm, err
}

func TopicNameIsValid(name string) bool {
	if len(name) == 0 {
		return false
	}
	if len(name) > 249 {
		return false
	}
	if !utf8.ValidString(name) {
		return false
	}
	for _, r := range name {
		if !validRune(r) {
			return false
		}
	}
	return true
}

func validRune(r rune) bool {
	if r >= 'a' && r <= 'z' {
		return true
	}
	if r >= 'A' && r <= 'Z' {
		return true
	}
	if r >= '0' && r <= '9' {
		return true
	}
	if r == '.' {
		return true
	}
	if r == '_' {
		return true
	}
	if r == '-' {
		return true
	}
	return false
}
