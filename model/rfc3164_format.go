package model

import (
	"bytes"
	"strconv"
	"time"
	uni "unicode"

	"golang.org/x/text/encoding"
	"golang.org/x/text/encoding/unicode"
)

// <PRI>Mmm dd hh:mm:ss HOSTNAME TAG MSG
// or <PRI>Mmm dd hh:mm:ss TAG MSG
// or <PRI>RFC3339 HOSTNAME TAG MSG
// or <PRI>RFC3339 TAG MSG

// TAG may be "TAG" or "TAG:" or "TAG[PID]" or "TAG[PID]:"
// HOSTNAME can be IPv4 (multiple "."), IPv6 (multiple ":"), or a hostname without domain.

// if <PRI> is not present, we assume it is just MSG

func pair2str(s1 []byte, s2 []byte) (string, string) {
	return string(s1), string(s2)
}

func ParseRfc3164Format(m []byte, decoder *encoding.Decoder) (smsg SyslogMessage, err error) {
	if decoder == nil {
		decoder = unicode.UTF8.NewDecoder()
	}
	m, err = decoder.Bytes(m)
	if err != nil {
		return smsg, &InvalidEncodingError{Err: err}
	}
	m = bytes.TrimSpace(m)

	def_smsg := SyslogMessage{Message: string(m)}
	n := time.Now().UnixNano()
	def_smsg.TimeGeneratedNum = n
	def_smsg.TimeReportedNum = n
	smsg.TimeGeneratedNum = n
	smsg.Properties = map[string]map[string]string{}

	if !bytes.HasPrefix(m, []byte("<")) {
		return def_smsg, nil
	}
	end_pri := bytes.Index(m, []byte(">"))
	if end_pri <= 1 {
		return def_smsg, nil
	}
	pri_s := m[1:end_pri]
	pri_num, err := strconv.Atoi(string(pri_s))
	if err != nil {
		return def_smsg, nil
	}
	smsg.Priority = Priority(pri_num)
	smsg.Facility = Facility(pri_num / 8)
	smsg.Severity = Severity(pri_num % 8)

	if len(m) <= (end_pri + 1) {
		return smsg, nil
	}
	m = bytes.TrimSpace(m[end_pri+1:])
	if len(m) == 0 {
		return smsg, nil
	}

	s := bytes.Split(m, SP)
	if m[0] >= byte('0') && m[0] <= byte('9') {
		// RFC3339
		s0 := string(s[0])
		t1, e := time.Parse(time.RFC3339Nano, s0)
		if e != nil {
			t2, e := time.Parse(time.RFC3339, s0)
			if e != nil {
				smsg.Message = string(m)
				smsg.TimeReportedNum = def_smsg.TimeReportedNum
				return smsg, nil
			}
			smsg.TimeReportedNum = t2.UnixNano()
		} else {
			smsg.TimeReportedNum = t1.UnixNano()
		}
		if len(s) == 1 {
			return smsg, nil
		}
		s = s[1:]
	} else {
		// old unix timestamp
		if len(s) < 3 {
			smsg.Message = string(m)
			smsg.TimeReportedNum = def_smsg.TimeReportedNum
			return smsg, nil
		}
		timestamp_b := bytes.Join(s[0:3], SP)
		t, e := time.Parse(time.Stamp, string(timestamp_b))
		if e != nil {
			smsg.Message = string(m)
			smsg.TimeReportedNum = def_smsg.TimeReportedNum
			return smsg, nil
		}
		t = t.AddDate(time.Now().Year(), 0, 0)
		smsg.TimeReportedNum = t.UnixNano()
		if len(s) == 3 {
			return smsg, nil
		}
		s = s[3:]
	}

	if len(s) == 1 {
		smsg.Message = string(s[0])
		return smsg, nil
	}

	if len(s) == 2 {
		// we either have HOSTNAME/MESSAGE or TAG/MESSAGE or HOSTNAME/TAG
		if bytes.Count(s[0], []byte(":")) == 7 || bytes.Count(s[0], []byte(".")) == 3 {
			// looks like an IPv6/IPv4 address
			smsg.Hostname = string(s[0])
			if bytes.ContainsAny(s[1], "[]:") {

				smsg.Appname, smsg.Procid = pair2str(parseTag(s[1]))
			} else {
				smsg.Message = string(s[1])
			}
			return smsg, nil
		}
		if bytes.ContainsAny(s[0], "[]:") {
			smsg.Appname, smsg.Procid = pair2str(parseTag(s[0]))
			smsg.Message = string(s[1])
			return smsg, nil
		}
		if bytes.ContainsAny(s[1], "[]:") {
			smsg.Hostname = string(s[0])
			smsg.Appname, smsg.Procid = pair2str(parseTag(s[0]))
			return smsg, nil
		}
		smsg.Appname = string(s[0])
		smsg.Message = string(s[1])
		return smsg, nil
	}

	if bytes.ContainsAny(s[0], "[]:") || !isHostname(s[0]) {
		// hostname is omitted
		smsg.Appname, smsg.Procid = pair2str(parseTag(s[0]))
		smsg.Message = string(bytes.Join(s[1:], SP))
		return smsg, nil
	}
	smsg.Hostname = string(s[0])
	smsg.Appname, smsg.Procid = pair2str(parseTag(s[1]))
	smsg.Message = string(bytes.Join(s[2:], SP))
	return smsg, nil
}

func parseTag(tag []byte) (appname []byte, procid []byte) {
	tag = bytes.Trim(tag, ":")
	i := bytes.Index(tag, []byte("["))
	if i >= 0 && len(tag) > (i+1) {
		j := bytes.Index(tag, []byte("]"))
		if j > i {
			procid = tag[i+1 : j]
		} else {
			procid = tag[i+1:]
		}
		if i > 0 {
			appname = tag[0:i]
		}
	} else {
		appname = tag
	}
	return
}

func isHostname(s []byte) bool {
	for _, r := range string(s) {
		if (!uni.IsLetter(r)) && (!uni.IsNumber(r)) && r != '.' && r != ':' {
			return false
		}
	}
	return true
}
