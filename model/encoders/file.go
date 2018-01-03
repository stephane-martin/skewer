package encoders

import (
	"fmt"
	"io"
	"time"

	"github.com/stephane-martin/skewer/model"
)

type encoderFile struct {
	w io.Writer
}

func newEncoderFile() *encoderFile {
	return &encoderFile{}
}

func (e *encoderFile) Enc(v interface{}, w io.Writer) error {
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
		return e.Enc(&val.Parsed.Fields, w)
	default:
		return defaultEncode(v, w)
	}
}
