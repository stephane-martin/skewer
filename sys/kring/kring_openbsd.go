package kring

import (
	"github.com/awnumar/memguard"
)

func GetSignaturePubkey(session string) (pubkey *memguard.LockedBuffer, err error) {
	return nil, nil
}

func NewSignaturePubkey(session string) (pubkey *memguard.LockedBuffer, privkey *memguard.LockedBuffer, err error) {
	return nil, nil, nil
}

func NewBoxSecret(session string) (secret *memguard.LockedBuffer, err error) {
	return nil, nil
}

func GetBoxSecret(session string) (secret *memguard.LockedBuffer, err error) {
	return nil, nil
}

func DeleteBoxSecret(session string) error {
	return nil
}

func DeleteSignaturePubKey(session string) error {

}

func JoinSessionKeyRing() error {
	return nil
}
