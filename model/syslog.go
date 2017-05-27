package model

import (
	"encoding/json"
	"fmt"
	"time"
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
	default:
		return nil, fmt.Errorf("unknown format")
	}
}
