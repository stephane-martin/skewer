package encoders

import (
	"io"

	"github.com/pquerna/ffjson/ffjson"
	"github.com/stephane-martin/skewer/model"
)

type encoderJson struct {
	w io.Writer
}

func newEncoderJson() *encoderJson {
	return &encoderJson{}
}

func (e *encoderJson) Enc(v interface{}, w io.Writer) error {
	if v == nil {
		return nil
	}
	switch val := v.(type) {
	case *model.FullMessage:
		return ffjson.NewEncoder(w).Encode(val.Parsed.Fields.Regular())
	case *model.ParsedMessage:
		return ffjson.NewEncoder(w).Encode(val.Fields.Regular())
	case *model.SyslogMessage:
		//val.SetTimeStrings()
		return ffjson.NewEncoder(w).Encode(val.Regular())
	default:
		return defaultEncode(v, w)
	}
}

type encoderFullJson struct {
	w io.Writer
}

func newEncoderFullJson() *encoderFullJson {
	return &encoderFullJson{}
}

func (e *encoderFullJson) Enc(v interface{}, w io.Writer) error {
	if v == nil {
		return nil
	}
	switch val := v.(type) {
	case *model.FullMessage:
		//val.Parsed.Fields.SetTimeStrings()
		return ffjson.NewEncoder(w).Encode(val.Parsed.Regular())
	case *model.ParsedMessage:
		return ffjson.NewEncoder(w).Encode(val.Regular())
	case *model.SyslogMessage:
		return ffjson.NewEncoder(w).Encode(val.Regular())
	default:
		return defaultEncode(v, w)
	}
}
