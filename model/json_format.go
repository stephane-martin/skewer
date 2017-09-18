package model

import (
	"encoding/json"
	"strconv"
	"strings"
	"time"
)

type jsonRsyslogMessage struct {
	Message       string                       `json:"msg"`
	TimeReported  string                       `json:"timereported"`
	TimeGenerated string                       `json:"timegenerated"`
	Hostname      string                       `json:"hostname"`
	Priority      string                       `json:"pri"`
	Appname       string                       `json:"app-name"`
	Procid        string                       `json:"procid"`
	Msgid         string                       `json:"msgid"`
	Uuid          string                       `json:"uuid"`
	Structured    string                       `json:"structured-data"`
	Properties    map[string]map[string]string `json:"$!"`
}

func ParseJsonFormat(m []byte) (msg *SyslogMessage, err error) {
	sourceMsg := jsonRsyslogMessage{}
	err = json.Unmarshal(m, &sourceMsg)
	if err != nil {
		return nil, &UnmarshalingJsonError{err}
	}

	pri, err := strconv.Atoi(sourceMsg.Priority)
	if err != nil {
		return nil, &InvalidPriorityError{}
	}

	n := time.Now()
	generated := n
	reported := n

	if sourceMsg.TimeReported != "-" && len(sourceMsg.TimeReported) > 0 {
		r, err := time.Parse(time.RFC3339Nano, sourceMsg.TimeReported)
		if err != nil {
			return nil, &TimeError{}
		}
		reported = r
	}

	if sourceMsg.TimeGenerated != "-" && len(sourceMsg.TimeGenerated) > 0 {
		g, err := time.Parse(time.RFC3339Nano, sourceMsg.TimeGenerated)
		if err != nil {
			return nil, &TimeError{}
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

	msg = &SyslogMessage{
		Priority:      Priority(pri),
		Facility:      Facility(pri / 8),
		Severity:      Severity(pri % 8),
		Version:       1,
		TimeReported:  reported,
		TimeGenerated: generated,
		Hostname:      hostname,
		Appname:       appname,
		Procid:        procid,
		Msgid:         msgid,
		Structured:    structured,
		Message:       strings.TrimSpace(sourceMsg.Message),
	}

	if len(sourceMsg.Properties) > 0 {
		msg.Properties = sourceMsg.Properties
	}

	return msg, nil
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
