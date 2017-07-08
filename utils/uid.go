package utils

import (
	"context"
	"math/rand"
	"time"

	"github.com/inconshreveable/log15"
	"github.com/oklog/ulid"
)

func Generator(ctx context.Context, logger log15.Logger) chan ulid.ULID {
	out := make(chan ulid.ULID)
	go func() {
		entropy := rand.New(rand.NewSource(time.Now().UnixNano()))
		for {
			var uid ulid.ULID
			var err error
			for {
				uid, err = ulid.New(ulid.Timestamp(time.Now()), entropy)
				if err != nil {
					logger.Error("Error generating ULID", "error", err)
				} else {
					break
				}
			}
			select {
			case <-ctx.Done():
				return
			case out <- uid:
			}
		}
	}()
	return out
}
