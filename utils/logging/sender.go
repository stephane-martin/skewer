package logging

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"time"

	"github.com/awnumar/memguard"
	proto "github.com/gogo/protobuf/proto"
	"github.com/inconshreveable/log15"
	"github.com/stephane-martin/skewer/utils/sbox"
)

type RemoteLoggerHandler struct {
	remote  *net.UnixConn
	msgChan chan *log15.Record
	ctx     context.Context
}

func NewRemoteLogger(ctx context.Context, remote *net.UnixConn, secret *memguard.LockedBuffer) log15.Logger {
	// the h.msgChan ensures that we write log messages sequentially to the remote socket
	logger := log15.New()
	h := RemoteLoggerHandler{remote: remote, ctx: ctx}
	h.msgChan = make(chan *log15.Record, 1000)
	logger.SetHandler(&h)

	go func() {
		defer func() { _ = h.Close() }()

		var rbis Record
		var r *log15.Record
		done := ctx.Done()
		buf := proto.NewBuffer(make([]byte, 0, 1024))
		encbuf := make([]byte, 0, 1024)

	Send:
		for {
			select {
			case <-done:
				return
			case r = <-h.msgChan:
				if r == nil {
					return
				}
				rbis = Record{Time: r.Time.UnixNano(), Lvl: int32(r.Lvl), Msg: r.Msg, Ctx: map[string]string{}}
				l := len(r.Ctx)
				var i int
				var ok bool
				label := ""
				val := ""

				for i < l {
					label, ok = r.Ctx[i].(string)
					if ok {
						i++
						if i < l {
							val = formatValue(r.Ctx[i])
							rbis.Ctx[label] = val
							i++
						}
					}

				}
				buf.Reset()
				encbuf = encbuf[:0]
				err := buf.Marshal(&rbis)
				if err != nil {
					continue Send
				}
				// ensure that we don't use secret after the logger has been canceled
				select {
				case <-done:
					return
				default:
					encbuf, err = sbox.EncryptTo(buf.Bytes(), secret, encbuf)
					if err != nil {
						continue Send
					}
					_, _ = remote.Write(encbuf)
				}
			}
		}
	}()

	return logger
}

func (h *RemoteLoggerHandler) Log(r *log15.Record) error {
	select {
	case <-h.ctx.Done():
		return nil
	default:
	}

	select {
	case h.msgChan <- r:
	case <-h.ctx.Done():
	}
	return nil
}

func (h *RemoteLoggerHandler) Close() error {
	return h.remote.Close()
}

const timeFormat = "2006-01-02T15:04:05-0700"

func formatShared(value interface{}) (result interface{}) {
	switch v := value.(type) {
	case time.Time:
		return v.Format(timeFormat)

	case error:
		return v.Error()

	case fmt.Stringer:
		return v.String()

	default:
		return v
	}
}

func formatValue(value interface{}) string {
	value = formatShared(value)
	switch v := value.(type) {
	case string:
		return v
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
		return fmt.Sprintf("%d", value)
	case float32:
		return strconv.FormatFloat(float64(v), 'f', 3, 64)
	case float64:
		return strconv.FormatFloat(v, 'f', 3, 64)
	default:
		return fmt.Sprintf("%+v", value)
	}
}
