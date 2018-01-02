package utils

import "strconv"

func Atoi32(s string) (int32, error) {
	res, err := strconv.Atoi(s)
	if err != nil {
		return 0, nil
	}
	// TODO: check size ?
	return int32(res), nil
}
