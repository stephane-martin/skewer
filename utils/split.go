package utils

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"strconv"

	"github.com/awnumar/memguard"
	"github.com/stephane-martin/skewer/utils/sbox"
)

var NOW []byte = []byte("now")
var SP []byte = []byte(" ")

func W(dest io.Writer, header []byte, message []byte, secret *memguard.LockedBuffer) (err error) {
	var enc []byte
	if secret == nil {
		enc = message
	} else {
		enc, err = sbox.Encrypt(message, secret)
		if err != nil {
			return err
		}
	}
	if len(header) == 0 {
		return ChainWrites(
			dest,
			[]byte(fmt.Sprintf("%010d ", len(enc))),
			enc,
		)
	}
	return ChainWrites(
		dest,
		[]byte(fmt.Sprintf("%010d ", len(header)+len(enc)+1)),
		header,
		SP,
		enc,
	)
}

func MakeDecryptSplit(secret *memguard.LockedBuffer) bufio.SplitFunc {
	spl := func(data []byte, atEOF bool) (int, []byte, error) {
		adv, tok, err := PluginSplit(data, atEOF)
		if err != nil || tok == nil || secret == nil {
			return adv, tok, err
		}
		dec, err := sbox.Decrypt(tok, secret)
		if err != nil {
			return 0, nil, err
		}
		return adv, dec, nil
	}
	return spl
}

func PluginSplit(data []byte, atEOF bool) (advance int, token []byte, eoferr error) {
	if atEOF {
		eoferr = io.EOF
	}
	if len(data) < 11 {
		return 0, nil, eoferr
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
	advance = 11 + datalen
	if len(data) < advance {
		return 0, nil, eoferr
	}
	return advance, data[11 : 11+datalen], nil
}

// RelpSplit is used to extract RELP lines from the incoming TCP stream
func RelpSplit(data []byte, atEOF bool) (advance int, token []byte, eoferr error) {
	if atEOF {
		eoferr = io.EOF
	}
	trimmedData := bytes.TrimLeft(data, " \r\n")
	if len(trimmedData) == 0 {

		return 0, nil, eoferr
	}
	splits := bytes.FieldsFunc(trimmedData, splitSpaceOrLF)
	l := len(splits)
	if l < 3 {
		// Request more data
		return 0, nil, eoferr
	}

	txnrStr := string(splits[0])
	command := string(splits[1])
	datalenStr := string(splits[2])
	tokenStr := txnrStr + " " + command + " " + datalenStr
	advance = len(data) - len(trimmedData) + len(tokenStr) + 1

	if l == 3 && (len(data) < advance) {
		// datalen field is not complete, request more data
		return 0, nil, eoferr
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
		token = bytes.Trim(data[:advance], " \r\n")
		return advance, token, nil
	}
	// Request more data
	return 0, nil, eoferr
}

func splitSpaceOrLF(r rune) bool {
	return r == ' ' || r == '\n' || r == '\r'
}
