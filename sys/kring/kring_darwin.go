package kring

import (
	"crypto/rand"
	"fmt"
	"os"

	"github.com/awnumar/memguard"
	"github.com/keybase/go-keychain"
	"golang.org/x/crypto/ed25519"
)

func storeSecret(service string, account string, label string, data *memguard.LockedBuffer) (err error) {
	exec, err := os.Executable()
	if err != nil {
		return err
	}
	item := keychain.NewItem()
	item.SetSecClass(keychain.SecClassGenericPassword)
	item.SetService(service)
	item.SetAccount(account)
	item.SetLabel(label)
	item.SetData(data.Buffer())
	item.SetAccess(&keychain.Access{
		Label:               "skewer",
		TrustedApplications: []string{exec},
	})
	return keychain.AddItem(item)
}

func getItem(service string, account string, label string) (res []byte, err error) {
	query := keychain.NewItem()
	query.SetSecClass(keychain.SecClassGenericPassword)
	query.SetService(service)
	query.SetAccount(account)
	query.SetLabel(label)
	query.SetMatchLimit(keychain.MatchLimitOne)
	query.SetReturnData(true)
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

func getSecret(service string, account string, label string) (secret *memguard.LockedBuffer, err error) {
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

func NewSignaturePubkey(session string) (pubkey *memguard.LockedBuffer, privkey *memguard.LockedBuffer, err error) {
	pub, priv, err := ed25519.GenerateKey(nil)
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
	err = storeSecret("skewer-sigpubkey", session, "sigpubkey", pubkey)
	if err != nil {
		pubkey.Destroy()
		privkey.Destroy()
		return nil, nil, err
	}
	return pubkey, privkey, nil
}

func GetSignaturePubkey(session string) (pubkey *memguard.LockedBuffer, err error) {
	pubkey, err = getSecret("skewer-sigpubkey", session, "sigpubkey")
	if err != nil {
		return nil, err
	}
	return pubkey, nil
}

func NewBoxSecret(session string) (secret *memguard.LockedBuffer, err error) {
	secretKey := make([]byte, 32)
	_, err = rand.Read(secretKey)
	if err != nil {
		return nil, err
	}
	secret, err = memguard.NewImmutableFromBytes(secretKey)
	if err != nil {
		return nil, err
	}
	err = storeSecret("skewer-secret", session, "boxsecret", secret)
	if err != nil {
		secret.Destroy()
		return nil, err
	}
	return secret, nil
}

func GetBoxSecret(session string) (secret *memguard.LockedBuffer, err error) {
	secret, err = getSecret("skewer-secret", session, "boxsecret")
	if err != nil {
		return nil, err
	}
	return secret, nil
}

func DeleteBoxSecret(session string) error {
	return keychain.DeleteGenericPasswordItem("skewer-secret", session)
}

func DeleteSignaturePubKey(session string) error {
	return keychain.DeleteGenericPasswordItem("skewer-sigpubkey", session)
}

func JoinSessionKeyRing() error {
	return nil
}
