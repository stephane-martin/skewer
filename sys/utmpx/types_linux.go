package utmpx

// #include <utmp.h>
// #include <utmpx.h>
// #include <sys/unistd.h>
// #include <sys/types.h>
import "C"
import (
	"fmt"
	"time"
)

const (
	NameSize = C.UT_NAMESIZE
	LineSize = C.UT_LINESIZE
	IDSize   = 4
	HostSize = C.UT_HOSTSIZE
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
}

type Entry struct {
	Type      Type      `json:"type"`
	TypeStr   string    `json:"typestr"`
	PID       int       `json:"pid"`
	Line      string    `json:"line"`
	ID        string    `json:"id"`
	User      string    `json:"user"`
	Host      string    `json:"host"`
	SessionID int       `json:"session_id"`
	Timestamp time.Time `json:"timestamp"`
}

func (e *Entry) String() string {
	return fmt.Sprintf(
		"Type: '%d %s', PID: '%d', Line: '%s', ID: '%s', User: '%s', Host: '%s', Session: '%d', Timestamp: '%s'",
		e.Type, e.TypeStr, e.PID, e.Line, e.ID, e.User, e.Host, e.SessionID, e.Timestamp,
	)
}

func next() (entry *Entry) {
	centry := C.getutxent()
	if centry == nil {
		return nil
	}
	entry = &Entry{
		Type:      Type(centry.ut_type),
		TypeStr:   Types[Type(centry.ut_type)],
		PID:       int(centry.ut_pid),
		Line:      toStr(&centry.ut_line[0], LineSize),
		ID:        toStr(&centry.ut_id[0], IDSize),
		User:      toStr(&centry.ut_user[0], NameSize),
		Host:      toStr(&centry.ut_host[0], HostSize),
		SessionID: int(centry.ut_session),
		Timestamp: time.Unix(int64(centry.ut_tv.tv_sec), int64(centry.ut_tv.tv_usec)*1000),
	}
	return entry
}
