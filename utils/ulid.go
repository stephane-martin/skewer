package utils

import (
	"encoding/json"
	"math/rand"
	"time"

	"github.com/oklog/ulid"
)

type MyULID ulid.ULID

var ZeroUid MyULID

func (uid MyULID) Marshal() ([]byte, error) {
	return ulid.ULID(uid).MarshalBinary()
}

func (uid MyULID) MarshalTo(dst []byte) (n int, err error) {
	if len(dst) < 16 {
		return 0, ulid.ErrBufferSize
	}
	copy(dst, uid[:])
	return 16, nil
}

func (uid *MyULID) Unmarshal(data []byte) error {
	if len(data) < 16 {
		return ulid.ErrDataSize
	}

	copy((*uid)[:], data[:16])
	return nil
}

func (uid *MyULID) Size() int {
	if uid == nil {
		return 0
	}
	return 16
}

func (uid MyULID) String() string {
	return (ulid.ULID)(uid).String()
}

func (uid MyULID) MarshalJSON() ([]byte, error) {
	return json.Marshal(ulid.ULID(uid).String())
}

func (uid *MyULID) UnmarshalJSON(data []byte) error {
	var s string
	err := json.Unmarshal(data, &s)
	if err != nil {
		return err
	}
	tmp, err := ulid.Parse(s)
	if err != nil {
		return err
	}
	*uid = MyULID(tmp)
	return nil
}

func (uid MyULID) Equal(other MyULID) bool {
	return uid == other
}

func (uid MyULID) Compare(other MyULID) int {
	return ulid.ULID(uid).Compare(ulid.ULID(other))
}

func Parse(uids string) (uid MyULID, err error) {
	var id ulid.ULID
	id, err = ulid.Parse(uids)
	if err != nil {
		return uid, err
	}
	return MyULID(id), nil
}

type Generator struct {
	entropy *rand.Rand
}

func NewGenerator() *Generator {
	gen := Generator{
		entropy: rand.New(rand.NewSource(time.Now().UnixNano())),
	}
	return &gen
}

func (g *Generator) Uid() MyULID {
	uid, err := ulid.New(ulid.Timestamp(time.Now()), g.entropy)
	if err != nil {
		panic(err)
	}
	return MyULID(uid)
}

func NewUid() MyULID {
	return NewGenerator().Uid()
}

func NewUidString() string {
	uid := NewUid()
	return uid.String()
}
