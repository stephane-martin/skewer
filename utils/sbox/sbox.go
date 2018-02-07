package sbox

import (
	"crypto/rand"
	"fmt"
	"unsafe"

	"github.com/awnumar/memguard"
	"golang.org/x/crypto/nacl/secretbox"
)

func sliceForAppend(in []byte, n int) (head, tail []byte) {
	if total := len(in) + n; cap(in) >= total {
		head = in[:total]
	} else {
		head = make([]byte, total)
		copy(head, in)
	}
	tail = head[len(in):]
	return
}

func LenEncrypted(message []byte) int {
	return len(message) + 24 + secretbox.Overhead
}

func LenDecrypted(encrypted []byte) int {
	return len(encrypted) - 24 - secretbox.Overhead
}

func EncryptTo(message []byte, secret *memguard.LockedBuffer, out []byte) (encrypted []byte, err error) {
	if secret == nil {
		return nil, fmt.Errorf("Encrypt: nil secret")
	}
	if len(message) == 0 {
		return nil, fmt.Errorf("Encrypt: empty message")
	}
	encrypted, out = sliceForAppend(out, LenEncrypted(message))
	_, err = rand.Read(out[:24])
	if err != nil {
		return nil, err
	}
	secretbox.Seal(out[:24], message, (*[24]byte)(unsafe.Pointer(&(out[0]))), (*[32]byte)(unsafe.Pointer(&(secret.Buffer()[0]))))
	//fmt.Fprintln(os.Stderr, "ENC", len(message))
	return encrypted, nil
}

func Encrypt(message []byte, secret *memguard.LockedBuffer) (encrypted []byte, err error) {
	return EncryptTo(message, secret, nil)
}

func Decrypt(encrypted []byte, secret *memguard.LockedBuffer) (decrypted []byte, err error) {
	return DecrypTo(encrypted, secret, nil)
}

func DecrypTo(encrypted []byte, secret *memguard.LockedBuffer, out []byte) (decrypted []byte, err error) {
	if secret == nil {
		return nil, fmt.Errorf("Decrypt: nil secret")
	}
	length := LenDecrypted(encrypted)
	if length <= 0 {
		return nil, fmt.Errorf("Decrypt: encrypted message too short")
	}
	decrypted, _ = sliceForAppend(out, length)
	var ok bool
	_, ok = secretbox.Open(decrypted[:len(out)], encrypted[24:], (*[24]byte)(unsafe.Pointer(&(encrypted[0]))), (*[32]byte)(unsafe.Pointer(&(secret.Buffer()[0]))))
	if !ok {
		return nil, fmt.Errorf("Error decrypting value")
	}
	return decrypted, nil
}
