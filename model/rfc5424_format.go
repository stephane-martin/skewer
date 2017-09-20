package model

import (
	"bytes"
	"fmt"
	"strconv"
	"time"
	"unicode/utf8"

	"golang.org/x/text/encoding"
	"golang.org/x/text/encoding/unicode"
)

func isASCII(s []byte) bool {
	for _, c := range s {
		if c > 127 {
			return false
		}
	}
	return true
}

var SP []byte = []byte(" ")
var DASH []byte = []byte("-")

func ParseRfc5424Format(m []byte, decoder *encoding.Decoder, dont_parse_sd bool) (smsg SyslogMessage, err error) {
	// HEADER = PRI VERSION SP TIMESTAMP SP HOSTNAME SP APP-NAME SP PROCID SP MSGID
	// PRI = "<" PRIVAL ">"
	// SYSLOG-MSG = HEADER SP STRUCTURED-DATA [SP MSG]

	if decoder == nil {
		decoder = unicode.UTF8.NewDecoder()
	}
	m, err = decoder.Bytes(m)
	if err != nil {
		return smsg, &InvalidEncodingError{Err: err}
	}

	m = bytes.TrimSpace(m)
	splits := bytes.SplitN(m, SP, 7)

	if len(splits) < 7 {
		return smsg, &NotEnoughPartsError{len(splits)}
	}

	smsg.Priority, smsg.Facility, smsg.Severity, smsg.Version, err = parsePriority(splits[0])
	if err != nil {
		return smsg, err
	}

	n := time.Now().UnixNano()
	s := string(splits[1])
	if s == "-" {
		smsg.TimeReported = time.Now().UnixNano()
	}
	if smsg.TimeReported == 0 {
		t1, err := time.Parse(time.RFC3339Nano, s)
		if err != nil {
			t2, err := time.Parse(time.RFC3339, s)
			if err != nil {
				smsg.TimeReported = n
			} else {
				smsg.TimeReported = t2.UnixNano()
			}
		} else {
			smsg.TimeReported = t1.UnixNano()
		}
	}
	smsg.TimeGenerated = n

	s = string(splits[2])
	if s != "-" {
		smsg.Hostname = s
	}
	s = string(splits[3])
	if s != "-" {
		smsg.Appname = s
	}
	s = string(splits[4])
	if s != "-" {
		smsg.Procid = s
	}
	s = string(splits[5])
	if s != "-" {
		smsg.Msgid = s
	}
	structured_and_msg := bytes.TrimSpace(splits[6])
	if bytes.HasPrefix(structured_and_msg, DASH) {
		// structured data is empty
		smsg.Message = string(bytes.TrimSpace(structured_and_msg[1:]))
	} else if bytes.HasPrefix(structured_and_msg, []byte("[")) {
		s1, s2, err := splitStructuredData(structured_and_msg)
		if err != nil {
			return smsg, err
		}
		smsg.Message = string(s2)
		if dont_parse_sd {
			smsg.Structured = string(s1)
		} else {
			smsg.Structured = ""
			props, err := parseStructData(s1)
			if err != nil {
				return smsg, err
			}
			if len(props) > 0 {
				smsg.Properties = props
			}
		}
	} else {
		return smsg, &InvalidStructuredDataError{"Structured data is not nil but does not start with '['"}
	}

	return smsg, nil
}

func splitStructuredData(structured_and_msg []byte) ([]byte, []byte, error) {
	length := len(structured_and_msg)
	for i := 0; i < length; i++ {
		if structured_and_msg[i] == ']' {
			if i == (length - 1) {
				return structured_and_msg, []byte{}, nil
			}
			if structured_and_msg[i+1] == ' ' {
				return structured_and_msg[:i+1], bytes.TrimSpace(structured_and_msg[i+1:]), nil
			}
		}
	}
	return []byte{}, []byte{}, &InvalidStructuredDataError{"Can not find the last ']' that marks the end of structured data"}
}

func parsePriority(pv []byte) (Priority, Facility, Severity, Version, error) {
	if pv[0] != byte('<') {
		return 0, 0, 0, 0, &InvalidPriorityError{}
	}
	i := bytes.Index(pv, []byte(">"))
	if i < 2 {
		return 0, 0, 0, 0, &InvalidPriorityError{}
	}
	if len(pv) <= (i + 1) {
		return 0, 0, 0, 0, &InvalidPriorityError{}
	}

	p, err := strconv.Atoi(string(pv[1:i]))
	if err != nil {
		return 0, 0, 0, 0, &InvalidPriorityError{}
	}

	f := Facility(p / 8)
	s := Severity(p % 8)
	v, err := strconv.Atoi(string(pv[i+1:]))
	if err != nil {
		return 0, 0, 0, 0, &InvalidPriorityError{}
	}

	return Priority(p), f, s, Version(v), nil
}

func parseStructData(sd []byte) (m map[string]map[string]string, err error) {
	// see https://tools.ietf.org/html/rfc5424#section-6.3
	if !utf8.Valid(sd) {
		return nil, &InvalidStructuredDataError{}
	}
	m = map[string]map[string]string{}
	l := len(sd)
	position := 0
	current_sdid := []byte{}
	current_name := []byte{}

	var openBracket func() error
	var sdid func() error
	var value func() error
	var param func() error

	value = func() error {
		// a bit long and painful to take care of escaped characters
		if position == l {
			return &InvalidStructuredDataError{"Expected SD-VALUE, got nothing"}
		}
		if sd[position] != '"' {
			return &InvalidStructuredDataError{"SD-VALUE should start with a quote"}
		}
		position++
		p := position
		found := false
		for p < l && !found {
			if sd[p] == '\\' {
				p++
				if p >= l {
					return &InvalidStructuredDataError{"Unexpected end after a \\"}
				}
				if sd[p] == '"' || sd[p] == '\\' || sd[p] == ']' {
					p++
				}
			} else if sd[p] == '"' {
				found = true
			} else {
				p++
			}
		}
		if found {
			val := sd[position:p]
			m[string(current_sdid)][string(current_name)] = string(val)
			position += len(val)
			position++ // count for the closing quote
			if position >= l {
				return &InvalidStructuredDataError{"Abrupt end of SD-ELEMENT"}
			}
			if sd[position] == ' ' {
				position++
				return param()
			} else if sd[position] == ']' {
				position++
				return openBracket()
			} else {
				return &InvalidStructuredDataError{fmt.Sprintf("Expected SP or ']' but got '%s' instead", string(sd[position]))}
			}

		} else {
			return &InvalidStructuredDataError{"The end of SD-VALUE was not found"}
		}
	}

	param = func() error {
		name_end := bytes.Index(sd[position:], []byte("="))
		if name_end < 1 {
			return &InvalidStructuredDataError{"Invalid SD-NAME"}
		}
		current_name = sd[position : position+name_end]
		if !isASCII(current_name) {
			return &InvalidStructuredDataError{"Invalid SD-NAME"}
		}
		position += name_end
		position++ // count the '='
		return value()
	}

	sdid = func() error {
		end := bytes.IndexAny(sd[position:], " ]")
		if end < 1 {
			return &InvalidStructuredDataError{"Invalid SDID"}
		}
		current_sdid = sd[position : position+end]
		if !isASCII(current_sdid) {
			return &InvalidStructuredDataError{"Invalid SDID"}
		}
		position += end
		m[string(current_sdid)] = map[string]string{}
		if sd[position] == byte(' ') {
			// now read the params
			position++
			return param()
		} else {
			// end of the element
			position++
			return openBracket()
		}

	}

	openBracket = func() error {
		if position == l {
			return nil
		}
		if sd[position] == '[' {
			position++
			return sdid()
		} else {
			return &InvalidStructuredDataError{fmt.Sprintf("Expected '[' but got '%s' instead", string(sd[position]))}
		}
	}

	err = openBracket()
	if err != nil {
		return nil, err
	}

	// remove empty SDID sections
	emptySDIDs := []string{}
	for id, paramvalues := range m {
		if len(paramvalues) == 0 {
			emptySDIDs = append(emptySDIDs, id)
		}
	}
	for _, id := range emptySDIDs {
		delete(m, id)
	}
	if len(m) == 0 {
		return nil, nil
	}

	return m, nil
}
