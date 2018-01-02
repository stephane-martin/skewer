package model

import (
	"fmt"
	"time"

	"github.com/stephane-martin/skewer/utils"
	"gopkg.in/Graylog2/go-gelf.v2/gelf"
)

func (m *FullMessage) ToGelfMessage() *gelf.Message {
	gm := m.Parsed.ToGelfMessage()
	if m.Uid != utils.ZeroUid {
		gm.Extra["skewer_uid"] = m.Uid.String()
	}
	if m.Txnr > 0 {
		gm.Extra["txnr"] = m.Txnr
	}
	return gm
}

func (m *ParsedMessage) ToGelfMessage() *gelf.Message {
	gm := m.Fields.ToGelfMessage()
	if len(m.Client) > 0 {
		gm.Extra["client"] = m.Client
	}
	if m.LocalPort > 0 {
		gm.Extra["port"] = m.LocalPort
	}
	if len(m.UnixSocketPath) > 0 {
		gm.Extra["socket_path"] = m.UnixSocketPath
	}
	return gm
}

func (m *SyslogMessage) ToGelfMessage() *gelf.Message {
	gelfm := gelf.Message{
		Version:  "1.1",
		Host:     m.HostName,
		Short:    m.Message,
		Full:     "",
		TimeUnix: float64(m.TimeReportedNum) / 1000000000,
		Level:    int32(m.Severity),
		Facility: m.Facility.String(),
		RawExtra: nil,
	}
	gelfm.Extra = map[string]interface{}{}
	for domain, props := range m.Properties.GetMap() {
		gelfm.Extra[domain] = map[string]string{}
		for k, v := range props.GetMap() {
			(gelfm.Extra[domain]).(map[string]string)[k] = v
		}
	}
	gelfm.Extra["facility"] = gelfm.Facility
	if len(m.AppName) > 0 {
		gelfm.Extra["appname"] = m.AppName
	}
	if len(m.ProcId) > 0 {
		gelfm.Extra["procid"] = m.ProcId
	}
	if len(m.MsgId) > 0 {
		gelfm.Extra["msgid"] = m.MsgId
	}

	return &gelfm
}

func FromGelfMessage(gelfm *gelf.Message) (msg *SyslogMessage) {
	msg = &SyslogMessage{}
	msg.FromGelfMessage(gelfm)
	return msg
}

func FullFromGelfMessage(gelfm *gelf.Message) (msg *FullMessage) {
	msg = &FullMessage{}
	msg.Parsed.Fields.FromGelfMessage(gelfm)
	return msg
}

func (m *SyslogMessage) FromGelfMessage(gelfm *gelf.Message) {
	//m.TimeReported = ""
	//m.TimeGenerated = ""
	m.Structured = ""
	if gelfm == nil {
		m.Reset()
		return
	}
	m.Message = gelfm.Short
	m.TimeReportedNum = int64(gelfm.TimeUnix * 1000000000)
	m.TimeGeneratedNum = time.Now().UnixNano()
	m.HostName = gelfm.Host
	m.Version = 1
	m.Severity = Severity(gelfm.Level)

	if len(gelfm.Facility) > 0 {
		m.Facility = FacilityFromString(gelfm.Facility)
	} else if fs, ok := gelfm.Extra["facility"]; ok {
		m.Facility = FacilityFromString(fmt.Sprintf("%s", fs))
	} else {
		m.Facility = 1
	}
	m.Priority = Priority(int(m.Facility)*8 + int(m.Severity))

	m.AppName = ""
	if appname, ok := gelfm.Extra["appname"]; ok {
		m.AppName = fmt.Sprintf("%s", appname)
	}
	m.ProcId = ""
	if procid, ok := gelfm.Extra["procid"]; ok {
		m.ProcId = fmt.Sprintf("%s", procid)
	}
	m.MsgId = ""
	if msgid, ok := gelfm.Extra["msgid"]; ok {
		m.MsgId = fmt.Sprintf("%s", msgid)
	}
	m.ClearProperties()
	if len(gelfm.Full) > 0 {
		m.SetProperty("gelf", "full", gelfm.Full)
	}
	for k, v := range gelfm.Extra {
		switch k {
		case "facility", "appname", "procid", "msgid":
		default:
			if vs, ok := v.(string); ok {
				m.SetProperty("gelf", k, vs)
			} else if vm, ok := v.(map[string]string); ok {
				for k1, v1 := range vm {
					m.SetProperty(k, k1, v1)
				}
			} else {
				m.SetProperty("gelf", k, fmt.Sprintf("%s", v))
			}
		}
	}
}
