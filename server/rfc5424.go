package server

import (
	"fmt"
	"strings"
	"strconv"
	"time"
)

type Facility int

type Severity int

type Version int

type Priority struct {
	P int
	F Facility
	S Severity
}

// HEADER = PRI VERSION SP TIMESTAMP SP HOSTNAME SP APP-NAME SP PROCID SP MSGID
// PRI = "<" PRIVAL ">"
type Header struct {
	Priority  Priority
	Version   Version
	Timestamp time.Time
	Hostname  string
	Appname   string
	Procid    string
	Msgid     string
}

// SYSLOG-MSG = HEADER SP STRUCTURED-DATA [SP MSG]
type SyslogMessage struct {
	Header     Header
	Structured string
	Message    string
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
		m.Header.Priority.F,
		m.Header.Priority.S,
		m.Header.Version,
		m.Header.Timestamp.Format(time.RFC3339),
		m.Header.Hostname,
		m.Header.Appname,
		m.Header.Procid,
		m.Header.Msgid,
		m.Structured,
		m.Message)
}

func Parse(m string) (*SyslogMessage, error) {
	smsg := SyslogMessage{}
	splits := strings.SplitN(m, " ", 8)

	if len(splits) < 7 {
		return nil, fmt.Errorf("Message does not have enough parts")
	}

	p, v, err := ParsePriorityVersion(splits[0])
	if err != nil {
		return nil, err
	}
	smsg.Header.Priority = p
	smsg.Header.Version = v

	if splits[1] == "-" {
		smsg.Header.Timestamp = time.Time{}
	} else {
		t, err := time.Parse(time.RFC3339, splits[1])
		if err != nil {
			return nil, fmt.Errorf("Invalid timestamp: %s", splits[1])	
		}
		smsg.Header.Timestamp = t
	}

	if splits[2] != "-" {
		smsg.Header.Hostname = splits[2]
	}
	if splits[3] != "-" {
		smsg.Header.Appname = splits[3]
	}
	if splits[4] != "-" {
		smsg.Header.Procid = splits[4]
	}
	if splits[5] != "-" {
		smsg.Header.Msgid = splits[5]
	}
	if splits[6] != "-" {
		smsg.Structured = splits[6]
	}
	if len(splits) == 8 {
		smsg.Message = strings.TrimSpace(splits[7])
	}

	return &smsg, nil
}

func ParsePriorityVersion(pv string) (Priority, Version, error) {
	p := Priority{}
	var v Version = 0
	if pv[0] != byte('<') {
		return p, v, fmt.Errorf("Invalid priority")
	}
	i := strings.Index(pv, ">")
	if i < 2 {
		return p, v, fmt.Errorf("Invalid priority")	
	}
	if len(pv) <= (i+1) {
		return p, v, fmt.Errorf("Invalid priority")		
	}
	n, err := strconv.Atoi(pv[1:i])
	if err != nil {
		return p, v, fmt.Errorf("Invalid priority")		
	}
	p.P = n
	p.F = Facility(n/8)
	p.S = Severity(n%8)
	n, err = strconv.Atoi(pv[i+1:])
	if err != nil {
		return p, v, fmt.Errorf("Invalid priority")			
	}
	v = Version(n)

	return p, v, nil
	
}
