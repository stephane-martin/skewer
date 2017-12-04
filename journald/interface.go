package journald

import "github.com/stephane-martin/skewer/utils/queue"

type JournaldReader interface {
	Start()
	Stop()
	Shutdown()
	Entries() *queue.MessageQueue
}
