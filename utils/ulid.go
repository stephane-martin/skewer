package utils

import (
	"encoding/json"
	"hash/fnv"
	"io"
	"math/rand"
	"strings"
	"time"

	"github.com/oklog/ulid"
	"github.com/zond/gotomic"
)

// MyULID wraps ulid.ULID to allow protobuf marshaling.
type MyULID string

var zzz ulid.ULID

// ZeroULID is a zero ULID.
var ZeroULID = MyULID(string(zzz[:]))

// Marshal marshals the ULID.
func (uid MyULID) Marshal() ([]byte, error) {
	if len(uid) < 16 {
		return []byte(ZeroULID), nil
	}
	return []byte(uid)[:16], nil
}

// MarshalTo marshals the ULID and writes it to dst.
func (uid MyULID) MarshalTo(dst []byte) (n int, err error) {
	if len(dst) < 16 {
		return 0, ulid.ErrBufferSize
	}
	if len(uid) < 16 {
		copy(dst, ZeroULID)
	} else {
		copy(dst, uid[:16])
	}
	return 16, nil
}

// Unmarshal unmarshals the data into a ULID.
func (uid *MyULID) Unmarshal(data []byte) error {
	if len(data) < 16 {
		return ulid.ErrDataSize
	}
	*uid = MyULID(string(data[:16]))
	return nil
}

// Size returns the size of the ULID in bytes.
func (uid *MyULID) Size() int {
	if uid == nil {
		return 0
	}
	return 16
}

func (uid MyULID) String() string {
	if len(uid) < 16 {
		return zzz.String()
	}
	var tmp ulid.ULID
	copy(tmp[:], uid[:16])
	return tmp.String()
}

// MarshalJSON marshals the ULID to JSON.
func (uid MyULID) MarshalJSON() ([]byte, error) {
	return json.Marshal(uid.String())
}

// UnmarshalJSON unmarshals the data into a ULID using JSON.
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
	*uid = MyULID(string(tmp[:]))
	return nil
}

// Equal checks that two ULIDs are the same.
func (uid MyULID) Equal(other MyULID) bool {
	if len(uid) < 16 && len(other) < 16 {
		return true
	}
	if len(uid) < 16 || len(other) < 16 {
		return false
	}
	return uid[:16] == other[:16]
}

// Equals checks equality with a gotomic.Thing.
func (uid MyULID) Equals(other gotomic.Thing) bool {
	if o, ok := other.(MyULID); ok {
		return uid.Equal(o)
	}
	return false
}

func (uid MyULID) HashCode() uint32 {
	if len(uid) < 16 {
		h := fnv.New32a()
		h.Write(zzz[:])
		return h.Sum32()
	}
	h := fnv.New32a()
	io.WriteString(h, string(uid)[:16])
	return h.Sum32()
}

// Compare returns an integer comparing id and other lexicographically.
func (uid MyULID) Compare(other MyULID) int {
	if len(uid) < 16 && len(other) < 16 {
		return 0
	}
	if len(uid) < 16 {
		return strings.Compare(string(ZeroULID), string(other)[:16])
	}
	if len(other) < 16 {
		return strings.Compare(string(uid)[:16], string(ZeroULID))
	}
	return strings.Compare(string(uid)[:16], string(other)[:16])
}

// ParseMyULID parse a string into a ULID.
func ParseMyULID(uidStr string) (uid MyULID, err error) {
	var id ulid.ULID
	id, err = ulid.Parse(uidStr)
	if err != nil {
		return uid, err
	}
	return MyULID(string(id[:])), nil
}

func MustParseULID(uid string) MyULID {
	tmp := ulid.MustParse(uid)
	return MyULID(string(tmp[:]))
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
	return MyULID(string(uid[:]))
}

// NewUid returns a ULID for the current time.
func NewUid() MyULID {
	return NewGenerator().Uid()
}

// NewUidString generates a ULID for the current time and serializes it to a string.
func NewUidString() string {
	uid := NewUid()
	return uid.String()
}
