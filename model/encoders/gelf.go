package encoders

import (
	"bytes"
	"io"

	"github.com/stephane-martin/skewer/model"
	"github.com/stephane-martin/skewer/utils"
	"gopkg.in/Graylog2/go-gelf.v2/gelf"
)

func FullToGelfMessage(m *model.FullMessage) *gelf.Message {
	gm := SyslogToGelfMessage(m.Fields)
	if m.Uid != utils.ZeroUid {
		gm.Extra["skewer_uid"] = m.Uid.String()
	}
	if m.Txnr > 0 {
		gm.Extra["txnr"] = m.Txnr
	}
	return gm
}

func SyslogToGelfMessage(m *model.SyslogMessage) *gelf.Message {
	gelfm := gelf.Message{
		Version:  "1.1",
		Host:     m.HostName,
		Short:    m.Message,
		Full:     "",
		TimeUnix: float64(m.TimeReportedNum) / 1000000000,
		Level:    int32(m.Severity),
		Facility: m.Facility.String(),
		RawExtra: nil,
	}
	gelfm.Extra = map[string]interface{}{}
	for domain, props := range m.Properties.GetMap() {
		gelfm.Extra[domain] = map[string]string{}
		for k, v := range props.GetMap() {
			(gelfm.Extra[domain]).(map[string]string)[k] = v
		}
	}
	gelfm.Extra["facility"] = gelfm.Facility
	if len(m.AppName) > 0 {
		gelfm.Extra["appname"] = m.AppName
	}
	if len(m.ProcId) > 0 {
		gelfm.Extra["procid"] = m.ProcId
	}
	if len(m.MsgId) > 0 {
		gelfm.Extra["msgid"] = m.MsgId
	}

	return &gelfm
}

func encodeGELF(v interface{}, w io.Writer) (err error) {
	if v == nil {
		return nil
	}
	switch val := v.(type) {
	case *model.FullMessage:
		buf := bytes.NewBuffer(nil)
		err = FullToGelfMessage(val).MarshalJSONBuf(buf)
		if err != nil {
			return err
		}
		_, err = w.Write(buf.Bytes())
		return err
	case *model.SyslogMessage:
		buf := bytes.NewBuffer(nil)
		err = SyslogToGelfMessage(val).MarshalJSONBuf(buf)
		if err != nil {
			return err
		}
		_, err = w.Write(buf.Bytes())
		return err
	default:
		return defaultEncode(v, w)
	}
}
