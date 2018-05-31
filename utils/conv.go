package utils

import (
	"encoding/binary"
	"fmt"
	"math"
	"strconv"
	"time"
)

func Atoi32(s string) (int32, error) {
	res, err := strconv.Atoi(s)
	if err != nil {
		return 0, nil
	}
	if res > math.MaxInt32 {
		return 0, fmt.Errorf("int32 overflow")
	}
	return int32(res), nil
}

func Time2Bytes(t time.Time, dst []byte) []byte {
	if cap(dst) < binary.MaxVarintLen32 {
		dst = make([]byte, binary.MaxVarintLen32)
	}
	dst = dst[:binary.MaxVarintLen32]
	n := binary.PutVarint(dst, t.UnixNano())
	return dst[:n]
}

func Bytes2Time(b []byte) (time.Time, int) {
	var ts time.Time
	t, n := binary.Varint(b)
	if n <= 0 {
		return ts, n
	}
	return time.Unix(0, t), n
}
