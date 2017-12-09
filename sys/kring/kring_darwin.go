package kring

import (
	"fmt"
	"io"
	"os"

	"github.com/awnumar/memguard"
	"github.com/keybase/go-keychain"
	"github.com/oklog/ulid"
	"github.com/stephane-martin/skewer/sys/semaphore"
	"github.com/stephane-martin/skewer/utils/sbox"
	"golang.org/x/crypto/ed25519"
)

type ring struct {
	creds RingCreds
}

func GetRing(creds RingCreds) Ring {
	return &ring{creds: creds}
}

func storeSecret(service string, creds RingCreds, label string, data *memguard.LockedBuffer) (err error) {
	exec, err := os.Executable()
	if err != nil {
		return err
	}
	encrypted, err := sbox.Encrypt(data.Buffer(), creds.Secret)
	if err != nil {
		return err
	}
	accountStr := creds.SessionID.String()
	item := keychain.NewItem()
	item.SetSecClass(keychain.SecClassGenericPassword)
	item.SetService(service)
	item.SetAccount(accountStr)
	item.SetLabel(label)
	item.SetData(encrypted)
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
		err := sem.Unlock()
		if err != nil {
			panic(err)
		}
		_ = sem.Close()
	}()
	return keychain.AddItem(item)
}

func getItem(service string, creds RingCreds, label string) (res []byte, err error) {
	accountStr := creds.SessionID.String()
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
		err := sem.Unlock()
		if err != nil {
			panic(err)
		}
		_ = sem.Close()
	}()
	results, err := keychain.QueryItem(query)
	if err != nil {
		return nil, err
	}
	if len(results) > 1 {
		return nil, fmt.Errorf("Too many results")
	}
	if len(results) == 1 {
		decrypted, err := sbox.Decrypt(results[0].Data, creds.Secret)
		if err != nil {
			return nil, err
		}
		return decrypted, nil
	}
	return nil, fmt.Errorf("Unknown secret")
}

func getSecret(service string, creds RingCreds, label string) (secret *memguard.LockedBuffer, err error) {
	item, err := getItem(service, creds, label)
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
	err = storeSecret("skewer-sigpubkey", r.creds, "sigpubkey", pubkey)
	pubkey.Destroy()
	if err != nil {
		privkey.Destroy()
		return nil, err
	}
	return privkey, nil
}

func (r *ring) GetSignaturePubkey() (pubkey *memguard.LockedBuffer, err error) {
	pubkey, err = getSecret("skewer-sigpubkey", r.creds, "sigpubkey")
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
	err = storeSecret("skewer-secret", r.creds, "boxsecret", secret)
	if err != nil {
		secret.Destroy()
		return nil, err
	}
	return secret, nil
}

func (r *ring) GetBoxSecret() (secret *memguard.LockedBuffer, err error) {
	secret, err = getSecret("skewer-secret", r.creds, "boxsecret")
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
		err := sem.Unlock()
		if err != nil {
			panic(err)
		}
		_ = sem.Close()
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
		err := sem.Unlock()
		if err != nil {
			panic(err)
		}
		_ = sem.Close()
	}()

	return keychain.DeleteGenericPasswordItem("skewer-sigpubkey", sessionStr)
}

func (r *ring) Destroy() error {
	r.creds.Secret.Destroy()
	return destroySem(r.creds.SessionID)
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
