package decoders

import (
	"github.com/gogo/protobuf/proto"
	"github.com/stephane-martin/skewer/model"
	"golang.org/x/text/encoding"
)

func pProtobuf(m []byte, decoder *encoding.Decoder) (msg *model.SyslogMessage, err error) {
	// binary format: we ignore decoder
	msg = model.Factory()
	err = proto.Unmarshal(m, msg)
	if err != nil {
		return nil, err
	}
	return msg, nil
}
