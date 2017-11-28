package kring

import (
	"crypto/rand"
	"fmt"
	"io"
	"os"
	"syscall"

	"github.com/awnumar/memguard"
	"github.com/jsipprell/keyctl"
	"github.com/oklog/ulid"
	"github.com/stephane-martin/skewer/sys/semaphore"
	"golang.org/x/crypto/ed25519"
)

func joinSessionKeyRing() error {
	_, _, errno := syscall.Syscall6(syscall_keyctl, 1, 0, 0, 0, 0, 0)
	if errno != 0 {
		return errno
	}
	return nil
}

type ring struct {
	creds RingCreds
}

func GetRing(creds RingCreds) Ring {
	return &ring{creds: creds}
}

func (r *ring) Destroy() {
	r.creds.Secret.Destroy()
	destroySem(r.creds.SessionID)
}

func NewRing() (r Ring, err error) {
	err = joinSessionKeyRing()
	if err != nil {
		return nil, err
	}
	creds, err := NewCreds()
	if err != nil {
		return nil, err
	}
	return GetRing(creds), nil
}

func (r *ring) WriteRingPass(w io.Writer) (err error) {
	_, err = w.Write(r.creds.Secret.Buffer())
	return err
}

func (r *ring) GetSessionID() ulid.ULID {
	return r.creds.SessionID
}

func getSecret(session ulid.ULID, label string) (pubkey *memguard.LockedBuffer, err error) {
	sessionStr := session.String()
	sem, err := semaphore.New(fmt.Sprintf("skw%s", sessionStr))
	if err != nil {
		fmt.Fprintln(os.Stderr, "new semaphore error", err)
		return nil, err
	}
	err = sem.Lock()
	if err != nil {
		fmt.Fprintln(os.Stderr, "semaphore lock error", err)
		return nil, err
	}
	defer func() {
		sem.Unlock()
		sem.Close()
	}()

	keyring, err := keyctl.SessionKeyring()
	if err != nil {
		return nil, err
	}
	key, err := keyring.Search(fmt.Sprintf("skewer-%s-%s", label, sessionStr))
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

func (r *ring) GetSignaturePubkey() (pubkey *memguard.LockedBuffer, err error) {
	return getSecret(r.creds.SessionID, "sigpubkey")
}

func (r *ring) NewSignaturePubkey() (privkey *memguard.LockedBuffer, err error) {
	sessionStr := r.creds.SessionID.String()
	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		return nil, err
	}

	sem, err := semaphore.New(fmt.Sprintf("skw%s", sessionStr))
	if err != nil {
		return nil, err
	}
	err = sem.Lock()
	if err != nil {
		return nil, err
	}
	defer func() {
		sem.Unlock()
		sem.Close()
	}()

	keyring, err := keyctl.SessionKeyring()
	if err != nil {
		return nil, err
	}
	pubkey, err := memguard.NewImmutableFromBytes(pub)
	if err != nil {
		return nil, err
	}
	privkey, err = memguard.NewImmutableFromBytes(priv)
	if err != nil {
		return nil, err
	}
	_, err = keyring.Add(fmt.Sprintf("skewer-sigpubkey-%s", sessionStr), pubkey.Buffer())
	pubkey.Destroy()
	if err != nil {
		privkey.Destroy()
		return nil, err
	}
	return privkey, nil
}

func (r *ring) NewBoxSecret() (secret *memguard.LockedBuffer, err error) {
	sessionStr := r.creds.SessionID.String()
	secretKey := make([]byte, 32)
	_, err = rand.Read(secretKey)
	if err != nil {
		return nil, err
	}

	sem, err := semaphore.New(fmt.Sprintf("skw%s", sessionStr))
	if err != nil {
		return nil, err
	}
	err = sem.Lock()
	if err != nil {
		return nil, err
	}
	defer func() {
		sem.Unlock()
		sem.Close()
	}()

	keyring, err := keyctl.SessionKeyring()
	if err != nil {
		return nil, err
	}
	secret, err = memguard.NewImmutableFromBytes(secretKey)
	if err != nil {
		return nil, err
	}
	_, err = keyring.Add(fmt.Sprintf("skewer-boxsecret-%s", sessionStr), secret.Buffer())
	if err != nil {
		secret.Destroy()
		return nil, err
	}
	return secret, nil
}

func (r *ring) GetBoxSecret() (secret *memguard.LockedBuffer, err error) {
	return getSecret(r.creds.SessionID, "boxsecret")
}

func (r *ring) DeleteBoxSecret() error {
	sessionStr := r.creds.SessionID.String()
	sem, err := semaphore.New(fmt.Sprintf("skw%s", sessionStr))
	if err != nil {
		return err
	}
	err = sem.Lock()
	if err != nil {
		return err
	}
	defer func() {
		sem.Unlock()
		sem.Close()
	}()

	keyring, err := keyctl.SessionKeyring()
	if err != nil {
		return err
	}
	key, err := keyring.Search(fmt.Sprintf("skewer-boxsecret-%s", sessionStr))
	if err != nil {
		return err
	}
	return key.Unlink()
}

func (r *ring) DeleteSignaturePubKey() error {
	sessionStr := r.creds.SessionID.String()
	sem, err := semaphore.New(fmt.Sprintf("skw%s", sessionStr))
	if err != nil {
		return err
	}
	err = sem.Lock()
	if err != nil {
		return err
	}
	defer func() {
		sem.Unlock()
		sem.Close()
	}()

	keyring, err := keyctl.SessionKeyring()
	if err != nil {
		return err
	}
	key, err := keyring.Search(fmt.Sprintf("skewer-sigpubkey-%s", sessionStr))
	if err != nil {
		return err
	}
	return key.Unlink()
}
