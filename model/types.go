package model

import (
	"bytes"
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

// ffjson: skip
type TcpUdpParsedMessage struct {
	Parsed ParsedMessage `json:"parsed" msg:"parsed"`
	Uid    [16]byte      `json:"uid" msg:"uid"`
	ConfId [16]byte      `json:"conf_id" msg:"conf_id"`
	Txnr   int           `json:"txnr" msg:"txnr"`
}

var syslogMessageFmt string = `Facility: %d
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
		syslogMessageFmt,
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

// Empty returns true if the message is empty
func (m *SyslogMessage) Empty() bool {
	return len(m.Message) == 0 && len(m.Structured) == 0 && len(m.Properties) == 0
}

// Marshal5424 formats the message as a RFC5424 line
func (m *SyslogMessage) Marshal5424() ([]byte, error) {
	b := bytes.NewBuffer(nil)
	fmt.Fprintf(b, "<%d>1 %s %s %s %s %s ",
		m.Priority,
		time.Unix(0, m.TimeReportedNum).UTC().Format(time.RFC3339Nano),
		nilify(m.Hostname),
		nilify(m.Appname),
		nilify(m.Procid),
		nilify(m.Msgid))

	if len(m.Properties) == 0 {
		fmt.Fprint(b, "-")
	}
	for sid := range m.Properties {
		fmt.Fprintf(b, "[%s", sid)
		for name, value := range m.Properties[sid] {
			fmt.Fprintf(b, " %s=\"%s\"", name, escapeSDParam(value))
		}
		fmt.Fprintf(b, "]")
	}

	if len(m.Message) > 0 {
		fmt.Fprint(b, " ")
		b.Write([]byte(m.Message))
	}
	return b.Bytes(), nil
}

func nilify(x string) string {
	if x == "" {
		return "-"
	}
	return x
}

func escapeSDParam(s string) string {
	escapeCount := 0
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '\\', '"', ']':
			escapeCount++
		}
	}
	if escapeCount == 0 {
		return s
	}

	t := make([]byte, len(s)+escapeCount)
	j := 0
	for i := 0; i < len(s); i++ {
		switch c := s[i]; c {
		case '\\', '"', ']':
			t[j] = '\\'
			t[j+1] = c
			j += 2
		default:
			t[j] = s[i]
			j++
		}
	}
	return string(t)
}
