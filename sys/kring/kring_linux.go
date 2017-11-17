package kring

import (
	"crypto/rand"
	"fmt"
	"syscall"

	"github.com/awnumar/memguard"
	"github.com/jsipprell/keyctl"
	"golang.org/x/crypto/ed25519"
)

func getSecret(session string, label string) (pubkey *memguard.LockedBuffer, err error) {
	keyring, err := keyctl.SessionKeyring()
	if err != nil {
		return nil, err
	}
	key, err := keyring.Search(fmt.Sprintf("skewer-%s-%s", label, session))
	if err != nil {
		return nil, err
	}
	data, err := key.Get()
	if err != nil {
		return nil, err
	}
	secret, err := memguard.NewImmutableFromBytes(data)
	if err != nil {
		return nil, err
	}
	return secret, nil
}

func GetSignaturePubkey(session string) (pubkey *memguard.LockedBuffer, err error) {
	return getSecret(session, "sigpubkey")
}

func NewSignaturePubkey(session string) (pubkey *memguard.LockedBuffer, privkey *memguard.LockedBuffer, err error) {
	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		return nil, nil, err
	}
	keyring, err := keyctl.SessionKeyring()
	if err != nil {
		return nil, nil, err
	}
	pubkey, err = memguard.NewImmutableFromBytes(pub)
	if err != nil {
		return nil, nil, err
	}
	privkey, err = memguard.NewImmutableFromBytes(priv)
	if err != nil {
		return nil, nil, err
	}
	_, err = keyring.Add(fmt.Sprintf("skewer-sigpubkey-%s", session), pubkey.Buffer())
	if err != nil {
		pubkey.Destroy()
		privkey.Destroy()
		return nil, nil, err
	}
	return pubkey, privkey, nil
}

func NewBoxSecret(session string) (secret *memguard.LockedBuffer, err error) {
	secretKey := make([]byte, 32)
	_, err = rand.Read(secretKey)
	if err != nil {
		return nil, err
	}
	keyring, err := keyctl.SessionKeyring()
	if err != nil {
		return nil, err
	}
	secret, err = memguard.NewImmutableFromBytes(secretKey)
	if err != nil {
		return nil, err
	}
	_, err = keyring.Add(fmt.Sprintf("skewer-boxsecret-%s", session), secret.Buffer())
	if err != nil {
		secret.Destroy()
		return nil, err
	}
	return secret, nil
}

func GetBoxSecret(session string) (secret *memguard.LockedBuffer, err error) {
	return getSecret(session, "boxsecret")
}

func DeleteBoxSecret(session string) error {
	keyring, err := keyctl.SessionKeyring()
	if err != nil {
		return err
	}
	key, err := keyring.Search(fmt.Sprintf("skewer-boxsecret-%s", session))
	if err != nil {
		return err
	}
	return key.Unlink()
}

func JoinSessionKeyRing() error {
	a := make([]uintptr, 6)
	a[0] = 1
	_, _, errno := syscall.Syscall6(syscall_keyctl, a[0], a[1], a[2], a[3], a[4], a[5])
	if errno != 0 {
		return errno
	}
	return nil
}
