package model

import (
	"strconv"
	"strings"
	"time"
	"unicode"
)

// <PRI>Mmm dd hh:mm:ss HOSTNAME TAG MSG
// or <PRI>Mmm dd hh:mm:ss TAG MSG
// or <PRI>RFC3339 HOSTNAME TAG MSG
// or <PRI>RFC3339 TAG MSG

// TAG may be "TAG" or "TAG:" or "TAG[PID]" or "TAG[PID]:"
// HOSTNAME can be IPv4 (multiple "."), IPv6 (multiple ":"), or a hostname without domain.

// if <PRI> is not present, we assume it is just MSG

func ParseRfc3164Format(m string) (*SyslogMessage, error) {
	smsg := SyslogMessage{}
	def_smsg := SyslogMessage{}
	def_smsg.Message = m
	n := time.Now()
	def_smsg.TimeGenerated = &n
	def_smsg.TimeReported = &n

	smsg.Properties = map[string]interface{}{}
	if !strings.HasPrefix(m, "<") {
		return &def_smsg, nil
	}
	end_pri := strings.Index(m, ">")
	if end_pri <= 1 {
		return &def_smsg, nil
	}
	pri_s := m[1:end_pri]
	pri_num, err := strconv.Atoi(pri_s)
	if err != nil {
		return &def_smsg, nil
	}
	smsg.Priority = Priority(pri_num)
	smsg.Facility = Facility(pri_num / 8)
	smsg.Severity = Severity(pri_num % 8)

	if len(m) <= (end_pri + 1) {
		return &smsg, nil
	}
	m = strings.TrimSpace(m[end_pri+1:])
	s := strings.Split(m, " ")
	if m[0] >= byte('0') && m[0] <= byte('9') {
		// RFC3339
		t1, e := time.Parse(time.RFC3339Nano, s[0])
		if e != nil {
			t2, e := time.Parse(time.RFC3339, s[0])
			if e != nil {
				smsg.Message = m
				smsg.TimeGenerated = def_smsg.TimeGenerated
				smsg.TimeReported = def_smsg.TimeReported
				return &smsg, nil
			}
			smsg.TimeGenerated = &t2
			smsg.TimeReported = &t2
		} else {
			smsg.TimeGenerated = &t1
			smsg.TimeReported = &t1
		}
		if len(s) == 1 {
			return &smsg, nil
		}
		s = s[1:]
	} else {
		// old unix timestamp
		if len(s) < 3 {
			smsg.Message = m
			smsg.TimeGenerated = def_smsg.TimeGenerated
			smsg.TimeReported = def_smsg.TimeReported
			return &smsg, nil
		}
		timestamp_s := strings.Join(s[0:3], " ")
		t, e := time.Parse(time.Stamp, timestamp_s)
		if e != nil {
			smsg.Message = m
			smsg.TimeGenerated = def_smsg.TimeGenerated
			smsg.TimeReported = def_smsg.TimeReported
			return &smsg, nil
		}
		smsg.TimeGenerated = &t
		smsg.TimeReported = &t
		if len(s) == 3 {
			return &smsg, nil
		}
		s = s[3:]
	}

	if len(s) == 1 {
		smsg.Message = s[0]
		return &smsg, nil
	}

	if len(s) == 2 {
		// we either have HOSTNAME/MESSAGE or TAG/MESSAGE or HOSTNAME/TAG
		if strings.Count(s[0], ":") == 7 || strings.Count(s[0], ".") == 3 {
			// looks like an IPv6/IPv4 address
			smsg.Hostname = s[0]
			if strings.ContainsAny(s[1], "[]:") {
				smsg.Appname, smsg.Procid = ParseTag(s[1])
			} else {
				smsg.Message = s[1]
			}
			return &smsg, nil
		}
		if strings.ContainsAny(s[0], "[]:") {
			smsg.Appname, smsg.Procid = ParseTag(s[0])
			smsg.Message = s[1]
			return &smsg, nil
		}
		if strings.ContainsAny(s[1], "[]:") {
			smsg.Hostname = s[0]
			smsg.Appname, smsg.Procid = ParseTag(s[0])
			return &smsg, nil
		}
		smsg.Appname = s[0]
		smsg.Message = s[1]
		return &smsg, nil
	}

	if strings.ContainsAny(s[0], "[]:") || !isHostname(s[0]) {
		// hostname is omitted
		smsg.Appname, smsg.Procid = ParseTag(s[0])
		smsg.Message = strings.Join(s[1:], " ")
		return &smsg, nil
	}
	smsg.Hostname = s[0]
	smsg.Appname, smsg.Procid = ParseTag(s[1])
	smsg.Message = strings.Join(s[2:], " ")
	return &smsg, nil
}

func ParseTag(tag string) (appname string, procid string) {
	tag = strings.Trim(tag, ":")
	i := strings.Index(tag, "[")
	if i >= 0 {
		j := strings.Index(tag, "]")
		if j > i {
			procid = tag[i:j]
		} else {
			procid = tag[i:]
		}
		if i > 0 {
			appname = tag[0:i]
		}
	} else {
		appname = tag
	}
	return
}

func isHostname(s string) bool {
	for _, r := range s {
		if (!unicode.IsLetter(r)) && (!unicode.IsNumber(r)) && r != '.' && r != ':' {
			return false
		}
	}
	return true
}
