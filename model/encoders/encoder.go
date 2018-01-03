package encoders

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"
)

var sp = []byte(" ")
var endl = []byte("\n")

type Format int

const (
	RFC5424 Format = iota
	RFC3164
	JSON
	FullJSON
	File
	GELF
)

var Formats = map[string]Format{
	"rfc5424":  RFC5424,
	"rfc3164":  RFC3164,
	"json":     JSON,
	"fulljson": FullJSON,
	"file":     File,
	"gelf":     GELF,
}

func ParseFormat(format string) Format {
	format = strings.ToLower(strings.TrimSpace(format))
	if f, ok := Formats[format]; ok {
		return f
	}
	return -1
}

type Encoder interface {
	Enc(v interface{}, w io.Writer) error
}

var NonEncodableError = fmt.Errorf("non encodable message")

func IsEncodingError(err error) bool {
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

func NewEncoder(frmt Format) (Encoder, error) {
	switch frmt {
	case RFC5424:
		return newEncoder5424(), nil
	case RFC3164:
		return newEncoder3164(), nil
	case JSON:
		return newEncoderJson(), nil
	case FullJSON:
		return newEncoderFullJson(), nil
	case File:
		return newEncoderFile(), nil
	case GELF:
		return newEncoderGELF(), nil
	default:
		return nil, fmt.Errorf("NewEncoder: unknown encoding format '%d'", frmt)
	}
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

func ChainEncode(e Encoder, objs ...interface{}) ([]byte, error) {
	var err error
	buf := bytes.NewBuffer(nil)
	for _, obj := range objs {
		err = e.Enc(obj, buf)
		if err != nil {
			return nil, err
		}
	}
	return buf.Bytes(), nil
}

func TcpOctetEncode(e Encoder, obj interface{}) ([]byte, error) {
	if obj == nil {
		return []byte{}, nil
	}
	var err error
	buf := bytes.NewBuffer(nil)
	err = e.Enc(obj, buf)
	if err != nil {
		return nil, err
	}
	data := buf.Bytes()
	if len(data) == 0 {
		return []byte{}, nil
	}
	return ChainEncode(e, len(data), sp, data)
}

func RelpEncode(e Encoder, txnr int32, command string, obj interface{}) ([]byte, error) {
	if obj == nil {
		return ChainEncode(e, txnr, sp, command, sp, int(0), endl)
	}
	var err error
	buf := bytes.NewBuffer(nil)
	err = e.Enc(obj, buf)
	if err != nil {
		return nil, err
	}
	data := buf.Bytes()
	if len(data) == 0 {
		return ChainEncode(e, txnr, sp, command, sp, int(0), endl)
	}
	return ChainEncode(e, txnr, sp, command, sp, len(data), sp, data, endl)
}
