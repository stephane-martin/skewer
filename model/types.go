package model

import (
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/awnumar/memguard"
	"github.com/stephane-martin/skewer/utils/sbox"
)

var Facilities map[Facility]string = map[Facility]string{
	0:  "kern",
	1:  "user",
	2:  "mail",
	3:  "daemon",
	4:  "auth",
	5:  "syslog",
	6:  "lpr",
	7:  "news",
	8:  "uucp",
	9:  "clock",
	10: "authpriv",
	11: "ftp",
	12: "ntp",
	13: "logaudit",
	14: "logalert",
	15: "cron",
	16: "local0",
	17: "local1",
	18: "local2",
	19: "local3",
	20: "local4",
	21: "local5",
	22: "local6",
	23: "local7",
}

var RFacilities map[string]Facility

func init() {
	RFacilities = map[string]Facility{}
	for k, v := range Facilities {
		RFacilities[v] = k
	}
}

type Priority int32
type Facility int32
type Severity int32
type Version int32

func (f Facility) String() string {
	if s, ok := Facilities[f]; ok {
		return s
	}
	return "user"
}

func FacilityFromString(s string) Facility {
	if f, ok := RFacilities[s]; ok {
		return f
	}
	return 1
}

type RegularSyslog struct {
	Priority      Priority                     `json:"priority"`
	Facility      Facility                     `json:"facility"`
	Severity      Severity                     `json:"severity"`
	Version       Version                      `json:"version"`
	TimeReported  time.Time                    `json:"timereported"`
	TimeGenerated time.Time                    `json:"timegenerated"`
	HostName      string                       `json:"hostname"`
	AppName       string                       `json:"appname"`
	ProcId        string                       `json:"procid"`
	MsgId         string                       `json:"msgid"`
	Structured    string                       `json:"structured"`
	Message       string                       `json:"message"`
	Properties    map[string]map[string]string `json:"properties"`
}

func (m *RegularSyslog) Internal() (res *SyslogMessage) {
	res = &SyslogMessage{
		Priority:         m.Priority,
		Facility:         m.Facility,
		Severity:         m.Severity,
		Version:          m.Version,
		TimeReportedNum:  m.TimeReported.UnixNano(),
		TimeGeneratedNum: m.TimeGenerated.UnixNano(),
		HostName:         m.HostName,
		AppName:          m.AppName,
		ProcId:           m.ProcId,
		MsgId:            m.MsgId,
		Structured:       m.Structured,
		Message:          m.Message,
	}
	res.SetAllProperties(m.Properties)
	return res
}

func (m *SyslogMessage) Regular() (reg *RegularSyslog) {
	return &RegularSyslog{
		Priority:      m.Priority,
		Facility:      m.Facility,
		Severity:      m.Severity,
		Version:       m.Version,
		TimeReported:  time.Unix(0, m.TimeReportedNum),
		TimeGenerated: time.Unix(0, m.TimeGeneratedNum),
		HostName:      m.HostName,
		AppName:       m.AppName,
		ProcId:        m.ProcId,
		MsgId:         m.MsgId,
		Structured:    m.Structured,
		Message:       m.Message,
		Properties:    m.GetAllProperties(),
	}
}

func (m *SyslogMessage) RegularJson() ([]byte, error) {
	return json.Marshal(m.Regular())
}

func (m *ParsedMessage) Regular() (reg *RegularSyslog) {
	reg = m.Fields.Regular()
	if _, ok := reg.Properties["skewer"]; !ok {
		reg.Properties["skewer"] = map[string]string{}
	}
	reg.Properties["skewer"]["client"] = m.Client
	reg.Properties["skewer"]["path"] = m.UnixSocketPath
	reg.Properties["skewer"]["port"] = strconv.FormatInt(int64(m.LocalPort), 10)
	return reg
}

func (m *ParsedMessage) RegularJson() ([]byte, error) {
	return json.Marshal(m.Regular())
}

type JsonRsyslogMessage struct {
	// used to parsed JSON input from rsyslog
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

func (m *SyslogMessage) GetTimeReported() time.Time {
	return time.Unix(0, m.TimeReportedNum).UTC()
}

func (m *SyslogMessage) GetTimeGenerated() time.Time {
	return time.Unix(0, m.TimeGeneratedNum).UTC()
}

func (m *SyslogMessage) Date() string {
	return m.GetTimeReported().Format("2006-01-02")
}

func (m *SyslogMessage) ClearProperties() {
	m.Properties = Properties{
		Map: map[string]*InnerProperties{},
	}
}

func (m *SyslogMessage) ClearDomain(domain string) {
	if m.Properties.Map == nil {
		m.Properties.Map = map[string]*InnerProperties{}
	}
	m.Properties.Map[domain] = &InnerProperties{
		Map: map[string]string{},
	}
}

func (m *SyslogMessage) GetProperty(domain, key string) string {
	if len(m.Properties.Map) == 0 {
		return ""
	}
	kv := m.Properties.Map[domain]
	if kv == nil {
		return ""
	}
	if len(kv.Map) == 0 {
		return ""
	}
	return kv.Map[key]
}

func (m *SyslogMessage) SetProperty(domain, key, value string) {
	if m.Properties.Map == nil {
		m.Properties.Map = map[string]*InnerProperties{}
	}
	kv := m.Properties.Map[domain]
	if kv == nil {
		m.Properties.Map[domain] = &InnerProperties{
			Map: map[string]string{},
		}
		kv = m.Properties.Map[domain]
	}
	if kv.Map == nil {
		kv.Map = map[string]string{}
	}
	kv.Map[key] = value
}

func (m *SyslogMessage) SetAllProperties(all map[string](map[string]string)) {
	m.ClearProperties()
	for domain, kv := range all {
		for k, v := range kv {
			m.SetProperty(domain, k, v)
		}
	}
}

func (m *SyslogMessage) GetAllProperties() (res map[string](map[string]string)) {
	res = map[string](map[string]string){}
	if len(m.Properties.Map) == 0 {
		return res
	}
	for domain, inner := range m.Properties.Map {
		if inner == nil {
			continue
		}
		if len(inner.Map) == 0 {
			continue
		}
		res[domain] = map[string]string{}
		for k, v := range inner.Map {
			res[domain][k] = v
		}
	}
	return res
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
	err = m.Unmarshal(dec)
	return err
}
