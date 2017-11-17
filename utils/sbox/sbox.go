package sbox

import (
	"crypto/rand"
	"fmt"
	"unsafe"

	"github.com/awnumar/memguard"
	"golang.org/x/crypto/nacl/secretbox"
)

func Encrypt(message []byte, secret *memguard.LockedBuffer) (encrypted []byte, err error) {
	prenonce := make([]byte, 24)
	_, err = rand.Read(prenonce)
	if err != nil {
		return nil, err
	}
	var nonce [24]byte
	copy(nonce[:], prenonce)

	encrypted = secretbox.Seal(nonce[:], message, &nonce, (*[32]byte)(unsafe.Pointer(&secret.Buffer()[0])))
	return encrypted, nil
}

func Decrypt(encrypted []byte, secret *memguard.LockedBuffer) (decrypted []byte, err error) {
	var nonce [24]byte
	var ok bool
	copy(nonce[:], encrypted[:24])
	decrypted, ok = secretbox.Open(nil, encrypted[24:], &nonce, (*[32]byte)(unsafe.Pointer(&secret.Buffer()[0])))
	if !ok {
		return nil, fmt.Errorf("Error decrypting value")
	}
	return decrypted, nil
}
