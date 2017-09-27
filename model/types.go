package model

import (
	"fmt"
	"time"
)

//go:generate msgp
//go:generate ffjson $GOFILE

type Priority int
type Facility int
type Severity int
type Version int

// ffjson: nodecoder
type SyslogMessage struct {
	Priority         Priority                     `json:"priority,string" msg:"priority"`
	Facility         Facility                     `json:"facility,string" msg:"facility"`
	Severity         Severity                     `json:"severity,string" msg:"severity"`
	Version          Version                      `json:"version,string" msg:"version"`
	TimeReportedNum  int64                        `json:"-" msg:"timereportednum"`
	TimeGeneratedNum int64                        `json:"-" msg:"timegeneratednum"`
	TimeReported     string                       `json:"timereported" msg:"timereported"`
	TimeGenerated    string                       `json:"timegenerated" msg:"timegenerated"`
	Hostname         string                       `json:"hostname" msg:"hostname"`
	Appname          string                       `json:"appname" msg:"appname"`
	Procid           string                       `json:"procid,omitempty" msg:"procid"`
	Msgid            string                       `json:"msgid,omitempty" msg:"msgid"`
	Structured       string                       `json:"structured,omitempty" msg:"structured"`
	Message          string                       `json:"message" msg:"message"`
	Properties       map[string]map[string]string `json:"properties,omitempty" msg:"properties"`
}

// ffjson: nodecoder
type ParsedMessage struct {
	Fields         SyslogMessage `json:"fields" msg:"fields"`
	Client         string        `json:"client,omitempty" msg:"client"`
	LocalPort      int           `json:"local_port,string" msg:"local_port"`
	UnixSocketPath string        `json:"unix_socket_path,omitempty" msg:"unix_socket_path"`
}

// ffjson: nodecoder
type ExportedMessage struct {
	ParsedMessage
}

// ffjson: skip
type TcpUdpParsedMessage struct {
	Parsed ParsedMessage `json:"parsed" msg:"parsed"`
	Uid    string        `json:"uid" msg:"uid"`
	ConfId string        `json:"conf_id" msg:"conf_id"`
}

// ffjson: skip
type RelpParsedMessage struct {
	Parsed ParsedMessage `json:"parsed" msg:"parsed"`
	Txnr   int           `json:"txnr" msg:"txnr"`
}

var SyslogMessageFmt string = `Facility: %d
Severity: %d
Version: %d
TimeReported: %s
TimeGenerated: %s
Hostname: %s
Appname: %s
ProcID: %s
MsgID: %s
Structured: %s
Message: %s
Properties: %s`

func (m *SyslogMessage) String() string {
	return fmt.Sprintf(
		SyslogMessageFmt,
		m.Facility,
		m.Severity,
		m.Version,
		time.Unix(0, m.TimeReportedNum).UTC().Format(time.RFC3339),
		time.Unix(0, m.TimeGeneratedNum).UTC().Format(time.RFC3339),
		m.Hostname,
		m.Appname,
		m.Procid,
		m.Msgid,
		m.Structured,
		m.Message,
		m.Properties,
	)
}
