package reservoir

import (
	"sync"

	"github.com/gogo/protobuf/proto"
	"github.com/stephane-martin/skewer/model"
	"github.com/stephane-martin/skewer/utils"
	"github.com/stephane-martin/skewer/utils/eerrors"
	"github.com/stephane-martin/skewer/utils/queue/stringq"
)

type Reservoir struct {
	ring *stringq.Ring
}

func NewReservoir(qsize uint64) *Reservoir {
	return &Reservoir{ring: stringq.NewRing(qsize)}
}

func (r *Reservoir) Add(uid utils.MyULID, msg string) {
	r.ring.Put(utils.UIDString{S: msg, UID: uid})
}

func (r *Reservoir) AddMessage(msg *model.FullMessage) error {
	buf := getBuffer()
	err := buf.Marshal(msg)
	if err != nil {
		return eerrors.Wrap(err, "Failed to protobuf-marshal message to be sent to the Store")
	}
	r.Add(msg.Uid, string(buf.Bytes()))
	putBuffer(buf)
	return nil
}

func (r *Reservoir) Dispose() {
	r.ring.Dispose()
}

func (r *Reservoir) DeliverTo(m map[utils.MyULID]string) error {
	for {
		s, err := r.ring.Poll(-1)
		if err == eerrors.ErrQDisposed {
			return err
		}
		if err != nil {
			return nil
		}
		m[s.UID] = s.S
	}
}

func getBuffer() (buf *proto.Buffer) {
	buf = pool.Get().(*proto.Buffer)
	buf.Reset()
	return buf
}

func putBuffer(buf *proto.Buffer) {
	if buf != nil {
		pool.Put(buf)
	}
}

var pool = &sync.Pool{
	New: func() interface{} {
		return proto.NewBuffer(make([]byte, 0, 16384))
	},
}
