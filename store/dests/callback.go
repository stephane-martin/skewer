package dests

import (
	"github.com/oklog/ulid"
	"github.com/stephane-martin/skewer/conf"
)

type storeCallback func(uid ulid.ULID, dest conf.DestinationType)
