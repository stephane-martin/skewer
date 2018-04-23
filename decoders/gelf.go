package decoders

import (
	"fmt"
	"time"

	"github.com/stephane-martin/skewer/model"
	"gopkg.in/Graylog2/go-gelf.v2/gelf"
)

func pGELF(m []byte) ([]*model.SyslogMessage, error) {
	gelfMsg := &gelf.Message{}
	err := gelfMsg.UnmarshalJSON(m)
	if err != nil {
		return nil, UnmarshalJsonError(err)
	}
	return []*model.SyslogMessage{FromGelfMessage(gelfMsg)}, nil
}

func FromGelfMessage(gelfm *gelf.Message) (msg *model.SyslogMessage) {
	msg = model.CleanFactory()
	fromGelfMessage(msg, gelfm)
	return msg
}

func FullFromGelfMessage(gelfm *gelf.Message) (msg *model.FullMessage) {
	msg = model.FullCleanFactory()
	fromGelfMessage(msg.Fields, gelfm)
	return msg
}

func fromGelfMessage(m *model.SyslogMessage, gelfm *gelf.Message) {
	if m == nil {
		return
	}
	//m.TimeReported = ""
	//m.TimeGenerated = ""
	if gelfm == nil {
		m.Clear()
		return
	}
	m.Structured = ""
	m.Message = gelfm.Short
	m.TimeReportedNum = int64(gelfm.TimeUnix * 1000000000)
	m.TimeGeneratedNum = time.Now().UnixNano()
	m.HostName = gelfm.Host
	m.Version = 1
	m.Severity = model.Severity(gelfm.Level)

	if len(gelfm.Facility) > 0 {
		m.Facility = model.FacilityFromString(gelfm.Facility)
	} else if fs, ok := gelfm.Extra["facility"]; ok {
		m.Facility = model.FacilityFromString(fmt.Sprintf("%s", fs))
	} else {
		m.Facility = 1
	}
	m.SetPriority()

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
