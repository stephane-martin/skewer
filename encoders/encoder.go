package encoders

import (
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"strconv"

	"github.com/stephane-martin/skewer/encoders/baseenc"
	"github.com/valyala/bytebufferpool"
)

var sp = []byte(" ")
var endl = []byte("\n")

var JsonMimetype = "application/json"
var NDJsonMimetype = "application/x-ndjson"
var ProtobufMimetype = "application/vnd.google.protobuf"
var OctetStreamMimetype = "application/octet-stream"
var PlainMimetype = mime.FormatMediaType("text/plain", map[string]string{"charset": "utf-8"})

var AcceptedMimeTypes = []string{
	JsonMimetype,
	NDJsonMimetype,
	ProtobufMimetype,
	OctetStreamMimetype,
	"text/plain",
}

var RMimeTypes = map[string]Encoder{
	JsonMimetype:        encodeJSON,
	NDJsonMimetype:      encodeJSON,
	ProtobufMimetype:    encodePB,
	OctetStreamMimetype: encodePB,
	PlainMimetype:       encode5424,
	"text/plain":        encode5424,
}

var MimeTypes = map[baseenc.Format]string{
	baseenc.RFC5424:  PlainMimetype,
	baseenc.RFC3164:  PlainMimetype,
	baseenc.JSON:     JsonMimetype,
	baseenc.File:     PlainMimetype,
	baseenc.GELF:     JsonMimetype,
	baseenc.Protobuf: ProtobufMimetype,
}

var encoders = map[baseenc.Format]Encoder{
	baseenc.RFC5424:  encode5424,
	baseenc.RFC3164:  encode3164,
	baseenc.JSON:     encodeJSON,
	baseenc.File:     encodeFile,
	baseenc.GELF:     encodeGELF,
	baseenc.Protobuf: encodePB,
}

type Encoder func(v interface{}, w io.Writer) error

var NonEncodableError = fmt.Errorf("non encodable message")

func IsEncodingError(err error) bool {
	// TODO: check
	if err == nil {
		return false
	}
	if err == NonEncodableError {
		return true
	}
	switch err.(type) {
	case *json.MarshalerError, *ErrInvalid5424:
		return true
	default:
		return false
	}
}

func GetEncoder(frmt baseenc.Format) (Encoder, error) {
	if e, ok := encoders[frmt]; ok {
		return e, nil
	}
	return nil, fmt.Errorf("NewEncoder: unknown encoding format '%d'", frmt)
}

func defaultEncode(v interface{}, w io.Writer) error {
	if v == nil {
		return nil
	}
	switch val := v.(type) {
	case []byte:
		_, err := w.Write(val)
		return err
	case string:
		_, err := w.Write([]byte(val))
		return err
	case int:
		_, err := w.Write([]byte(strconv.FormatInt(int64(val), 10)))
		return err
	case int32:
		_, err := w.Write([]byte(strconv.FormatInt(int64(val), 10)))
		return err
	case int64:
		_, err := w.Write([]byte(strconv.FormatInt(int64(val), 10)))
		return err
	case uint:
		_, err := w.Write([]byte(strconv.FormatUint(uint64(val), 10)))
		return err
	case uint32:
		_, err := w.Write([]byte(strconv.FormatUint(uint64(val), 10)))
		return err
	case uint64:
		_, err := w.Write([]byte(strconv.FormatUint(uint64(val), 10)))
		return err
	default:
		return fmt.Errorf("Dont know how to encode that type: '%T'", val)
	}
}

func ChainEncode(e Encoder, objs ...interface{}) (ret string, err error) {
	if len(objs) == 0 {
		return "", nil
	}
	buf := bytebufferpool.Get()
	var obj interface{}
	for _, obj = range objs {
		err = e(obj, buf)
		if err != nil {
			bytebufferpool.Put(buf)
			return "", err
		}
	}
	ret = buf.String()
	bytebufferpool.Put(buf)
	return ret, nil
}

func TcpOctetEncode(e Encoder, obj interface{}) (ret string, err error) {
	if obj == nil {
		return "", nil
	}
	buf := bytebufferpool.Get()
	err = e(obj, buf)
	if err != nil {
		bytebufferpool.Put(buf)
		return "", err
	}
	data := buf.Bytes()
	if len(data) == 0 {
		bytebufferpool.Put(buf)
		return "", nil
	}
	ret, err = ChainEncode(e, len(data), sp, data)
	bytebufferpool.Put(buf)
	return ret, err
}

func RelpEncode(e Encoder, txnr int32, command string, obj interface{}) (ret string, err error) {
	if obj == nil {
		return ChainEncode(e, txnr, sp, command, sp, int(0), endl)
	}
	buf := bytebufferpool.Get()
	err = e(obj, buf)
	if err != nil {
		bytebufferpool.Put(buf)
		return "", err
	}
	data := buf.Bytes()
	if len(data) == 0 {
		ret, err = ChainEncode(e, txnr, sp, command, sp, int(0), endl)
		bytebufferpool.Put(buf)
		return ret, err
	}
	ret, err = ChainEncode(e, txnr, sp, command, sp, len(data), sp, data, endl)
	bytebufferpool.Put(buf)
	return ret, err
}
