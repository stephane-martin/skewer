package kring

import (
	"fmt"
	"io"
	"os"

	"github.com/awnumar/memguard"
	"github.com/keybase/go-keychain"
	"github.com/oklog/ulid"
	"github.com/stephane-martin/skewer/sys/semaphore"
	"golang.org/x/crypto/ed25519"
)

type ring struct {
	creds RingCreds
}

func GetRing(creds RingCreds) Ring {
	return &ring{creds: creds}
}

func storeSecret(service string, account ulid.ULID, label string, data *memguard.LockedBuffer) (err error) {
	exec, err := os.Executable()
	if err != nil {
		return err
	}
	accountStr := account.String()
	item := keychain.NewItem()
	item.SetSecClass(keychain.SecClassGenericPassword)
	item.SetService(service)
	item.SetAccount(accountStr)
	item.SetLabel(label)
	item.SetData(data.Buffer())
	item.SetAccess(&keychain.Access{
		Label:               "skewer",
		TrustedApplications: []string{exec},
	})
	sem, err := semaphore.New(fmt.Sprintf("skw%s", accountStr))
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
	return keychain.AddItem(item)
}

func getItem(service string, account ulid.ULID, label string) (res []byte, err error) {
	accountStr := account.String()
	query := keychain.NewItem()
	query.SetSecClass(keychain.SecClassGenericPassword)
	query.SetService(service)
	query.SetAccount(accountStr)
	query.SetLabel(label)
	query.SetMatchLimit(keychain.MatchLimitOne)
	query.SetReturnData(true)
	sem, err := semaphore.New(fmt.Sprintf("skw%s", accountStr))
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
	results, err := keychain.QueryItem(query)
	if err != nil {
		return nil, err
	}
	if len(results) > 1 {
		return nil, fmt.Errorf("Too many results")
	}
	if len(results) == 1 {
		return results[0].Data, nil
	}
	return nil, fmt.Errorf("Unknown secret")
}

func getSecret(service string, account ulid.ULID, label string) (secret *memguard.LockedBuffer, err error) {
	item, err := getItem(service, account, label)
	if err != nil {
		return nil, err
	}
	secret, err = memguard.NewImmutableFromBytes(item)
	if err != nil {
		return nil, err
	}
	return secret, nil
}

func (r *ring) NewSignaturePubkey() (privkey *memguard.LockedBuffer, err error) {
	pub, priv, err := ed25519.GenerateKey(nil)
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
	err = storeSecret("skewer-sigpubkey", r.creds.SessionID, "sigpubkey", pubkey)
	pubkey.Destroy()
	if err != nil {
		privkey.Destroy()
		return nil, err
	}
	return privkey, nil
}

func (r *ring) GetSignaturePubkey() (pubkey *memguard.LockedBuffer, err error) {
	pubkey, err = getSecret("skewer-sigpubkey", r.creds.SessionID, "sigpubkey")
	if err != nil {
		return nil, err
	}
	return pubkey, nil
}

func (r *ring) NewBoxSecret() (secret *memguard.LockedBuffer, err error) {
	secret, err = NewSecret()
	if err != nil {
		return nil, err
	}
	err = storeSecret("skewer-secret", r.creds.SessionID, "boxsecret", secret)
	if err != nil {
		secret.Destroy()
		return nil, err
	}
	return secret, nil
}

func (r *ring) GetBoxSecret() (secret *memguard.LockedBuffer, err error) {
	secret, err = getSecret("skewer-secret", r.creds.SessionID, "boxsecret")
	if err != nil {
		return nil, err
	}
	return secret, nil
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

	return keychain.DeleteGenericPasswordItem("skewer-secret", sessionStr)
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

	return keychain.DeleteGenericPasswordItem("skewer-sigpubkey", sessionStr)
}

func (r *ring) Destroy() {
	r.creds.Secret.Destroy()
	destroySem(r.creds.SessionID)
}

func NewRing() (r Ring, err error) {
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
