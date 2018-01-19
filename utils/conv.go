package utils

import (
	"fmt"
	"math"
	"strconv"
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
