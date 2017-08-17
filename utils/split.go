package utils

import (
	"bytes"
	"fmt"
	"strconv"
)

func PluginSplit(data []byte, atEOF bool) (int, []byte, error) {
	trimmed_data := bytes.TrimLeft(data, " \r\n")
	if len(trimmed_data) < 11 {
		return 0, nil, nil
	}
	trimmed := len(data) - len(trimmed_data)
	if trimmed_data[10] != byte(' ') {
		return 0, nil, fmt.Errorf("Wrong plugin format, 11th char is not space: '%s'", string(data))
	}
	var i int
	for i = 0; i < 10; i++ {
		if trimmed_data[i] < byte('0') || trimmed_data[i] > byte('9') {
			return 0, nil, fmt.Errorf("Wrong plugin format")
		}
	}
	datalen, err := strconv.Atoi(string(trimmed_data[:10]))
	if err != nil {
		return 0, nil, err
	}
	advance := trimmed + 11 + datalen
	if len(data) < advance {
		return 0, nil, nil
	}
	return advance, bytes.Trim(trimmed_data[11:11+datalen], " \r\n"), nil
}
