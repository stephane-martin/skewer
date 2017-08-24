package model

import (
	"encoding/json"
	"strings"
	"unicode/utf8"
)

type Stasher interface {
	Stash(m *TcpUdpParsedMessage)
}

type ListenerInfo struct {
	Port           int    `json:"port" msg:"port"`
	BindAddr       string `json:"bind_addr" mdg:"bind_addr"`
	UnixSocketPath string `json:"unix_socket_path" msg:"unix_socket_path"`
	Protocol       string `json:"protocol" msg:"protocol"`
}

type RawMessage struct {
	Message        string
	Client         string
	LocalPort      int
	UnixSocketPath string
}

type RelpRawMessage struct {
	Raw  *RawMessage
	Txnr int
}

type Parser struct {
	format string
}

func (p *Parser) Parse(m string, dont_parse_sd bool) (*SyslogMessage, error) {
	return Parse(m, p.format, dont_parse_sd)
}

func GetParser(format string) *Parser {
	if format == "rfc5424" || format == "rfc3164" || format == "json" || format == "auto" {
		return &Parser{format: format}
	}
	return nil
}

func Parse(m string, format string, dont_parse_sd bool) (sm *SyslogMessage, err error) {

	switch format {
	case "rfc5424":
		sm, err = ParseRfc5424Format(m, dont_parse_sd)
	case "rfc3164":
		sm, err = ParseRfc3164Format(m)
	case "json":
		sm, err = ParseJsonFormat(m)
	case "auto":
		if m[0] == byte('{') {
			sm, err = ParseJsonFormat(m)
		} else if m[0] != byte('<') {
			sm, err = ParseRfc3164Format(m)
		} else {
			i := strings.Index(m, ">")
			if i < 2 {
				sm, err = ParseRfc3164Format(m)
			} else if len(m) == (i + 1) {
				sm, err = ParseRfc3164Format(m)
			} else if m[i+1] == byte('1') {
				sm, err = ParseRfc5424Format(m, dont_parse_sd)
			} else {
				sm, err = ParseRfc3164Format(m)
			}
		}

	default:
		return nil, &UnknownFormatError{format}
	}
	if err != nil {
		return nil, err
	}
	// special handling of JSON messages produced by go-audit
	if sm.Appname == "go-audit" {
		var auditMsg AuditMessageGroup
		err = json.Unmarshal([]byte(sm.Message), &auditMsg)
		if err != nil {
			return sm, nil
		}
		sm.AuditSubMessages = auditMsg.Msgs
		if len(auditMsg.UidMap) > 0 {
			if sm.Properties == nil {
				sm.Properties = map[string]map[string]string{}

			}
			props := map[string]map[string]string{}
			props["uid_map"] = auditMsg.UidMap
			sm.Properties = props
		}
		sm.Message = ""
	}
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
