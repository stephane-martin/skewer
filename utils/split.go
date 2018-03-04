package utils

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"net"
	"strconv"
	"sync"

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
	pool *sync.Pool
}

func NewEncryptWriter(dest io.Writer, encryptkey *memguard.LockedBuffer) *EncryptWriter {
	return &EncryptWriter{
		dest: dest,
		key:  encryptkey,
		pool: &sync.Pool{
			New: func() interface{} {
				return make([]byte, 4096)
			},
		},
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
	var bufpooled []byte
	if s.key == nil {
		encLength := len(p)
		buf = make([]byte, 0, 11+encLength)
		buf = append(buf, fmt.Sprintf("%010d ", encLength)...)
		buf = append(buf, p...)
	} else {
		encLength := len(p) + 24 + secretbox.Overhead
		if encLength <= (4096 - 11) {
			bufpooled = s.pool.Get().([]byte)
			defer s.pool.Put(bufpooled)
			buf = bufpooled[:11+encLength]
		} else {
			buf = make([]byte, 11+encLength)
		}
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
			dec, err = sbox.DecrypTo(tok, secret, buf[:0])
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
