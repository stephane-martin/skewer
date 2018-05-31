package encoders

import (
	"fmt"
	"io"
	"time"

	"github.com/stephane-martin/skewer/model"
)

func encodeFile(v interface{}, w io.Writer) error {
	if v == nil {
		return nil
	}
	switch val := v.(type) {
	case *model.SyslogMessage:
		if len(val.HostName) == 0 {
			val.HostName = "-"
		}
		if len(val.AppName) == 0 {
			val.AppName = "-"
		}
		_, err := fmt.Fprintf(
			w,
			"%s %s %s %s",
			val.GetTimeReported().Format(time.RFC3339),
			val.HostName,
			val.AppName,
			val.Message,
		)
		return err
	case *model.FullMessage:
		return encodeFile(val.Fields, w)
	default:
		return defaultEncode(v, w)
	}
}
