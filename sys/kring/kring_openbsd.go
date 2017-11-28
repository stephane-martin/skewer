package kring

import (
	"io"

	"github.com/awnumar/memguard"
	"github.com/oklog/ulid"
)

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

func (r *ring) NewSignaturePubkey() (privkey *memguard.LockedBuffer, err error) {

}

func (r *ring) GetSignaturePubkey() (pubkey *memguard.LockedBuffer, err error) {

}

func (r *ring) NewBoxSecret() (secret *memguard.LockedBuffer, err error) {

}

func (r *ring) GetBoxSecret() (secret *memguard.LockedBuffer, err error) {

}

func (r *ring) DeleteBoxSecret() error {

}

func (r *ring) DeleteSignaturePubKey() error {

}
