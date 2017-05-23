package server

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

func ParseRfc5424Format(m string) (*SyslogMessage, error) {
	// HEADER = PRI VERSION SP TIMESTAMP SP HOSTNAME SP APP-NAME SP PROCID SP MSGID
	// PRI = "<" PRIVAL ">"
	// SYSLOG-MSG = HEADER SP STRUCTURED-DATA [SP MSG]

	smsg := SyslogMessage{}
	splits := strings.SplitN(m, " ", 7)

	if len(splits) < 7 {
		return nil, fmt.Errorf("Message does not have enough parts")
	}

	var err error
	smsg.Priority, smsg.Facility, smsg.Severity, smsg.Version, err = ParsePriorityVersion(splits[0])
	if err != nil {
		return nil, err
	}

	if splits[1] == "-" {
		smsg.TimeReported = nil
	} else {
		t, err := time.Parse(time.RFC3339, splits[1])
		if err != nil {
			return nil, fmt.Errorf("Invalid timestamp: %s", splits[1])
		}
		smsg.TimeReported = &t
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
	structured_and_msg := strings.TrimSpace(splits[6])
	if strings.HasPrefix(structured_and_msg, "-") {
		// structured data is empty
		smsg.Message = strings.TrimSpace(structured_and_msg[1:])
	} else if strings.HasPrefix(structured_and_msg, "[") {
		s1, s2, err := SplitStructuredAndMessage(structured_and_msg)
		if err != nil {
			return nil, err
		}
		smsg.Structured = s1
		smsg.Message = s2
	} else {
		return nil, fmt.Errorf("Invalid structured data")
	}

	smsg.Properties = map[string]interface{}{}

	return &smsg, nil
}

func SplitStructuredAndMessage(structured_and_msg string) (string, string, error) {
	length := len(structured_and_msg)
	for i := 0; i < length; i++ {
		if structured_and_msg[i] == ']' {
			if i == (length - 1) {
				return structured_and_msg, "", nil
			}
			if structured_and_msg[i+1] == ' ' {
				return structured_and_msg[:i+1], strings.TrimSpace(structured_and_msg[i+1:]), nil
			}
		}
	}
	return "", "", fmt.Errorf("Invalid structured data")
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
