package model

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	sarama "gopkg.in/Shopify/sarama.v1"
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
	TimeReported  time.Time              `json:"timereported,omitempty"`
	TimeGenerated time.Time              `json:"timegenerated,omitempty"`
	Hostname      string                 `json:"hostname"`
	Appname       string                 `json:"appname"`
	Procid        string                 `json:"procid"`
	Msgid         string                 `json:"msgid"`
	Structured    string                 `json:"structured"`
	Message       string                 `json:"message"`
	AuditMessage  interface{}            `json:"audit"`
	Properties    map[string]interface{} `json:"properties"`
}

type RawMessage struct {
	Message   string
	Client    string
	LocalPort int
}

type ParsedMessage struct {
	Fields    *SyslogMessage `json:"fields"`
	Client    string         `json:"client"`
	LocalPort int            `json:"local_port,string"`
}

type TcpUdpParsedMessage struct {
	Parsed    ParsedMessage `json:"parsed"`
	Uid       string        `json:"uid"`
	ConfIndex int           `json:"conf_index"`
}

type RelpRawMessage struct {
	RawMessage
	Txnr int
}

type RelpParsedMessage struct {
	Parsed ParsedMessage `json:"parsed"`
	Txnr   int           `json:"txnr"`
}

func (m *ParsedMessage) ToKafkaMessage(partitionKey string, topic string) (km *sarama.ProducerMessage, err error) {
	value, err := json.Marshal(m)
	if err != nil {
		return nil, err
	}

	kafka_msg := sarama.ProducerMessage{
		Key:       sarama.StringEncoder(partitionKey),
		Value:     sarama.ByteEncoder(value),
		Topic:     topic,
		Timestamp: m.Fields.TimeReported,
	}
	return &kafka_msg, nil
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
		m.TimeGenerated.Format(time.RFC3339),
		m.Hostname,
		m.Appname,
		m.Procid,
		m.Msgid,
		m.Structured,
		m.Message,
		props,
	)
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
		return nil, fmt.Errorf("unknown format")
	}
	if err != nil {
		return nil, err
	}
	// special handling of JSON messages produced by go-audit
	if sm.Appname == "go-audit" {
		var auditMsg interface{}
		err = json.Unmarshal([]byte(sm.Message), &auditMsg)
		if err != nil {
			return sm, nil
		}
		sm.AuditMessage = auditMsg
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
