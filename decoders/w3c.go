package decoders

import (
	"bytes"
	"io"
	"strings"

	"github.com/stephane-martin/skewer/decoders/base"
	"github.com/stephane-martin/skewer/model"
	w3c "github.com/stephane-martin/w3c-extendedlog-parser"
)

func W3CDecoder(fieldNames string) base.Parser {
	// https://www.w3.org/TR/WD-logfile.html
	fields := strings.Split(fieldNames, " ")
	return func(m []byte) (msgs []*model.SyslogMessage, err error) {
		parser := w3c.NewFileParser(bytes.NewReader(m)).SetFieldNames(fields)
		msgs = make([]*model.SyslogMessage, 0, 1)
		var msg *model.SyslogMessage
		var line *w3c.Line

		for {
			line, err = parser.Next()
			if err != nil && err != io.EOF {
				return nil, err
			}
			if line == nil {
				break
			}
			msg = model.CleanFactory()
			msg.ClearDomain("w3c")
			msg.Properties.Map["w3c"].Map = line.GetProperties()
			msgs = append(msgs, msg)
		}

		return msgs, nil
	}
}
