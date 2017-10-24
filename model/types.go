package model

import (
	"bytes"
	"fmt"
	"os"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/pquerna/ffjson/ffjson"
	"github.com/stephane-martin/skewer/utils"
)

//go:generate msgp
//go:generate ffjson $GOFILE
//msgp:ignore JsonRsyslogMessage

type Priority int
type Facility int
type Severity int
type Version int

// ffjson: nodecoder
type SyslogMessage struct {
	Priority         Priority `json:"priority,string" msg:"priority"`
	Facility         Facility `json:"facility,string" msg:"facility"`
	Severity         Severity `json:"severity,string" msg:"severity"`
	Version          Version  `json:"version,string" msg:"version"`
	TimeReportedNum  int64    `json:"-" msg:"timereportednum"`
	TimeGeneratedNum int64    `json:"-" msg:"timegeneratednum"`
	TimeReported     string   `json:"timereported" msg:"timereported"`
	TimeGenerated    string   `json:"timegenerated" msg:"timegenerated"`
	Hostname         string   `json:"hostname" msg:"hostname"`
	Appname          string   `json:"appname" msg:"appname"`
	Procid           string   `json:"procid" msg:"procid"`
	Msgid            string   `json:"msgid" msg:"msgid"`
	Structured       string   `json:"structured" msg:"structured"`
	Message          string   `json:"message" msg:"message"`

	Properties map[string]map[string]string `json:"properties" msg:"properties"`
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
	ConnID uintptr       `json:"-" msg:"-"`
}

// ffjson: noencoder
type JsonRsyslogMessage struct {
	Message       string `json:"msg"`
	TimeReported  string `json:"timereported"`
	TimeGenerated string `json:"timegenerated"`
	Hostname      string `json:"hostname"`
	Priority      string `json:"pri"`
	Appname       string `json:"app-name"`
	Procid        string `json:"procid"`
	Msgid         string `json:"msgid"`
	Uuid          string `json:"uuid"`
	Structured    string `json:"structured-data"`

	Properties map[string]map[string]string `json:"$!"`
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

func (m *TcpUdpParsedMessage) MarshalAll(frmt string) ([]byte, error) {
	switch frmt {
	case "rfc5424":
		return m.Parsed.Fields.Marshal5424()
	case "rfc3164":
		return m.Parsed.Fields.Marshal3164()
	case "json":
		return ffjson.Marshal(&m.Parsed.Fields)
	case "fulljson":
		return ffjson.Marshal(m)
	default:
		return nil, fmt.Errorf("MarshalAll: unknown format '%s'", frmt)
	}
}

// MarshalAll formats the message in the given format
func (m *SyslogMessage) MarshalAll(frmt string) ([]byte, error) {
	switch frmt {
	case "rfc5424":
		return m.Marshal5424()
	case "rfc3164":
		return m.Marshal3164()
	case "json":
		return ffjson.Marshal(m)
	default:
		return nil, fmt.Errorf("MarshalAll: unknown format '%s'", frmt)
	}
}

// Marshal3164 formats the message as a RFC3164 line
func (m *SyslogMessage) Marshal3164() ([]byte, error) {
	b := bytes.NewBuffer(nil)
	procid := strings.TrimSpace(m.Procid)
	if len(procid) > 0 {
		procid = fmt.Sprintf("[%s]", procid)
	}
	hostname := strings.TrimSpace(m.Hostname)
	if len(hostname) == 0 {
		hostname, _ = os.Hostname()
	}
	fmt.Fprintf(
		b, "<%d>%s %s %s%s: %s",
		m.Priority,
		time.Unix(0, m.TimeReportedNum).UTC().Format("Jan _2 15:04:05"),
		hostname,
		m.Appname,
		procid,
		m.Message,
	)
	return b.Bytes(), nil
}

// Marshal5424 formats the message as a RFC5424 line
func (m *SyslogMessage) Marshal5424() (res []byte, err error) {
	err = m.validRfc5424()
	if err != nil {
		return nil, err
	}
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
			if len(name) > 32 {
				name = name[:32]
			}
			fmt.Fprintf(b, " %s=\"%s\"", name, escapeSDParam(value))
		}
		fmt.Fprintf(b, "]")
	}

	if len(m.Message) > 0 {
		fmt.Fprint(b, " ")
		b.Write([]byte(m.Message))
	}
	res = b.Bytes()
	return
}

func (m *SyslogMessage) validRfc5424() error {
	if !utils.PrintableUsASCII(m.Hostname) {
		return invalid5424("Hostname", m.Hostname)
	}
	if len(m.Hostname) > 255 {
		return invalid5424("Hostname", m.Hostname)
	}
	if !utils.PrintableUsASCII(m.Appname) {
		return invalid5424("Appname", m.Appname)
	}
	if len(m.Appname) > 48 {
		return invalid5424("Appname", m.Appname)
	}
	if !utils.PrintableUsASCII(m.Procid) {
		return invalid5424("Procid", m.Procid)
	}
	if len(m.Procid) > 128 {
		return invalid5424("Procid", m.Procid)
	}
	if !utils.PrintableUsASCII(m.Msgid) {
		return invalid5424("Msgid", m.Msgid)
	}
	if len(m.Msgid) > 32 {
		return invalid5424("Msgid", m.Msgid)
	}

	for sid := range m.Properties {
		if !validName(sid) {
			return invalid5424("StructuredData/ID", sid)
		}
		for param, value := range m.Properties[sid] {
			if !validName(param) {
				return invalid5424("StructuredData/Name", param)
			}
			if !utf8.ValidString(value) {
				return invalid5424("StructuredData/Value", value)
			}
		}
	}
	return nil
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

func validName(s string) bool {
	for _, ch := range s {
		if ch < 33 || ch > 126 {
			return false
		}
		if ch == '=' || ch == ']' || ch == '"' {
			return false
		}
	}
	return true
}

type ErrInvalid5424 struct {
	Property string
	Value    interface{}
}

func (e ErrInvalid5424) Error() string {
	return fmt.Sprintf("Message cannot be RFC5424 serialized: %s is invalid ('%v')", e.Property, e.Value)
}

func invalid5424(property string, value interface{}) error {
	return ErrInvalid5424{Property: property, Value: value}
}
