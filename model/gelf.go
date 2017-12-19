package model

import (
	"fmt"

	"gopkg.in/Graylog2/go-gelf.v2/gelf"
)

func (m *SyslogMessage) ToGelfMessage() *gelf.Message {
	gelfm := gelf.Message{
		Version:  "1.1",
		Host:     m.Hostname,
		Short:    m.Message,
		Full:     "",
		TimeUnix: float64(m.TimeReportedNum) / 1000000000,
		Level:    int32(m.Severity),
		RawExtra: nil,
	}
	gelfm.Extra = map[string]interface{}{}
	for domain, props := range m.Properties {
		gelfm.Extra[domain] = map[string]string{}
		for k, v := range props {
			(gelfm.Extra[domain]).(map[string]string)[k] = v
		}
	}
	gelfm.Extra["facility"] = m.Facility.String()
	if len(m.Appname) > 0 {
		gelfm.Extra["appname"] = m.Appname
	}
	if len(m.Procid) > 0 {
		gelfm.Extra["procid"] = m.Procid
	}
	if len(m.Msgid) > 0 {
		gelfm.Extra["msgid"] = m.Msgid
	}

	return &gelfm
}

func (m *SyslogMessage) FromGelfMessage(gelfm *gelf.Message) {
	m.TimeReported = ""
	m.TimeGenerated = ""
	m.Structured = ""
	if gelfm == nil {
		m.Message = ""
		m.TimeReportedNum = 0
		m.TimeGeneratedNum = 0
		m.Hostname = ""
		m.Version = 0
		m.Severity = 0
		m.Facility = 0
		m.Priority = 0
		m.Appname = ""
		m.Procid = ""
		m.Msgid = ""
		m.Properties = map[string]map[string]string{}
		return
	}
	if len(gelfm.Full) > 0 {
		m.Message = gelfm.Full
	} else {
		m.Message = gelfm.Short
	}
	m.TimeReportedNum = int64(gelfm.TimeUnix * 1000000000)
	m.TimeGeneratedNum = m.TimeReportedNum
	m.Hostname = gelfm.Host
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

	m.Appname = ""
	if appname, ok := gelfm.Extra["appname"]; ok {
		m.Appname = fmt.Sprintf("%s", appname)
	}
	m.Procid = ""
	if procid, ok := gelfm.Extra["procid"]; ok {
		m.Procid = fmt.Sprintf("%s", procid)
	}
	m.Msgid = ""
	if msgid, ok := gelfm.Extra["msgid"]; ok {
		m.Msgid = fmt.Sprintf("%s", msgid)
	}

	m.Properties = map[string]map[string]string{}
	m.Properties["gelf"] = map[string]string{}
	for k, v := range gelfm.Extra {
		switch k {
		case "facility", "appname", "procid", "msgid":
		default:
			if vs, ok := v.(string); ok {
				m.Properties["gelf"][k] = vs
			} else if vm, ok := v.(map[string]string); ok {
				if _, ok := m.Properties[k]; !ok {
					m.Properties[k] = map[string]string{}
				}
				for k1, v1 := range vm {
					m.Properties[k][k1] = v1
				}
			} else {
				m.Properties["gelf"][k] = fmt.Sprintf("%s", v)
			}
		}
	}
}
