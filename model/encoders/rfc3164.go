package encoders

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/stephane-martin/skewer/model"
)

func encode3164(v interface{}, w io.Writer) error {
	if v == nil {
		return nil
	}
	switch val := v.(type) {
	case *model.FullMessage:
		return encodeMsg3164(&val.Parsed.Fields, w)
	case *model.ParsedMessage:
		return encodeMsg3164(&val.Fields, w)
	case *model.SyslogMessage:
		return encodeMsg3164(val, w)
	default:
		return defaultEncode(v, w)
	}
}

func encodeMsg3164(m *model.SyslogMessage, b io.Writer) (err error) {
	procid := strings.TrimSpace(m.ProcId)
	if len(procid) > 0 {
		procid = fmt.Sprintf("[%s]", procid)
	}
	hostname := strings.TrimSpace(m.HostName)
	if len(hostname) == 0 {
		hostname, _ = os.Hostname()
	}
	_, err = fmt.Fprintf(
		b, "<%d>%s %s %s%s: %s",
		m.Priority,
		m.GetTimeReported().Format("Jan _2 15:04:05"),
		hostname,
		m.AppName,
		procid,
		m.Message,
	)
	return err
}
