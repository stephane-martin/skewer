package journald

import "github.com/stephane-martin/skewer/utils/queue"

type JournaldReader interface {
	Start(coding string)
	Stop()
	Shutdown()
	Entries() *queue.MessageQueue
}
