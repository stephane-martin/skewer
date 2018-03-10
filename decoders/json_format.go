package decoders

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/pquerna/ffjson/ffjson"
	"github.com/stephane-martin/skewer/decoders/base"
	"github.com/stephane-martin/skewer/model"
	"gopkg.in/Graylog2/go-gelf.v2/gelf"
)

func pGELF(m []byte) ([]*model.SyslogMessage, error) {
	gelfMsg := &gelf.Message{}
	err := gelfMsg.UnmarshalJSON(m)
	if err != nil {
		return nil, &base.UnmarshalingJsonError{err}
	}
	return []*model.SyslogMessage{FromGelfMessage(gelfMsg)}, nil
}

func pJSON(m []byte) ([]*model.SyslogMessage, error) {
	sourceMsg := model.RegularSyslog{}
	err := ffjson.Unmarshal(m, &sourceMsg)
	if err != nil {
		return nil, &base.UnmarshalingJsonError{err}
	}
	return []*model.SyslogMessage{sourceMsg.Internal()}, nil
}

func pRsyslogJSON(m []byte) ([]*model.SyslogMessage, error) {
	sourceMsg := model.JsonRsyslogMessage{}
	err := ffjson.Unmarshal(m, &sourceMsg)
	if err != nil {
		return nil, &base.UnmarshalingJsonError{err}
	}

	pri, err := strconv.Atoi(sourceMsg.Priority)
	if err != nil {
		return nil, &base.InvalidPriorityError{}
	}

	n := time.Now()
	generated := n
	reported := n

	if sourceMsg.TimeReported != "-" && len(sourceMsg.TimeReported) > 0 {
		r, err := time.Parse(time.RFC3339Nano, sourceMsg.TimeReported)
		if err != nil {
			return nil, &base.TimeError{}
		}
		reported = r
	}

	if sourceMsg.TimeGenerated != "-" && len(sourceMsg.TimeGenerated) > 0 {
		g, err := time.Parse(time.RFC3339Nano, sourceMsg.TimeGenerated)
		if err != nil {
			return nil, &base.TimeError{}
		}
		generated = g
	}

	hostname := ""
	if sourceMsg.Hostname != "-" {
		hostname = strings.TrimSpace(sourceMsg.Hostname)
	}

	appname := ""
	if sourceMsg.Appname != "-" {
		appname = strings.TrimSpace(sourceMsg.Appname)
	}

	procid := ""
	if sourceMsg.Procid != "-" {
		procid = strings.TrimSpace(sourceMsg.Procid)
	}

	msgid := ""
	if sourceMsg.Msgid != "-" {
		msgid = strings.TrimSpace(sourceMsg.Msgid)
	}

	structured := ""
	if sourceMsg.Structured != "-" {
		structured = strings.TrimSpace(sourceMsg.Structured)
	}

	msg := model.CleanFactory()
	msg.Priority = model.Priority(pri)
	msg.Facility = model.Facility(pri / 8)
	msg.Severity = model.Severity(pri % 8)
	msg.Version = 1
	msg.TimeReportedNum = reported.UnixNano()
	msg.TimeGeneratedNum = generated.UnixNano()
	msg.HostName = hostname
	msg.AppName = appname
	msg.ProcId = procid
	msg.MsgId = msgid
	msg.Structured = structured
	msg.Message = strings.TrimSpace(sourceMsg.Message)

	for k, v := range sourceMsg.Properties {
		msg.SetProperty("rsyslog", strings.TrimSpace(k), strings.TrimSpace(fmt.Sprintf("%v", v)))
	}

	return []*model.SyslogMessage{msg}, nil
}

/*
{
  "msg": " spamd: clean message (3.9\/5.0) for debian-spamd:110 in 2.2 seconds, 12388 bytes.",
  "rawmsg": "<22>May 20 03:06:48 spamd[6948]: spamd: clean message (3.9\/5.0) for debian-spamd:110 in 2.2 seconds, 12388 bytes.",
  "timereported": "2017-05-20T03:06:48.819757+02:00",
  "hostname": "vmail_container",
  "syslogtag": "spamd[6948]:",
  "inputname": "imuxsock",
  "fromhost": "vmail_container",
  "fromhost-ip": "127.0.0.1",
  "pri": "22",
  "syslogfacility": "2",
  "syslogseverity": "6",
  "timegenerated": "2017-05-20T03:06:48.819757+02:00",
  "programname": "spamd",
  "protocol-version": "0",
  "structured-data": "-",
  "app-name": "spamd",
  "procid": "6948",
  "msgid": "-",
  "uuid": null,
  "$!":
    {
      "pid": 6948,
      "uid": 110,
      "gid": 116,
      "appname": "spamd child",
      "cmd": "spamd child"
    }
}
*/
