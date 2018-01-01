// +build !openbsd

package scomp

import "github.com/stephane-martin/skewer/services/base"

var PledgeSupported bool = false

func SetupPledge(t base.Types) (err error) {
	return nil
}
