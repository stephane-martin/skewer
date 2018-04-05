package journald

import "github.com/stephane-martin/skewer/utils"

type JournaldReader interface {
	Start(utils.MyULID)
	Stop()
	Shutdown()
}
