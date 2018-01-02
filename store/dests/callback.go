package dests

import (
	"github.com/stephane-martin/skewer/conf"
	"github.com/stephane-martin/skewer/utils"
)

type storeCallback func(uid utils.MyULID, dest conf.DestinationType)
