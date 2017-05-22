package server

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

type Priority int
type Facility int
type Severity int
type Version int

// HEADER = PRI VERSION SP TIMESTAMP SP HOSTNAME SP APP-NAME SP PROCID SP MSGID
// PRI = "<" PRIVAL ">"
// SYSLOG-MSG = HEADER SP STRUCTURED-DATA [SP MSG]
type SyslogMessage struct {
	Priority   Priority  `json:"priority,string"`
	Facility   Facility  `json:"facility,string"`
	Severity   Severity  `json:"severity,string"`
	Version    Version   `json:"version,string"`
	Timestamp  time.Time `json:"timestamp"`
	Hostname   string    `json:"hostname"`
	Appname    string    `json:"appname"`
	Procid     string    `json:"procid"`
	Msgid      string    `json:"msgid"`
	Structured string    `json:"structured"`
	Message    string    `json:"message"`
}

var SyslogMessageFmt string = `Facility: %d
Severity: %d
Version: %d
Timestamp: %s
Hostname: %s
Appname: %s
ProcID: %s
MsgID: %s
Structured: %s
Message: %s`

func (m *SyslogMessage) String() string {
	return fmt.Sprintf(
		SyslogMessageFmt,
		m.Facility,
		m.Severity,
		m.Version,
		m.Timestamp.Format(time.RFC3339),
		m.Hostname,
		m.Appname,
		m.Procid,
		m.Msgid,
		m.Structured,
		m.Message)
}

func Parse(m string) (*SyslogMessage, error) {
	smsg := SyslogMessage{}
	splits := strings.SplitN(m, " ", 8)

	if len(splits) < 7 {
		return nil, fmt.Errorf("Message does not have enough parts")
	}

	var err error
	smsg.Priority, smsg.Facility, smsg.Severity, smsg.Version, err = ParsePriorityVersion(splits[0])
	if err != nil {
		return nil, err
	}

	if splits[1] == "-" {
		smsg.Timestamp = time.Time{}
	} else {
		t, err := time.Parse(time.RFC3339, splits[1])
		if err != nil {
			return nil, fmt.Errorf("Invalid timestamp: %s", splits[1])
		}
		smsg.Timestamp = t
	}

	if splits[2] != "-" {
		smsg.Hostname = splits[2]
	}
	if splits[3] != "-" {
		smsg.Appname = splits[3]
	}
	if splits[4] != "-" {
		smsg.Procid = splits[4]
	}
	if splits[5] != "-" {
		smsg.Msgid = splits[5]
	}
	if splits[6] != "-" {
		smsg.Structured = splits[6]
	}
	if len(splits) == 8 {
		smsg.Message = strings.TrimSpace(splits[7])
	}

	return &smsg, nil
}

func ParsePriorityVersion(pv string) (Priority, Facility, Severity, Version, error) {
	if pv[0] != byte('<') {
		return 0, 0, 0, 0, fmt.Errorf("Invalid priority")
	}
	i := strings.Index(pv, ">")
	if i < 2 {
		return 0, 0, 0, 0, fmt.Errorf("Invalid priority")
	}
	if len(pv) <= (i + 1) {
		return 0, 0, 0, 0, fmt.Errorf("Invalid priority")
	}
	p, err := strconv.Atoi(pv[1:i])
	if err != nil {
		return 0, 0, 0, 0, fmt.Errorf("Invalid priority")
	}
	f := Facility(p / 8)
	s := Severity(p % 8)
	v, err := strconv.Atoi(pv[i+1:])
	if err != nil {
		return 0, 0, 0, 0, fmt.Errorf("Invalid priority")
	}

	return Priority(p), f, s, Version(v), nil
}
