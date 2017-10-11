// +build !openbsd

package scomp

var PledgeSupported bool = false

func SetupPledge(name string) (err error) {
	return nil
}

