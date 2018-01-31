package encoders

import (
	"io"

	"github.com/pquerna/ffjson/ffjson"
	"github.com/stephane-martin/skewer/model"
)

func encodeJSON(v interface{}, w io.Writer) error {
	if v == nil {
		return nil
	}
	switch val := v.(type) {
	case *model.FullMessage:
		return ffjson.NewEncoder(w).Encode(val.Fields.Regular())
	case *model.SyslogMessage:
		return ffjson.NewEncoder(w).Encode(val.Regular())
	default:
		return defaultEncode(v, w)
	}
}
