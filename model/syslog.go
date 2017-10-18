package model

//go:generate goderive .

import (
	"bytes"
	"unicode/utf8"

	"golang.org/x/text/encoding"
)

func (m *SyslogMessage) Equals(other *SyslogMessage) bool {
	return deriveEqualSyslogMsg(m, other)
}

type Stasher interface {
	Stash(m *TcpUdpParsedMessage) (error, error)
}

type Reporter interface {
	Stasher
	Report(infos []ListenerInfo) error
}

type ListenerInfo struct {
	Port           int    `json:"port" msg:"port"`
	BindAddr       string `json:"bind_addr" mdg:"bind_addr"`
	UnixSocketPath string `json:"unix_socket_path" msg:"unix_socket_path"`
	Protocol       string `json:"protocol" msg:"protocol"`
}

type RawMessage struct {
	Client         string
	LocalPort      int
	UnixSocketPath string
}

type RawTcpMessage struct {
	RawMessage
	Message []byte
	Size    int
	Txnr    int
}

type RawUdpMessage struct {
	RawMessage
	Message [65536]byte
	Size    int
}

type Parser struct {
	format string
}

func (p *Parser) Parse(m []byte, decoder *encoding.Decoder, dont_parse_sd bool) (SyslogMessage, error) {
	return Parse(m, p.format, decoder, dont_parse_sd)
}

func Fuzz(m []byte) int {
	msg, err := Parse(m, "auto", nil, false)
	if err != nil {
		return 0
	}
	b, err := msg.MarshalMsg(nil)
	if err != nil {
		panic(err)
	}
	msg2 := SyslogMessage{}
	rest, err := msg2.UnmarshalMsg(b)
	if err != nil {
		panic("Unmarshaling failed")
	}
	if len(rest) > 0 {
		panic("after marshalling there is more bytes remaining")
	}
	if !msg.Equals(&msg2) {
		panic("msg and msg2 are not equal")
	}
	return 1
}

func GetParser(format string) *Parser {
	if format == "rfc5424" || format == "rfc3164" || format == "json" || format == "auto" {
		return &Parser{format: format}
	}
	return nil
}

func Parse(m []byte, format string, decoder *encoding.Decoder, dont_parse_sd bool) (sm SyslogMessage, err error) {

	switch format {
	case "rfc5424":
		sm, err = ParseRfc5424Format(m, decoder, dont_parse_sd)
	case "rfc3164":
		sm, err = ParseRfc3164Format(m, decoder)
	case "json":
		sm, err = ParseJsonFormat(m, decoder)
	case "auto":
		if len(m) == 0 {
			return sm, &EmptyMessageError{}
		}
		if m[0] == byte('{') {
			sm, err = ParseJsonFormat(m, decoder)
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
	if err != nil {
		return sm, err
	}
	// special handling of JSON messages produced by go-audit
	/*
		if sm.Appname == "go-audit" {
			var auditMsg AuditMessageGroup
			err = json.Unmarshal([]byte(sm.Message), &auditMsg)
			if err != nil {
				return sm, nil
			}
			sm.AuditSubMessages = auditMsg.Msgs
			if len(auditMsg.UidMap) > 0 {
				sm.Properties = map[string]map[string]string{}
				sm.Properties["uid_map"] = auditMsg.UidMap
			}
			sm.Message = ""
		}
	*/
	return sm, nil
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
