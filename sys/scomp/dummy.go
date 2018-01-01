// +build !linux

package scomp

import "github.com/stephane-martin/skewer/services/base"

func SetupSeccomp(t base.Types) (err error) {
	return nil
}
