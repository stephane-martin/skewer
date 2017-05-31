package model

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	uuid "github.com/satori/go.uuid"
)

type Priority int
type Facility int
type Severity int
type Version int

type SyslogMessage struct {
	Priority      Priority               `json:"priority,string"`
	Facility      Facility               `json:"facility,string"`
	Severity      Severity               `json:"severity,string"`
	Version       Version                `json:"version,string"`
	TimeReported  *time.Time             `json:"timereported"`
	TimeGenerated *time.Time             `json:"timegenerated"`
	Hostname      string                 `json:"hostname"`
	Appname       string                 `json:"appname"`
	Procid        string                 `json:"procid"`
	Msgid         string                 `json:"msgid"`
	Structured    string                 `json:"structured"`
	Message       string                 `json:"message"`
	Properties    map[string]interface{} `json:"properties"`
}

type TcpRawMessage struct {
	RawMessage
	Uid uuid.UUID
}

type TcpParsedMessage struct {
	Message   *SyslogMessage `json:"message"`
	Uid       string         `json:"-"`
	Client    string         `json:"client"`
	LocalPort int            `json:"local_port,string"`
	ConfIndex int            `json:"-"`
}

type RawMessage struct {
	Message   string
	Client    string
	LocalPort int
}

type ParsedMessage struct {
	Message   *SyslogMessage `json:"message"`
	Client    string         `json:"client"`
	LocalPort int            `json:"local_port,string"`
}

type RelpRawMessage struct {
	RawMessage
	Txnr int
}

type RelpParsedMessage struct {
	Message   *SyslogMessage `json:"message"`
	Txnr      int            `json:"-"`
	Client    string         `json:"client"`
	LocalPort int            `json:"local_port,string"`
}

var SyslogMessageFmt string = `Facility: %d
Severity: %d
Version: %d
Timestamp: %s
Hostname: %s
Appname: %s
ProcID: %s
MsgID: %s
Structured: %s
Message: %s
Properties: %s`

func (m *SyslogMessage) String() string {
	props := ""
	b, err := json.Marshal(m.Properties)
	if err == nil {
		props = string(b)
	}
	return fmt.Sprintf(
		SyslogMessageFmt,
		m.Facility,
		m.Severity,
		m.Version,
		m.TimeReported.Format(time.RFC3339),
		m.Hostname,
		m.Appname,
		m.Procid,
		m.Msgid,
		m.Structured,
		m.Message,
		props,
	)
}

func Parse(m string, format string) (*SyslogMessage, error) {
	switch format {
	case "rfc5424":
		return ParseRfc5424Format(m)
	case "rfc3164":
		return ParseRfc3164Format(m)
	case "json":
		return ParseJsonFormat(m)
	case "auto":
		if m[0] == byte('{') {
			return ParseJsonFormat(m)
		}
		if m[0] != byte('<') {
			return ParseRfc3164Format(m)
		}
		i := strings.Index(m, ">")
		if i < 2 {
			return ParseRfc3164Format(m)
		}
		if len(m) == (i + 1) {
			return ParseRfc3164Format(m)
		}
		if m[i+1] == byte('1') {
			return ParseRfc5424Format(m)
		}
		return ParseRfc3164Format(m)

	default:
		return nil, fmt.Errorf("unknown format")
	}
}
