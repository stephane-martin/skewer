package decoders

import (
	"bytes"
	"fmt"
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
	Protobuf
)

var Formats = map[string]Format{
	"rfc5424":  RFC5424,
	"rfc3164":  RFC3164,
	"json":     JSON,
	"fulljson": FullJSON,
	"gelf":     GELF,
	"influxdb": InfluxDB,
	"auto":     Auto,
	"protobuf": Protobuf,
}

var parsers = map[Format]Parser{
	RFC5424:  p5424,
	RFC3164:  p3164,
	JSON:     pJson,
	FullJSON: pFulljson,
	GELF:     pGelf,
	InfluxDB: pInflux,
	Auto:     pAuto,
	Protobuf: pProtobuf,
}

func ParseFormat(format string) Format {
	format = strings.ToLower(strings.TrimSpace(format))
	if f, ok := Formats[format]; ok {
		return f
	}
	return -1
}

type Parser func(m []byte, decoder *encoding.Decoder) (*model.SyslogMessage, error)

func Fuzz(m []byte) int {
	msg, err := pAuto(m, nil)
	if err != nil {
		return 0
	}
	b, err := msg.Marshal()
	if err != nil {
		panic(err)
	}
	msg2 := model.Factory()
	err = msg2.Unmarshal(b)
	if err != nil {
		panic("Unmarshaling failed")
	}
	if !msg.Equal(msg2) {
		panic("msg and msg2 are not equal")
	}
	model.Free(msg2)
	return 1
}

func GetParser(format Format) (Parser, error) {
	if p, ok := parsers[format]; ok {
		return p, nil
	}
	return nil, fmt.Errorf("Unknown decoding format: %d", format)
}

func pAuto(m []byte, decoder *encoding.Decoder) (sm *model.SyslogMessage, err error) {
	if len(m) == 0 {
		return sm, &EmptyMessageError{}
	}
	if m[0] == byte('{') {
		sm, err = pJson(m, decoder)
		if err != nil {
			sm, err = pFulljson(m, decoder)
		}
	} else if m[0] != byte('<') {
		sm, err = p3164(m, decoder)
	} else {
		i := bytes.Index(m, []byte(">"))
		if i < 2 {
			sm, err = p3164(m, decoder)
		} else if len(m) == (i + 1) {
			sm, err = p3164(m, decoder)
		} else if m[i+1] == byte('1') {
			sm, err = p5424(m, decoder)
		} else {
			sm, err = p3164(m, decoder)
		}
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
