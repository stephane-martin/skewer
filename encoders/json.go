package encoders

import (
	"io"
	"sync"

	"github.com/linkedin/goavro"
	"github.com/pquerna/ffjson/ffjson"
	"github.com/stephane-martin/skewer/model"
	"github.com/stephane-martin/skewer/model/avro"
)

var loadSchemaOnce sync.Once
var codec *goavro.Codec
var fullcodec *goavro.Codec

func loadSchema() {
	loadSchemaOnce.Do(func() {
		var err error
		codec, err = goavro.NewCodec(avro.NewSyslogMessage().Schema())
		if err != nil {
			panic(err)
		}
		fullcodec, err = goavro.NewCodec(avro.NewFullMessage().Schema())
		if err != nil {
			panic(err)
		}
	})
}

func encodeJSON(v interface{}, w io.Writer) error {
	if v == nil {
		return nil
	}
	switch val := v.(type) {
	case *model.FullMessage:
		return ffjson.NewEncoder(w).Encode(val.Fields.Regular())

	case *model.SyslogMessage:
		return ffjson.NewEncoder(w).Encode(val.Regular())

	}
	return defaultEncode(v, w)
}

func encodeAVRO(v interface{}, w io.Writer) error {
	if v == nil {
		return nil
	}
	switch val := v.(type) {
	case *model.FullMessage:
		return val.Fields.Avro().Serialize(w)
	case *model.SyslogMessage:
		return val.Avro().Serialize(w)
	}
	return defaultEncode(v, w)
}

func encodeJSONAVRO(v interface{}, w io.Writer) error {
	if v == nil {
		return nil
	}
	loadSchema()
	switch val := v.(type) {
	case *model.FullMessage:
		buf, err := codec.TextualFromNative(nil, val.Fields.NativeAvro())
		if err != nil {
			return EncodingError(err)
		}
		_, err = w.Write(buf)
		return err
	case *model.SyslogMessage:
		buf, err := codec.TextualFromNative(nil, val.NativeAvro())
		if err != nil {
			return EncodingError(err)
		}
		_, err = w.Write(buf)
		return err
	}
	return defaultEncode(v, w)
}

func encodeFullJSON(v interface{}, w io.Writer) error {
	if v == nil {
		return nil
	}
	switch val := v.(type) {
	case *model.FullMessage:
		return ffjson.NewEncoder(w).Encode(val.Regular())

	case *model.SyslogMessage:
		return ffjson.NewEncoder(w).Encode(val.Regular())
	}
	return defaultEncode(v, w)
}

func encodeFullAVRO(v interface{}, w io.Writer) error {
	if v == nil {
		return nil
	}
	switch val := v.(type) {
	case *model.FullMessage:
		return val.Avro().Serialize(w)
	case *model.SyslogMessage:
		return val.Avro().Serialize(w)
	}
	return defaultEncode(v, w)
}

func encodeFullJSONAVRO(v interface{}, w io.Writer) error {
	if v == nil {
		return nil
	}
	loadSchema()
	switch val := v.(type) {
	case *model.FullMessage:
		buf, err := fullcodec.TextualFromNative(nil, val.NativeAvro())
		if err != nil {
			return EncodingError(err)
		}
		_, err = w.Write(buf)
		return err
	case *model.SyslogMessage:
		buf, err := fullcodec.TextualFromNative(nil, val.NativeAvro())
		if err != nil {
			return EncodingError(err)
		}
		_, err = w.Write(buf)
		return err
	}
	return defaultEncode(v, w)
}
