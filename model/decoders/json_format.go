package decoders

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/pquerna/ffjson/ffjson"
	"github.com/stephane-martin/skewer/model"
	"golang.org/x/text/encoding"
	"golang.org/x/text/encoding/unicode"
	"gopkg.in/Graylog2/go-gelf.v2/gelf"
)

func ParseGelfFormat(m []byte, decoder *encoding.Decoder) (msg *model.SyslogMessage, rerr error) {
	// we ignore decoder, JSON is always UTF-8
	decoder = unicode.UTF8.NewDecoder()

	var err error
	m, err = decoder.Bytes(m)
	if err != nil {
		return nil, &InvalidEncodingError{Err: err}
	}
	gelfMsg := &gelf.Message{}
	err = gelfMsg.UnmarshalJSON(m)
	if err != nil {
		return nil, &UnmarshalingJsonError{err}
	}
	return FromGelfMessage(gelfMsg), nil
}

func ParseFullJsonFormat(m []byte, decoder *encoding.Decoder) (msg *model.SyslogMessage, rerr error) {
	// we ignore decoder, JSON is always UTF-8
	decoder = unicode.UTF8.NewDecoder()

	var err error
	m, err = decoder.Bytes(m)
	if err != nil {
		return nil, &InvalidEncodingError{Err: err}
	}
	sourceMsg := model.RegularSyslog{}
	err = ffjson.Unmarshal(m, &sourceMsg)
	if err != nil {
		return nil, &UnmarshalingJsonError{err}
	}
	return sourceMsg.Internal(), nil
}

func ParseJsonFormat(m []byte, decoder *encoding.Decoder) (msg *model.SyslogMessage, rerr error) {
	// we ignore decoder, JSON is always UTF-8
	decoder = unicode.UTF8.NewDecoder()

	var err error
	m, err = decoder.Bytes(m)
	if err != nil {
		return nil, &InvalidEncodingError{Err: err}
	}
	sourceMsg := model.JsonRsyslogMessage{}
	err = ffjson.Unmarshal(m, &sourceMsg)
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

	msg = &model.SyslogMessage{
		Priority:         model.Priority(pri),
		Facility:         model.Facility(pri / 8),
		Severity:         model.Severity(pri % 8),
		Version:          1,
		TimeReportedNum:  reported.UnixNano(),
		TimeGeneratedNum: generated.UnixNano(),
		HostName:         hostname,
		AppName:          appname,
		ProcId:           procid,
		MsgId:            msgid,
		Structured:       structured,
		Message:          strings.TrimSpace(sourceMsg.Message),
	}

	for k, v := range sourceMsg.Properties {
		msg.SetProperty("rsyslog", strings.TrimSpace(k), strings.TrimSpace(fmt.Sprintf("%v", v)))
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
