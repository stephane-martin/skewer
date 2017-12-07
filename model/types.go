package model

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/awnumar/memguard"
	"github.com/pquerna/ffjson/ffjson"
	"github.com/stephane-martin/skewer/utils/sbox"
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

type ParsedMessage struct {
	Fields         SyslogMessage `json:"fields" msg:"fields"`
	Client         string        `json:"client,omitempty" msg:"client"`
	LocalPort      int           `json:"local_port,string" msg:"local_port"`
	UnixSocketPath string        `json:"unix_socket_path,omitempty" msg:"unix_socket_path"`
}

// ffjson: skip
type FullMessage struct {
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

	Properties map[string]interface{} `json:"$!"`
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
		m.GetTimeReported().Format(time.RFC3339Nano),
		m.GetTimeGenerated().Format(time.RFC3339Nano),
		m.Hostname,
		m.Appname,
		m.Procid,
		m.Msgid,
		m.Structured,
		m.Message,
		m.Properties,
	)
}

func (m SyslogMessage) GetTimeReported() time.Time {
	return time.Unix(0, m.TimeReportedNum).UTC()
}

func (m SyslogMessage) GetTimeGenerated() time.Time {
	return time.Unix(0, m.TimeGeneratedNum).UTC()
}

func (m SyslogMessage) Date() string {
	return m.GetTimeReported().Format("2006-01-02")
}

// Empty returns true if the message is empty
func (m *SyslogMessage) Empty() bool {
	return len(m.Message) == 0 && len(m.Structured) == 0 && len(m.Properties) == 0
}

/*
func (m *FullMessage) Encrypt(secret *memguard.LockedBuffer) (enc []byte, err error) {
	dec, err := m.MarshalMsg(nil)
	if err != nil {
		return nil, err
	}
	if secret == nil {
		return dec, nil
	}
	enc, err = sbox.Encrypt(dec, secret)
	if err != nil {
		return nil, err
	}
	return enc, err
}
*/

func (m *FullMessage) Decrypt(secret *memguard.LockedBuffer, enc []byte) (err error) {
	if len(enc) == 0 {
		return fmt.Errorf("Empty message")
	}
	var dec []byte
	if secret != nil {
		dec, err = sbox.Decrypt(enc, secret)
		if err != nil {
			return err
		}
	} else {
		dec = enc
	}
	_, err = m.UnmarshalMsg(dec)
	return err
}

func (m *FullMessage) MarshalAll(frmt string) (b []byte, err error) {
	buf := bytes.NewBuffer(nil)
	err = m.Encode(buf, frmt)
	if err == nil {
		b = buf.Bytes()
	}
	return
}

func (m *FullMessage) Encode(b io.Writer, frmt string) error {
	switch frmt {
	case "rfc5424":
		return m.Parsed.Fields.Encode5424(b)
	case "rfc3164":
		return m.Parsed.Fields.Encode3164(b)
	case "json":
		m.Parsed.Fields.TimeReported = m.Parsed.Fields.GetTimeReported().Format(time.RFC3339Nano)
		m.Parsed.Fields.TimeGenerated = m.Parsed.Fields.GetTimeGenerated().Format(time.RFC3339Nano)
		return ffjson.NewEncoder(b).Encode(&m.Parsed.Fields)
	case "fulljson":
		m.Parsed.Fields.TimeReported = m.Parsed.Fields.GetTimeReported().Format(time.RFC3339Nano)
		m.Parsed.Fields.TimeGenerated = m.Parsed.Fields.GetTimeGenerated().Format(time.RFC3339Nano)
		return ffjson.NewEncoder(b).Encode(&m)
	default:
		return fmt.Errorf("MarshalAll: unknown format '%s'", frmt)
	}
}

func (m *SyslogMessage) Encode3164(b io.Writer) (err error) {
	procid := strings.TrimSpace(m.Procid)
	if len(procid) > 0 {
		procid = fmt.Sprintf("[%s]", procid)
	}
	hostname := strings.TrimSpace(m.Hostname)
	if len(hostname) == 0 {
		hostname, _ = os.Hostname()
	}
	_, err = fmt.Fprintf(
		b, "<%d>%s %s %s%s: %s",
		m.Priority,
		m.GetTimeReported().Format("Jan _2 15:04:05"),
		hostname,
		m.Appname,
		procid,
		m.Message,
	)
	return err
}

func (m *SyslogMessage) Encode5424(b io.Writer) (err error) {
	err = m.validRfc5424()

	if err != nil {
		return err
	}

	_, err = fmt.Fprintf(
		b,
		"<%d>1 %s %s %s %s %s ",
		m.Priority,
		m.GetTimeReported().Format(time.RFC3339),
		nilify(m.Hostname),
		nilify(m.Appname),
		nilify(m.Procid),
		nilify(m.Msgid),
	)

	if err != nil {
		return err
	}

	if len(m.Properties) == 0 {
		_, err = fmt.Fprint(b, "-")
		if err != nil {
			return err
		}
	}

	for sid := range m.Properties {
		_, err = fmt.Fprintf(b, "[%s", sid)
		if err != nil {
			return err
		}
		for name, value := range m.Properties[sid] {
			if len(name) > 32 {
				name = name[:32]
			}
			_, err = fmt.Fprintf(b, " %s=\"%s\"", name, escapeSDParam(value))
			if err != nil {
				return err
			}
		}
		_, err = fmt.Fprintf(b, "]")
		if err != nil {
			return err
		}
	}

	if len(m.Message) > 0 {
		_, err = fmt.Fprint(b, " ")
		if err != nil {
			return err
		}
		_, err = b.Write([]byte(m.Message))
		if err != nil {
			return err
		}
	}
	return nil
}
