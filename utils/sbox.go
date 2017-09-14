package utils

import (
	"crypto/rand"
	"fmt"

	"golang.org/x/crypto/nacl/secretbox"
)

func Encrypt(message []byte, secret [32]byte) (encrypted []byte, err error) {
	prenonce := make([]byte, 24)
	_, err = rand.Read(prenonce)
	if err != nil {
		return nil, err
	}
	var nonce [24]byte
	copy(nonce[:], prenonce)
	encrypted = secretbox.Seal(nonce[:], message, &nonce, &secret)
	return encrypted, nil
}

func Decrypt(encrypted []byte, secret [32]byte) (decrypted []byte, err error) {
	var nonce [24]byte
	var ok bool
	copy(nonce[:], encrypted[:24])
	decrypted, ok = secretbox.Open(nil, encrypted[24:], &nonce, &secret)
	if !ok {
		return nil, fmt.Errorf("Error decrypting value")
	}
	return decrypted, nil
}
