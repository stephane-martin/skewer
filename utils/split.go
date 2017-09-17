package utils

import (
	"fmt"
	"io"
	"strconv"
)

var NOW []byte = []byte("now")
var SP []byte = []byte(" ")

func ret(n int, e error) error {
	return e
}

func W(dest io.Writer, header string, message []byte) (err error) {
	return Chain(
		func() error { return ret(fmt.Fprintf(dest, "%010d ", len(header)+len(message)+1)) },
		func() error { return ret(io.WriteString(dest, header)) },
		func() error { return ret(dest.Write(SP)) },
		func() error { return ret(dest.Write(message)) },
	)
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
