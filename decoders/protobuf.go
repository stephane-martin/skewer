package decoders

import (
	"github.com/gogo/protobuf/proto"
	"github.com/stephane-martin/skewer/model"
)

func pProtobuf(m []byte) ([]*model.SyslogMessage, error) {
	// binary format: we ignore decoder
	msg := model.Factory()
	err := proto.Unmarshal(m, msg)
	if err != nil {
		return nil, err
	}
	return []*model.SyslogMessage{msg}, nil
}
