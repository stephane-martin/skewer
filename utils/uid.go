package utils

import (
	"context"
	"math/rand"
	"time"

	"github.com/inconshreveable/log15"
	"github.com/oklog/ulid"
)

func Generator(ctx context.Context, logger log15.Logger) (out chan ulid.ULID) {
	out = make(chan ulid.ULID)

	go func() {
		done := ctx.Done()
		entropy := rand.New(rand.NewSource(time.Now().UnixNano()))
		var uid ulid.ULID
		var err error

		for {
			for {
				uid, err = ulid.New(ulid.Timestamp(time.Now()), entropy)
				if err != nil {
					logger.Error("Error generating ULID", "error", err)
				} else {
					break
				}
			}
			select {
			case <-done:
				return
			case out <- uid:
			}
		}
	}()

	return out
}

func NewUid() (uid ulid.ULID) {
	uid, _ = ulid.New(ulid.Timestamp(time.Now()), rand.New(rand.NewSource(time.Now().UnixNano())))
	return uid
}
