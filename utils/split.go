package utils

import (
	"fmt"
	"io"
	"strconv"
)

var NOW []byte = []byte("now")

func W(dest io.Writer, header string, message []byte) {
	l := len(header) + len(message) + 1
	fmt.Fprintf(dest, "%010d ", l)
	dest.Write([]byte(header))
	dest.Write([]byte(" "))
	dest.Write(message)
}

func PluginSplit(data []byte, atEOF bool) (int, []byte, error) {
	if len(data) < 11 {
		return 0, nil, nil
	}
	if data[10] != byte(' ') {
		return 0, nil, fmt.Errorf("Wrong plugin format, 11th char is not space: '%s'", string(data))
	}
	var i int
	for i = 0; i < 10; i++ {
		if data[i] < byte('0') || data[i] > byte('9') {
			return 0, nil, fmt.Errorf("Wrong plugin format")
		}
	}
	datalen, err := strconv.Atoi(string(data[:10]))
	if err != nil {
		return 0, nil, err
	}
	advance := 11 + datalen
	if len(data) < advance {
		return 0, nil, nil
	}
	return advance, data[11 : 11+datalen], nil
}
