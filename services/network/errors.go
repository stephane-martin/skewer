package network

import (
	"github.com/stephane-martin/skewer/utils/eerrors"
)

var ServerDefinitelyStopped = eerrors.WithTypes(eerrors.New("Server is definitely stopped"), "Stopped")
var ServerNotStopped = eerrors.WithTypes(eerrors.New("Server is not stopped"), "Stopped")
