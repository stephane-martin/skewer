package logging

//go:generate goderive .

import (
	"context"
	"net"
	"time"

	"github.com/inconshreveable/log15"
	"github.com/tinylib/msgp/msgp"
)

func receive(ctx context.Context, l log15.Logger, c net.Conn) {
	keyNames := log15.RecordKeyNames{
		Time: timeKey,
		Msg:  msgKey,
		Lvl:  lvlKey,
	}
	h := l.GetHandler()
	rem := msgp.NewReader(c)

Listen:
	for {
		select {
		case <-ctx.Done():
			break Listen
		default:
			r := Record{}
			c.SetReadDeadline(time.Now().Add(2 * time.Second))
			e := r.DecodeMsg(rem)
			if e == nil {
				logr := log15.Record{Lvl: log15.Lvl(r.Lvl), Msg: r.Msg, Time: r.Time, KeyNames: keyNames}
				logr.Ctx = make([]interface{}, 0, 2*len(r.Ctx))
				for _, k := range deriveSortRecord(deriveKeysRecord(r.Ctx)) {
					logr.Ctx = append(logr.Ctx, k)
					logr.Ctx = append(logr.Ctx, r.Ctx[k])
				}
				h.Log(&logr)
			}

		}
	}
}

func LogReceiver(ctx context.Context, l log15.Logger, connections []net.Conn) {
	for _, c := range connections {
		go receive(ctx, l, c)
	}
}
