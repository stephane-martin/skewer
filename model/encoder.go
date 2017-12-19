package model

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"time"
	"unicode/utf8"

	"github.com/pquerna/ffjson/ffjson"
	"github.com/stephane-martin/skewer/utils"
)

var sp = []byte(" ")
var endl = []byte("\n")

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

func NewEncoder(frmt string) (Encoder, error) {
	switch frmt {
	case "rfc5424":
		return newEncoder5424(), nil
	case "rfc3164":
		return newEncoder3164(), nil
	case "json":
		return newEncoderJson(), nil
	case "fulljson":
		return newEncoderFullJson(), nil
	case "file":
		return newEncoderFile(), nil
	default:
		return nil, fmt.Errorf("NewEncoder: unknown encoding format '%s'", frmt)
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
	case *FullMessage:
		if len(val.Parsed.Fields.Hostname) == 0 {
			val.Parsed.Fields.Hostname = "-"
		}
		if len(val.Parsed.Fields.Appname) == 0 {
			val.Parsed.Fields.Appname = "-"
		}
		_, err := fmt.Fprintf(
			w,
			"%s %s %s %s",
			val.Parsed.Fields.GetTimeReported().Format(time.RFC3339),
			val.Parsed.Fields.Hostname,
			val.Parsed.Fields.Appname,
			val.Parsed.Fields.Message,
		)
		return err
	case *SyslogMessage:
		if len(val.Hostname) == 0 {
			val.Hostname = "-"
		}
		if len(val.Appname) == 0 {
			val.Appname = "-"
		}
		_, err := fmt.Fprintf(
			w,
			"%s %s %s %s",
			val.GetTimeReported().Format(time.RFC3339),
			val.Hostname,
			val.Appname,
			val.Message,
		)
		return err
	default:
		return defaultEncode(v, w)
	}
}

type encoder5424 struct {
	w io.Writer
}

func newEncoder5424() *encoder5424 {
	return &encoder5424{}
}

func (e *encoder5424) Enc(v interface{}, w io.Writer) error {
	if v == nil {
		return nil
	}
	switch val := v.(type) {
	case *FullMessage:
		return val.Parsed.Fields.Encode5424(w)
	case *SyslogMessage:
		return val.Encode5424(w)
	default:
		return defaultEncode(v, w)
	}
}

type encoder3164 struct {
	w io.Writer
}

func newEncoder3164() *encoder3164 {
	return &encoder3164{}
}

func (e *encoder3164) Enc(v interface{}, w io.Writer) error {
	if v == nil {
		return nil
	}
	switch val := v.(type) {
	case *FullMessage:
		return val.Parsed.Fields.Encode3164(w)
	case *SyslogMessage:
		return val.Encode3164(w)
	default:
		return defaultEncode(v, w)
	}
}

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
	case *FullMessage:
		return ffjson.NewEncoder(w).Encode(&val.Parsed.Fields)
	case *SyslogMessage:
		return ffjson.NewEncoder(w).Encode(val)
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
	case *FullMessage:
		return ffjson.NewEncoder(w).Encode(&val.Parsed)
	default:
		return defaultEncode(v, w)
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

type ErrInvalid5424 struct {
	Property string
	Value    interface{}
}

func (e *ErrInvalid5424) Error() string {
	return fmt.Sprintf("Message cannot be RFC5424 serialized: %s is invalid ('%v')", e.Property, e.Value)
}

func invalid5424(property string, value interface{}) error {
	return &ErrInvalid5424{Property: property, Value: value}
}

func (m *SyslogMessage) validRfc5424() error {
	if !utils.PrintableUsASCII(m.Hostname) {
		return invalid5424("Hostname", m.Hostname)
	}
	if len(m.Hostname) > 255 {
		return invalid5424("Hostname", m.Hostname)
	}
	if !utils.PrintableUsASCII(m.Appname) {
		return invalid5424("Appname", m.Appname)
	}
	if len(m.Appname) > 48 {
		return invalid5424("Appname", m.Appname)
	}
	if !utils.PrintableUsASCII(m.Procid) {
		return invalid5424("Procid", m.Procid)
	}
	if len(m.Procid) > 128 {
		return invalid5424("Procid", m.Procid)
	}
	if !utils.PrintableUsASCII(m.Msgid) {
		return invalid5424("Msgid", m.Msgid)
	}
	if len(m.Msgid) > 32 {
		return invalid5424("Msgid", m.Msgid)
	}

	for sid := range m.Properties {
		if !validName(sid) {
			return invalid5424("StructuredData/ID", sid)
		}
		for param, value := range m.Properties[sid] {
			if !validName(param) {
				return invalid5424("StructuredData/Name", param)
			}
			if !utf8.ValidString(value) {
				return invalid5424("StructuredData/Value", value)
			}
		}
	}
	return nil
}

func nilify(x string) string {
	if x == "" {
		return "-"
	}
	return x
}

func escapeSDParam(s string) string {
	escapeCount := 0
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '\\', '"', ']':
			escapeCount++
		}
	}
	if escapeCount == 0 {
		return s
	}

	t := make([]byte, len(s)+escapeCount)
	j := 0
	for i := 0; i < len(s); i++ {
		switch c := s[i]; c {
		case '\\', '"', ']':
			t[j] = '\\'
			t[j+1] = c
			j += 2
		default:
			t[j] = s[i]
			j++
		}
	}
	return string(t)
}

func validName(s string) bool {
	for _, ch := range s {
		if ch < 33 || ch > 126 {
			return false
		}
		if ch == '=' || ch == ']' || ch == '"' {
			return false
		}
	}
	return true
}
