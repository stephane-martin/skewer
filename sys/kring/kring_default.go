package kring

import (
	"fmt"
	"io"
	"unsafe"

	"github.com/awnumar/memguard"
	"github.com/stephane-martin/go-semaphore"
	"github.com/stephane-martin/skewer/sys/shm"
	"github.com/stephane-martin/skewer/utils"
	"github.com/stephane-martin/skewer/utils/sbox"
	"golang.org/x/crypto/ed25519"
	"golang.org/x/crypto/nacl/secretbox"
)

type defring struct {
	creds   RingCreds
	sh      *shm.SharedMem
	shmName string
	semName string
}

func GetDefRing(creds RingCreds) Ring {
	r := defring{creds: creds}
	r.shmName = fmt.Sprintf("skm%s", creds.SessionID.String())
	r.semName = fmt.Sprintf("skw%s", creds.SessionID.String())
	return &r
}

func (r *defring) Destroy() error {
	r.creds.Secret.Destroy()
	if r.sh != nil {
		err := r.sh.Close()
		if err != nil {
			return err
		}
		err = r.sh.Delete()
		if err != nil {
			return err
		}
	}
	return semaphore.Destroy(r.semName)
}

func NewDefRing() (Ring, error) {
	creds, err := NewCreds()
	if err != nil {
		return nil, err
	}
	r := defring{creds: creds}
	r.shmName = fmt.Sprintf("skm%s", creds.SessionID.String())
	r.semName = fmt.Sprintf("skw%s", creds.SessionID.String())
	sh, err := shm.Create(r.shmName, int(sizeOfBag))
	if err != nil {
		return nil, err
	}
	r.sh = sh
	return &r, nil
}

func (r *defring) WriteRingPass(w io.Writer) (err error) {
	_, err = w.Write(r.creds.Secret.Buffer())
	return err
}

func (r *defring) GetSessionID() utils.MyULID {
	return r.creds.SessionID
}

func (r *defring) NewSignaturePubkey() (privkey *memguard.LockedBuffer, err error) {
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

	sem, err := semaphore.New(r.semName, 1)
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

	sh := r.sh
	if r.sh == nil {
		sh, err = shm.Open(r.shmName)
		if err != nil {
			return nil, err
		}
		defer func() {
			err = sh.Close()
			if err != nil {
				privkey = nil
			}
		}()
	}

	mybag := (*bag)(sh.Pointer())
	err = mybag.SetSigPubKey(pubkey, r.creds)
	pubkey.Destroy()
	if err != nil {
		return nil, err
	}

	return privkey, nil

}

func (r *defring) GetSignaturePubkey() (pubkey *memguard.LockedBuffer, err error) {
	sem, err := semaphore.New(r.semName, 1)
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

	sh := r.sh
	if r.sh == nil {
		sh, err = shm.Open(r.shmName)
		if err != nil {
			return nil, err
		}
		defer func() {
			err = sh.Close()
			if err != nil {
				pubkey = nil
			}
		}()
	}

	mybag := (*bag)(sh.Pointer())
	pubkey, err = mybag.GetSigPubKey(r.creds)
	if err != nil {
		return nil, err
	}
	return pubkey, nil
}

func (r *defring) NewBoxSecret() (secret *memguard.LockedBuffer, err error) {
	secret, err = NewSecret()
	if err != nil {
		return nil, err
	}

	sem, err := semaphore.New(r.semName, 1)
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

	sh := r.sh
	if r.sh == nil {
		sh, err = shm.Open(r.shmName)
		if err != nil {
			return nil, err
		}
		defer func() {
			err = sh.Close()
			if err != nil {
				secret = nil
			}
		}()
	}

	mybag := (*bag)(sh.Pointer())
	err = mybag.SetBoxSecret(secret, r.creds)
	if err != nil {
		return nil, err
	}
	return secret, nil
}

func (r *defring) GetBoxSecret() (secret *memguard.LockedBuffer, err error) {
	sem, err := semaphore.New(r.semName, 1)
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

	sh := r.sh
	if r.sh == nil {
		sh, err = shm.Open(r.shmName)
		if err != nil {
			return nil, err
		}
		defer func() {
			err = sh.Close()
			if err != nil {
				secret = nil
			}
		}()
	}

	mybag := (*bag)(sh.Pointer())
	secret, err = mybag.GetBoxSecret(r.creds)
	if err != nil {
		return nil, err
	}
	return secret, nil
}

func (r *defring) DeleteBoxSecret() error {
	return nil
}

func (r *defring) DeleteSignaturePubKey() error {
	return nil
}

const encSecretSize = 32 + 24 + secretbox.Overhead

type bag struct {
	sigpubkey [encSecretSize]byte
	boxsecret [encSecretSize]byte
}

func (b *bag) SetSigPubKey(m *memguard.LockedBuffer, creds RingCreds) error {
	enc, err := sbox.Encrypt(m.Buffer(), creds.Secret)
	if err != nil {
		return err
	}
	copy(b.sigpubkey[:], enc)
	return nil
}

func (b *bag) SetBoxSecret(m *memguard.LockedBuffer, creds RingCreds) error {
	enc, err := sbox.Encrypt(m.Buffer(), creds.Secret)
	if err != nil {
		return err
	}
	copy(b.boxsecret[:], enc)
	return nil
}

func (b *bag) GetSigPubKey(creds RingCreds) (key *memguard.LockedBuffer, err error) {
	dec, err := sbox.Decrypt(b.sigpubkey[:], creds.Secret)
	if err != nil {
		return nil, err
	}
	key, err = memguard.NewImmutableFromBytes(dec)
	if err != nil {
		return nil, err
	}
	return key, nil
}

func (b *bag) GetBoxSecret(creds RingCreds) (key *memguard.LockedBuffer, err error) {
	dec, err := sbox.Decrypt(b.boxsecret[:], creds.Secret)
	if err != nil {
		return nil, err
	}
	key, err = memguard.NewImmutableFromBytes(dec)
	if err != nil {
		return nil, err
	}
	return key, nil
}

var sizeOfBag uintptr

func init() {
	sizeOfBag = unsafe.Sizeof(bag{})
}
