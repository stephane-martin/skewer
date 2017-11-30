// +build openbsd freebsd netbsd

package kring

func GetRing(creds RingCreds) Ring {
	return GetDefRing(creds)
}

func NewRing() (r Ring, err error) {
	return NewDefRing()
}
