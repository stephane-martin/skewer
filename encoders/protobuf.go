package encoders

import (
	"io"

	"github.com/gogo/protobuf/proto"
	"github.com/stephane-martin/skewer/model"
)

func encodePB(v interface{}, w io.Writer) error {
	if v == nil {
		return nil
	}
	switch val := v.(type) {
	case *model.FullMessage:
		buf, err := proto.Marshal(val)
		if err != nil {
			return EncodingError(err)
		}
		_, err = w.Write(buf)
		return err
	case *model.SyslogMessage:
		buf, err := proto.Marshal(val)
		if err != nil {
			return EncodingError(err)
		}
		_, err = w.Write(buf)
		return err
	default:
		return defaultEncode(v, w)
	}
}
