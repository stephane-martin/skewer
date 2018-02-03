package utils

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"net"
	"strconv"

	"github.com/awnumar/memguard"
	"github.com/stephane-martin/skewer/utils/sbox"
	"golang.org/x/crypto/ed25519"
	"golang.org/x/crypto/nacl/secretbox"
)

var NOW []byte = []byte("now")
var SP []byte = []byte(" ")

type SigWriter struct {
	dest io.Writer
	key  *memguard.LockedBuffer
}

func NewSignatureWriter(dest io.Writer, privsignkey *memguard.LockedBuffer) *SigWriter {
	return &SigWriter{dest: dest, key: privsignkey}
}

func (s *SigWriter) Write(p []byte) (n int, err error) {
	if len(p) == 0 {
		return 0, nil
	}
	signature := ed25519.Sign(s.key.Buffer(), p)
	err = ChainWrites(
		s.dest,
		[]byte(fmt.Sprintf("%010d %010d ", len(p), len(signature))),
		p,
		signature,
	)
	if err == nil {
		return len(p), nil
	}
	return 0, err
}

func (s *SigWriter) WriteWithHeader(header []byte, message []byte) (err error) {
	fullmessage := make([]byte, 0, len(header)+len(message)+1)
	fullmessage = append(fullmessage, header...)
	fullmessage = append(fullmessage, SP...)
	fullmessage = append(fullmessage, message...)
	_, err = s.Write(fullmessage)
	return err
}

// MakeSignSplit returns a split function that checks for message signatures.
func MakeSignSplit(signpubkey *memguard.LockedBuffer) (signSplit bufio.SplitFunc) {

	signSplit = func(data []byte, atEOF bool) (advance int, token []byte, eoferr error) {
		if atEOF {
			eoferr = io.EOF
		}
		if len(data) < 22 {
			return 0, nil, eoferr
		}
		if data[10] != byte(' ') || data[21] != byte(' ') {
			return 0, nil, fmt.Errorf("Wrong sign format, 11th or 22th char is not space: '%s'", string(data))
		}

		var i int
		for i = 0; i < 10; i++ {
			if data[i] < byte('0') || data[i] > byte('9') {
				return 0, nil, fmt.Errorf("Wrong sign format")
			}
		}
		for i = 11; i < 21; i++ {
			if data[i] < byte('0') || data[i] > byte('9') {
				return 0, nil, fmt.Errorf("Wrong sign format")
			}
		}
		fullmessagelen, err := strconv.Atoi(string(data[:10]))
		if err != nil {
			return 0, nil, err
		}
		signaturelen, err := strconv.Atoi(string(data[11:21]))
		if err != nil {
			return 0, nil, err
		}

		advance = 22 + fullmessagelen + signaturelen
		if len(data) < advance {
			return 0, nil, eoferr
		}
		fullmessage := data[22 : 22+fullmessagelen]
		signature := data[22+fullmessagelen : 22+fullmessagelen+signaturelen]
		if ed25519.Verify(signpubkey.Buffer(), fullmessage, signature) {
			return advance, fullmessage, nil
		}
		return 0, nil, fmt.Errorf("Wrong signature")
	}
	return signSplit
}

type EncryptWriter struct {
	dest io.Writer
	key  *memguard.LockedBuffer
}

func NewEncryptWriter(dest io.Writer, encryptkey *memguard.LockedBuffer) *EncryptWriter {
	return &EncryptWriter{dest: dest, key: encryptkey}
}

func (s *EncryptWriter) WriteMsgUnix(b []byte, oob []byte, addr *net.UnixAddr) (n int, oobn int, err error) {
	if len(b) == 0 {
		return 0, 0, nil
	}
	if conn, ok := s.dest.(*net.UnixConn); ok {
		var enc []byte
		if s.key == nil {
			enc = b
		} else {
			enc, err = sbox.Encrypt(b, s.key)
			if err != nil {
				return 0, 0, err
			}
		}
		buf := bytes.NewBuffer(nil)
		buf.Write([]byte(fmt.Sprintf("%010d ", len(enc))))
		buf.Write(enc)
		return conn.WriteMsgUnix(buf.Bytes(), oob, addr)
	}
	return 0, 0, nil
}

func (s *EncryptWriter) Write(p []byte) (n int, err error) {
	if len(p) == 0 {
		return 0, nil
	}
	var buf []byte
	if s.key == nil {
		encLength := len(p)
		buf = make([]byte, 0, 11+encLength)
		buf = append(buf, fmt.Sprintf("%010d ", encLength)...)
		buf = append(buf, p...)
	} else {
		encLength := len(p) + 24 + secretbox.Overhead
		buf = make([]byte, 11+encLength)
		_, err = sbox.EncryptTo(p, s.key, buf[:11])
		if err != nil {
			return 0, err
		}
		copy(buf[:11], fmt.Sprintf("%010d ", encLength))
	}
	_, err = s.dest.Write(buf)
	if err == nil {
		return len(p), nil
	}
	return 0, err
}

func (s *EncryptWriter) WriteWithHeader(header []byte, message []byte) (err error) {
	fullmessage := make([]byte, 0, len(header)+len(message)+1)
	fullmessage = append(fullmessage, header...)
	fullmessage = append(fullmessage, SP...)
	fullmessage = append(fullmessage, message...)
	_, err = s.Write(fullmessage)
	return err
}

// MakeDecryptSplit returns a split function that extracts and decrypts messages.
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

// PluginSplit is a split function used by plugins.
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
	return advance, data[11:advance], nil
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
