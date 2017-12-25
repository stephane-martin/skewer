package utils

import (
	"math/rand"
	"time"

	"github.com/oklog/ulid"
)

var ZeroUid ulid.ULID

type Generator struct {
	entropy *rand.Rand
}

func NewGenerator() *Generator {
	gen := Generator{
		entropy: rand.New(rand.NewSource(time.Now().UnixNano())),
	}
	return &gen
}

func (g *Generator) Uid() (uid ulid.ULID) {
	var err error
	uid, err = ulid.New(ulid.Timestamp(time.Now()), g.entropy)
	if err != nil {
		panic(err)
	}
	return
}

func NewUid() (uid ulid.ULID) {
	return NewGenerator().Uid()
}
