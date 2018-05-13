package utils

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
	"sync"

	"github.com/awnumar/memguard"
	"github.com/stephane-martin/skewer/utils/sbox"
	"golang.org/x/crypto/ed25519"
	"golang.org/x/crypto/nacl/secretbox"
)

var NOW []byte = []byte("now")
var SP []byte = []byte(" ")

var spool = &sync.Pool{
	New: func() interface{} {
		return make([]byte, 4096)
	},
}

func getBuf() []byte {
	return spool.Get().([]byte)[:0]
}

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
	var b strings.Builder
	b.Grow(22 + len(p) + len(signature))
	b.WriteString(fmt.Sprintf("%010d %010d ", len(p), len(signature)))
	b.Write(p)
	b.Write(signature)
	_, err = io.WriteString(s.dest, b.String())
	if err == nil {
		return len(p), nil
	}
	return 0, err
}

func (s *SigWriter) WriteWithHeader(header []byte, message []byte) (err error) {
	var b strings.Builder
	b.Grow(len(header) + len(message) + 1)
	b.Write(header)
	b.Write(SP)
	b.Write(message)
	_, err = io.WriteString(s, b.String())
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
		if data[10] != sp || data[21] != sp {
			return 0, nil, fmt.Errorf("Wrong sign format, 11th or 22th char is not space: '%s'", string(data))
		}

		var i int
		for i = 0; i < 10; i++ {
			if data[i] < zero || data[i] > nine {
				return 0, nil, fmt.Errorf("Wrong sign format")
			}
		}
		for i = 11; i < 21; i++ {
			if data[i] < zero || data[i] > nine {
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
	return &EncryptWriter{
		dest: dest,
		key:  encryptkey,
	}
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
		buf.Grow(11 + len(enc))
		io.WriteString(buf, fmt.Sprintf("%010d ", len(enc)))
		buf.Write(enc)
		return conn.WriteMsgUnix(buf.Bytes(), oob, addr)
	}
	return 0, 0, nil
}

func (s *EncryptWriter) Write(p []byte) (n int, err error) {
	if len(p) == 0 {
		return 0, nil
	}
	buf := getBuf()
	if s.key == nil {
		encLength := len(p)
		buf = append(buf, fmt.Sprintf("%010d ", encLength)...)
		buf = append(buf, p...)
	} else {
		encLength := len(p) + 24 + secretbox.Overhead
		if cap(buf) < 11+encLength {
			buf = make([]byte, 11+encLength)
		}
		buf = buf[:11+encLength]
		_, err = sbox.EncryptTo(p, s.key, buf[:11])
		if err != nil {
			return 0, err
		}
		copy(buf[:11], fmt.Sprintf("%010d ", encLength))
	}
	// ensure we make a *copy* of buf before we pass it to s.dest
	// then in any cases we can release buf afterwards
	_, err = io.WriteString(s.dest, string(buf))
	spool.Put(buf)
	if err != nil {
		return 0, err
	}
	return len(p), nil
}

func (s *EncryptWriter) WriteWithHeader(header []byte, message []byte) (err error) {
	var b strings.Builder
	b.Grow(len(header) + len(message) + 1)
	b.Write(header)
	b.Write(SP)
	b.Write(message)
	_, err = io.WriteString(s, b.String())
	return err
}

// MakeDecryptSplit returns a split function that extracts and decrypts messages.
func MakeDecryptSplit(secret *memguard.LockedBuffer) bufio.SplitFunc {
	buf := make([]byte, 0, 4096)
	// - we assume that spl will be called by a single goroutine
	// - spl may return tokens that rely on the same backing array
	spl := func(data []byte, atEOF bool) (adv int, dec []byte, err error) {
		var tok []byte
		adv, tok, err = PluginSplit(data, atEOF)
		if err != nil || tok == nil || secret == nil {
			return adv, tok, err
		}
		if sbox.LenDecrypted(tok) <= 4096 {
			dec, err = sbox.DecryptTo(tok, secret, buf[:0])
		} else {
			dec, err = sbox.Decrypt(tok, secret)
		}
		if err != nil {
			return 0, nil, err
		}
		return adv, dec, nil
	}
	return spl
}

var sp = byte(' ')
var zero = byte('0')
var nine = byte('9')

// PluginSplit is a split function used by plugins.
func PluginSplit(data []byte, atEOF bool) (advance int, token []byte, eoferr error) {
	if atEOF {
		eoferr = io.EOF
	}
	if len(data) < 11 {
		return 0, nil, eoferr
	}
	if data[10] != sp {
		return 0, nil, fmt.Errorf("Wrong plugin format, 11th char is not space: '%s'", string(data))
	}
	var i int
	for i = 0; i < 10; i++ {
		if data[i] < zero || data[i] > nine {
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
	// TXNR COMMAND DATALEN[ DATA]\n
	fields, adv := NFields(data, 3)
	if len(fields) < 3 || adv == len(data) {
		return 0, nil, eoferr
	}

	txnrB := fields[0]
	command := fields[1]
	datalenB := fields[2]

	_, err := strconv.Atoi(string(txnrB))
	if err != nil {
		return 0, nil, err
	}

	datalen, err := strconv.Atoi(string(datalenB))
	if err != nil {
		return 0, nil, err
	}

	token = make([]byte, 0, adv+datalen+2)
	token = append(token, txnrB...)
	token = append(token, ' ')
	token = append(token, command...)
	if datalen == 0 {
		return adv, token, nil
	}

	advance = adv + (datalen + 1) + 1 // SP DATA LF
	if len(data) >= advance {
		token = append(token, ' ')
		token = append(token, bytes.TrimSpace(data[adv+1:advance])...)
		return advance, token, nil
	}
	return 0, nil, eoferr
}

func NFields(s []byte, n int) (fields [][]byte, advance int) {
	if n == 0 {
		return nil, 0
	}
	if n == -1 {
		n = bytes.Count(s, SP) + 1
	}
	fields = make([][]byte, 0, n)
	var adv, nextsp int

	for len(fields) < n {
		s, adv = ltrim(s)
		advance += adv
		if len(s) == 0 {
			return
		}
		nextsp = bytes.IndexAny(s, " \r\n\t")
		if nextsp < 0 {
			fields = append(fields, s[:len(s):len(s)])
			advance += len(s)
			return
		}
		fields = append(fields, s[:nextsp:nextsp])
		advance += nextsp
		s = s[nextsp:]
	}
	return
}

func ltrim(s []byte) (ret []byte, advance int) {
	ret = bytes.TrimLeft(s, " \r\n\t")
	advance = len(s) - len(ret)
	return
}
