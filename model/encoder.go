package model

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

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
	case "gelf":
		return newEncoderGELF(), nil
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
	case *SyslogMessage:
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
	case *FullMessage:
		return e.Enc(&val.Parsed.Fields, w)
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
		return encodeMsg5424(&val.Parsed.Fields, w)
	case *SyslogMessage:
		return encodeMsg5424(val, w)
	default:
		return defaultEncode(v, w)
	}
}

type encoderGELF struct {
	w io.Writer
}

func newEncoderGELF() *encoderGELF {
	return &encoderGELF{}
}

func (e *encoderGELF) Enc(v interface{}, w io.Writer) (err error) {
	if v == nil {
		return nil
	}
	switch val := v.(type) {
	case *FullMessage:
		buf := bytes.NewBuffer(nil)
		err = val.ToGelfMessage().MarshalJSONBuf(buf)
		if err != nil {
			return err
		}
		_, err = w.Write(buf.Bytes())
		return err
	case *SyslogMessage:
		buf := bytes.NewBuffer(nil)
		err = val.ToGelfMessage().MarshalJSONBuf(buf)
		if err != nil {
			return err
		}
		_, err = w.Write(buf.Bytes())
		return err
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
		return encodeMsg3164(&val.Parsed.Fields, w)
	case *SyslogMessage:
		return encodeMsg3164(val, w)
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
		return json.NewEncoder(w).Encode(val.Parsed.Fields.Regular())
	case *ParsedMessage:
		return json.NewEncoder(w).Encode(val.Fields.Regular())
	case *SyslogMessage:
		//val.SetTimeStrings()
		return json.NewEncoder(w).Encode(val.Regular())
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
		//val.Parsed.Fields.SetTimeStrings()
		return json.NewEncoder(w).Encode(val.Parsed.Regular())
	case *ParsedMessage:
		return json.NewEncoder(w).Encode(val.Regular())
	case *SyslogMessage:
		return json.NewEncoder(w).Encode(val.Regular())
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
	if !utils.PrintableUsASCII(m.HostName) {
		return invalid5424("Hostname", m.HostName)
	}
	if len(m.HostName) > 255 {
		return invalid5424("Hostname", m.HostName)
	}
	if !utils.PrintableUsASCII(m.AppName) {
		return invalid5424("Appname", m.AppName)
	}
	if len(m.AppName) > 48 {
		return invalid5424("Appname", m.AppName)
	}
	if !utils.PrintableUsASCII(m.ProcId) {
		return invalid5424("Procid", m.ProcId)
	}
	if len(m.ProcId) > 128 {
		return invalid5424("Procid", m.ProcId)
	}
	if !utils.PrintableUsASCII(m.MsgId) {
		return invalid5424("Msgid", m.MsgId)
	}
	if len(m.MsgId) > 32 {
		return invalid5424("Msgid", m.MsgId)
	}

	for sid := range m.Properties.GetMap() {
		if !validName(sid) {
			return invalid5424("StructuredData/ID", sid)
		}
		for param, value := range m.Properties.Map[sid].GetMap() {
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

func encodeMsg5424(m *SyslogMessage, b io.Writer) (err error) {
	err = m.validRfc5424()

	if err != nil {
		return err
	}

	_, err = fmt.Fprintf(
		b,
		"<%d>1 %s %s %s %s %s ",
		m.Priority,
		m.GetTimeReported().Format(time.RFC3339),
		nilify(m.HostName),
		nilify(m.AppName),
		nilify(m.ProcId),
		nilify(m.MsgId),
	)

	if err != nil {
		return err
	}

	if len(m.Properties.GetMap()) == 0 {
		_, err = fmt.Fprint(b, "-")
		if err != nil {
			return err
		}
	}

	for sid := range m.Properties.GetMap() {
		_, err = fmt.Fprintf(b, "[%s", sid)
		if err != nil {
			return err
		}
		for name, value := range m.Properties.Map[sid].GetMap() {
			if len(name) > 32 {
				name = name[:32]
			}
			_, err = fmt.Fprintf(b, " %s=\"%s\"", name, escapeSDParam(value))
			if err != nil {
				return err
			}
		}
		_, err = fmt.Fprintf(b, "]")
		if err != nil {
			return err
		}
	}

	if len(m.Message) > 0 {
		_, err = fmt.Fprint(b, " ")
		if err != nil {
			return err
		}
		_, err = b.Write([]byte(m.Message))
		if err != nil {
			return err
		}
	}
	return nil
}

func encodeMsg3164(m *SyslogMessage, b io.Writer) (err error) {
	procid := strings.TrimSpace(m.ProcId)
	if len(procid) > 0 {
		procid = fmt.Sprintf("[%s]", procid)
	}
	hostname := strings.TrimSpace(m.HostName)
	if len(hostname) == 0 {
		hostname, _ = os.Hostname()
	}
	_, err = fmt.Fprintf(
		b, "<%d>%s %s %s%s: %s",
		m.Priority,
		m.GetTimeReported().Format("Jan _2 15:04:05"),
		hostname,
		m.AppName,
		procid,
		m.Message,
	)
	return err
}
