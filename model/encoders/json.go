package encoders

import (
	"io"

	"github.com/pquerna/ffjson/ffjson"
	"github.com/stephane-martin/skewer/model"
)

func encodeJson(v interface{}, w io.Writer) error {
	if v == nil {
		return nil
	}
	switch val := v.(type) {
	case *model.FullMessage:
		return ffjson.NewEncoder(w).Encode(val.Fields.Regular())
	case *model.SyslogMessage:
		//val.SetTimeStrings()
		return ffjson.NewEncoder(w).Encode(val.Regular())
	default:
		return defaultEncode(v, w)
	}
}

func encodeFullJson(v interface{}, w io.Writer) error {
	// TODO: really full json...
	if v == nil {
		return nil
	}
	switch val := v.(type) {
	case *model.FullMessage:
		//val.Parsed.Fields.SetTimeStrings()
		return ffjson.NewEncoder(w).Encode(val.Fields.Regular())
	case *model.SyslogMessage:
		return ffjson.NewEncoder(w).Encode(val.Regular())
	default:
		return defaultEncode(v, w)
	}
}
