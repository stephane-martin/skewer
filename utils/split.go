package utils

import (
	"bytes"
	"fmt"
	"io"
	"strconv"
)

var NOW []byte = []byte("now")
var SP []byte = []byte(" ")

func W(dest io.Writer, header []byte, message []byte) (err error) {
	return ChainWrites(
		dest,
		[]byte(fmt.Sprintf("%010d ", len(header)+len(message)+1)),
		[]byte(header),
		SP,
		message,
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

// RelpSplit is used to extract RELP lines from the incoming TCP stream
func RelpSplit(data []byte, atEOF bool) (int, []byte, error) {
	trimmedData := bytes.TrimLeft(data, " \r\n")
	if len(trimmedData) == 0 {
		return 0, nil, nil
	}
	splits := bytes.FieldsFunc(trimmedData, splitSpaceOrLF)
	l := len(splits)
	if l < 3 {
		// Request more data
		return 0, nil, nil
	}

	txnrStr := string(splits[0])
	command := string(splits[1])
	datalenStr := string(splits[2])
	tokenStr := txnrStr + " " + command + " " + datalenStr
	advance := len(data) - len(trimmedData) + len(tokenStr) + 1

	if l == 3 && (len(data) < advance) {
		// datalen field is not complete, request more data
		return 0, nil, nil
	}

	_, err := strconv.Atoi(txnrStr)
	if err != nil {
		return 0, nil, err
	}
	datalen, err := strconv.Atoi(datalenStr)
	if err != nil {
		return 0, nil, err
	}
	if datalen == 0 {
		return advance, []byte(tokenStr), nil
	}
	advance += datalen + 1
	if len(data) >= advance {
		token := bytes.Trim(data[:advance], " \r\n")
		return advance, token, nil
	}
	// Request more data
	return 0, nil, nil
}

func splitSpaceOrLF(r rune) bool {
	return r == ' ' || r == '\n' || r == '\r'
}
