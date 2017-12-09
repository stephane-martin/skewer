package logging

//go:generate goderive .

import (
	"context"
	"fmt"
	"net"
	"os"
	"sync"
	"time"

	"github.com/awnumar/memguard"
	"github.com/inconshreveable/log15"
	"github.com/stephane-martin/skewer/utils/sbox"
)

func receive(ctx context.Context, secret *memguard.LockedBuffer, l log15.Logger, c *net.UnixConn) {
	keyNames := log15.RecordKeyNames{
		Time: timeKey,
		Msg:  msgKey,
		Lvl:  lvlKey,
	}
	h := l.GetHandler()
	var err error
	var enc [65535]byte
	var dec []byte
	var n int

Listen:
	for {
		select {
		case <-ctx.Done():
			break Listen
		default:
			r := Record{}
			_ = c.SetReadDeadline(time.Now().Add(2 * time.Second))
			n, _, err = c.ReadFrom(enc[:])
			if err != nil {
				if e, ok := err.(net.Error); ok {
					if !e.Timeout() {
						fmt.Fprintln(os.Stderr, "AHEM", err)
					}
				} else {
					fmt.Fprintln(os.Stderr, "AHEM", err)
				}
				continue Listen
			}
			dec, err = sbox.Decrypt(enc[:n], secret)
			if err != nil {
				fmt.Fprintln(os.Stderr, "AHEUM", err)
				continue Listen
			}
			_, err = r.UnmarshalMsg(dec)
			if err != nil {
				fmt.Fprintln(os.Stderr, "FICK", err)
				continue Listen
			}
			logr := log15.Record{Lvl: log15.Lvl(r.Lvl), Msg: r.Msg, Time: r.Time, KeyNames: keyNames}
			logr.Ctx = make([]interface{}, 0, 2*len(r.Ctx))
			for _, k := range deriveSortRecord(deriveKeysRecord(r.Ctx)) {
				logr.Ctx = append(logr.Ctx, k)
				logr.Ctx = append(logr.Ctx, r.Ctx[k])
			}
			_ = h.Log(&logr)
		}
	}
}

func LogReceiver(ctx context.Context, secret *memguard.LockedBuffer, l log15.Logger, connections []*net.UnixConn) *sync.WaitGroup {
	wg := &sync.WaitGroup{}
	for _, conn := range connections {
		wg.Add(1)
		go func(c *net.UnixConn) {
			receive(ctx, secret, l, c)
			wg.Done()
		}(conn)
	}
	return wg
}
