package encoders

import (
	"fmt"
	"io"
	"time"
	"unicode/utf8"

	"github.com/stephane-martin/skewer/model"
	"github.com/stephane-martin/skewer/utils"
	"github.com/stephane-martin/skewer/utils/eerrors"
)

func encode5424(v interface{}, w io.Writer) error {
	if v == nil {
		return nil
	}
	switch val := v.(type) {
	case *model.FullMessage:
		return encodeMsg5424(val.Fields, w)
	case *model.SyslogMessage:
		return encodeMsg5424(val, w)
	default:
		return defaultEncode(v, w)
	}
}

func invalid5424(property string, value interface{}) error {
	return EncodingError(eerrors.Errorf(
		"Message cannot be RFC5424 serialized: %s is invalid ('%v')",
		property, value,
	))
}

func validRfc5424(m *model.SyslogMessage) error {
	if !utils.PrintableUsASCII(m.HostName) {
		return invalid5424("Hostname", m.HostName)
	}
	if len(m.HostName) > 255 {
		return invalid5424("Hostname", m.HostName)
	}
	if !utils.PrintableUsASCII(m.AppName) {
		return invalid5424("Appname", m.AppName)
	}
	if len(m.AppName) > 48 {
		return invalid5424("Appname", m.AppName)
	}
	if !utils.PrintableUsASCII(m.ProcId) {
		return invalid5424("Procid", m.ProcId)
	}
	if len(m.ProcId) > 128 {
		return invalid5424("Procid", m.ProcId)
	}
	if !utils.PrintableUsASCII(m.MsgId) {
		return invalid5424("Msgid", m.MsgId)
	}
	if len(m.MsgId) > 32 {
		return invalid5424("Msgid", m.MsgId)
	}

	for sid := range m.Properties.GetMap() {
		if !validName(sid) {
			return invalid5424("StructuredData/ID", sid)
		}
		for param, value := range m.Properties.Map[sid].GetMap() {
			if !validName(param) {
				return invalid5424("StructuredData/Name", param)
			}
			if !utf8.ValidString(value) {
				return invalid5424("StructuredData/Value", value)
			}
		}
	}
	return nil
}

func nilify(x string) string {
	if x == "" {
		return "-"
	}
	return x
}

func escapeSDParam(s string) string {
	escapeCount := 0
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '\\', '"', ']':
			escapeCount++
		}
	}
	if escapeCount == 0 {
		return s
	}

	t := make([]byte, len(s)+escapeCount)
	j := 0
	for i := 0; i < len(s); i++ {
		switch c := s[i]; c {
		case '\\', '"', ']':
			t[j] = '\\'
			t[j+1] = c
			j += 2
		default:
			t[j] = s[i]
			j++
		}
	}
	return string(t)
}

func validName(s string) bool {
	for _, ch := range s {
		if ch < 33 || ch > 126 {
			return false
		}
		if ch == '=' || ch == ']' || ch == '"' {
			return false
		}
	}
	return true
}

func encodeMsg5424(m *model.SyslogMessage, b io.Writer) (err error) {
	err = validRfc5424(m)

	if err != nil {
		return err
	}

	_, err = fmt.Fprintf(
		b,
		"<%d>1 %s %s %s %s %s ",
		m.Priority,
		m.GetTimeReported().Format(time.RFC3339),
		nilify(m.HostName),
		nilify(m.AppName),
		nilify(m.ProcId),
		nilify(m.MsgId),
	)

	if err != nil {
		return err
	}

	if len(m.Properties.GetMap()) == 0 {
		_, err = fmt.Fprint(b, "-")
		if err != nil {
			return err
		}
	}

	for sid := range m.Properties.GetMap() {
		_, err = fmt.Fprintf(b, "[%s", sid)
		if err != nil {
			return err
		}
		for name, value := range m.Properties.Map[sid].GetMap() {
			if len(name) > 32 {
				name = name[:32]
			}
			_, err = fmt.Fprintf(b, " %s=\"%s\"", name, escapeSDParam(value))
			if err != nil {
				return err
			}
		}
		_, err = fmt.Fprintf(b, "]")
		if err != nil {
			return err
		}
	}

	if len(m.Message) > 0 {
		_, err = fmt.Fprint(b, " ")
		if err != nil {
			return err
		}
		_, err = io.WriteString(b, m.Message)
		if err != nil {
			return err
		}
	}
	return nil
}
