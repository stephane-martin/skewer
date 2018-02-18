package utmpx

// #include <utmpx.h>
// #include <sys/unistd.h>
// #include <sys/types.h>
import "C"
import (
	"fmt"
	"time"
)

const (
	PathUTMPX = C._PATH_UTMPX
	PathWTMPX = C._PATH_UTMPX
	UserSize  = C._UTX_USERSIZE
	LineSize  = C._UTX_LINESIZE
	IDSize    = C._UTX_IDSIZE
	HostSize  = C._UTX_HOSTSIZE
)

type Type int

const (
	Empty        Type = C.EMPTY
	RunLvl            = C.RUN_LVL
	BootTime          = C.BOOT_TIME
	OldTime           = C.OLD_TIME
	NewTime           = C.NEW_TIME
	InitProcess       = C.INIT_PROCESS
	LoginProcess      = C.LOGIN_PROCESS
	UserProcess       = C.USER_PROCESS
	DeadProcess       = C.DEAD_PROCESS
	Accounting        = C.ACCOUNTING
	Signature         = C.SIGNATURE
	ShutdownTime      = C.SHUTDOWN_TIME
)

var Types = map[Type]string{
	Empty:        "empty",
	RunLvl:       "run level",
	BootTime:     "boot time",
	OldTime:      "old time",
	NewTime:      "new time",
	InitProcess:  "init process",
	LoginProcess: "login process",
	UserProcess:  "user process",
	DeadProcess:  "dead process",
	Accounting:   "accounting",
	Signature:    "signature",
	ShutdownTime: "shutdown time",
}

type Entry struct {
	User      string    `json:"user"`
	ID        string    `json:"id"`
	Line      string    `json:"line"`
	Host      string    `json:"host"`
	PID       int       `json:"pid"`
	Type      Type      `json:"type"`
	TypeStr   string    `json:"typestr"`
	Timestamp time.Time `json:"timestamp"`
}

func (e *Entry) String() string {
	return fmt.Sprintf(
		"User: '%s', ID: '%s', Line: '%s', Host: '%s', PID: '%d', Type: '%d %s', Timestamp: '%s'",
		e.User, e.ID, e.Line, e.Host, e.PID, e.Type, e.TypeStr, e.Timestamp,
	)
}

func next() (entry *Entry) {
	centry := C.getutxent()
	if centry == nil {
		return nil
	}
	entry = &Entry{
		User:      toStr(&centry.ut_user[0], UserSize),
		ID:        toStr(&centry.ut_id[0], IDSize),
		Line:      toStr(&centry.ut_line[0], LineSize),
		Host:      toStr(&centry.ut_host[0], HostSize),
		PID:       int(centry.ut_pid),
		Type:      Type(centry.ut_type),
		TypeStr:   Types[Type(centry.ut_type)],
		Timestamp: time.Unix(int64(centry.ut_tv.tv_sec), int64(centry.ut_tv.tv_usec)*1000),
	}
	return entry
}
