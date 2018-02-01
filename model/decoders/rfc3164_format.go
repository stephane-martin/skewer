package decoders

import (
	"bytes"
	"strconv"
	"time"
	uni "unicode"

	"github.com/stephane-martin/skewer/model"
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

func p3164(m []byte, decoder *encoding.Decoder) (smsg *model.SyslogMessage, err error) {
	if decoder == nil {
		decoder = unicode.UTF8.NewDecoder()
	}
	m, err = decoder.Bytes(m)
	if err != nil {
		return smsg, &InvalidEncodingError{Err: err}
	}
	m = bytes.TrimSpace(m)

	defaultMsg := model.CleanFactory()
	defaultMsg.Message = string(m)

	smsg = model.CleanFactory()
	n := time.Now().UnixNano()
	defaultMsg.TimeGeneratedNum = n
	defaultMsg.TimeReportedNum = n
	smsg.TimeGeneratedNum = n

	if !bytes.HasPrefix(m, []byte("<")) {
		model.Free(smsg)
		return defaultMsg, nil
	}
	priEnd := bytes.Index(m, []byte(">"))
	if priEnd <= 1 {
		model.Free(smsg)
		return defaultMsg, nil
	}
	priStr := m[1:priEnd]
	priNum, err := strconv.Atoi(string(priStr))
	if err != nil {
		model.Free(smsg)
		return defaultMsg, nil
	}
	smsg.Priority = model.Priority(priNum)
	smsg.Facility = model.Facility(priNum / 8)
	smsg.Severity = model.Severity(priNum % 8)

	if len(m) <= (priEnd + 1) {
		model.Free(defaultMsg)
		return smsg, nil
	}
	m = bytes.TrimSpace(m[priEnd+1:])
	if len(m) == 0 {
		model.Free(defaultMsg)
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
				smsg.TimeReportedNum = defaultMsg.TimeReportedNum
				model.Free(defaultMsg)
				return smsg, nil
			}
			smsg.TimeReportedNum = t2.UnixNano()
		} else {
			smsg.TimeReportedNum = t1.UnixNano()
		}
		if len(s) == 1 {
			model.Free(defaultMsg)
			return smsg, nil
		}
		s = s[1:]
	} else {
		// old unix timestamp
		if len(s) < 3 {
			smsg.Message = string(m)
			smsg.TimeReportedNum = defaultMsg.TimeReportedNum
			model.Free(defaultMsg)
			return smsg, nil
		}
		timestampBytes := bytes.Join(s[0:3], SP)
		t, e := time.Parse(time.Stamp, string(timestampBytes))
		if e != nil {
			smsg.Message = string(m)
			smsg.TimeReportedNum = defaultMsg.TimeReportedNum
			model.Free(defaultMsg)
			return smsg, nil
		}
		t = t.AddDate(time.Now().Year(), 0, 0)
		smsg.TimeReportedNum = t.UnixNano()
		if len(s) == 3 {
			model.Free(defaultMsg)
			return smsg, nil
		}
		s = s[3:]
	}

	if len(s) == 1 {
		smsg.Message = string(s[0])
		model.Free(defaultMsg)
		return smsg, nil
	}

	if len(s) == 2 {
		// we either have HOSTNAME/MESSAGE or TAG/MESSAGE or HOSTNAME/TAG
		if bytes.Count(s[0], []byte(":")) == 7 || bytes.Count(s[0], []byte(".")) == 3 {
			// looks like an IPv6/IPv4 address
			smsg.HostName = string(s[0])
			if bytes.ContainsAny(s[1], "[]:") {

				smsg.AppName, smsg.ProcId = pair2str(parseTag(s[1]))
			} else {
				smsg.Message = string(s[1])
			}
			model.Free(defaultMsg)
			return smsg, nil
		}
		if bytes.ContainsAny(s[0], "[]:") {
			smsg.AppName, smsg.ProcId = pair2str(parseTag(s[0]))
			smsg.Message = string(s[1])
			model.Free(defaultMsg)
			return smsg, nil
		}
		if bytes.ContainsAny(s[1], "[]:") {
			smsg.HostName = string(s[0])
			smsg.AppName, smsg.ProcId = pair2str(parseTag(s[0]))
			model.Free(defaultMsg)
			return smsg, nil
		}
		smsg.AppName = string(s[0])
		smsg.Message = string(s[1])
		model.Free(defaultMsg)
		return smsg, nil
	}

	if bytes.ContainsAny(s[0], "[]:") || !isHostname(s[0]) {
		// hostname is omitted
		smsg.AppName, smsg.ProcId = pair2str(parseTag(s[0]))
		smsg.Message = string(bytes.Join(s[1:], SP))
		model.Free(defaultMsg)
		return smsg, nil
	}
	smsg.HostName = string(s[0])
	smsg.AppName, smsg.ProcId = pair2str(parseTag(s[1]))
	smsg.Message = string(bytes.Join(s[2:], SP))
	model.Free(defaultMsg)
	return smsg, nil
}

func parseTag(tag []byte) (appname []byte, procid []byte) {
	tag = bytes.Trim(tag, ":")
	i := bytes.Index(tag, []byte("["))
	if i >= 0 && len(tag) > (i+1) {
		j := bytes.Index(tag, []byte("]"))
		if j > i {
			procid = tag[(i + 1):j]
		} else {
			procid = tag[(i + 1):]
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
		if (!uni.IsLetter(r)) && (!uni.IsNumber(r)) && r != '.' && r != ':' && r != '-' && r != '_' {
			return false
		}
	}
	return true
}
