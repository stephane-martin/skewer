package accounting

// #include <sys/unistd.h>
// #include <sys/types.h>

// #include <linux/acct.h>
// #include <string.h>
// long cvt(comp_t c) {
//   return (c & 0x1fff) << (((c >> 13) & 0x7) * 3);
// }
import "C"
import (
	"os/user"
	"strconv"
	"strings"
	"time"
	"unsafe"

	"github.com/stephane-martin/skewer/sys"
)

const COMMLEN = C.ACCT_COMM

var Ssize int = C.sizeof_struct_acct_v3

func Tick() int64 {
	return sys.GetTick()
}

/*
struct acct_v3
{
	char		ac_flag;		Flags
	char		ac_version;		Always set to ACCT_VERSION
	__u16		ac_tty;			Control Terminal
	__u32		ac_exitcode;	Exitcode
	__u32		ac_uid;			Real User ID
	__u32		ac_gid;			Real Group ID
	__u32		ac_pid;			Process ID
	__u32		ac_ppid;		Parent Process ID
	__u32		ac_btime;		Process Creation Time
	float		ac_etime;		Elapsed Time
	comp_t		ac_utime;		User Time
	comp_t		ac_stime;		System Time
	comp_t		ac_mem;			Average Memory Usage
	comp_t		ac_io;			Chars Transferred
	comp_t		ac_rw;			Blocks Read or Written
	comp_t		ac_minflt;		Minor Pagefaults
	comp_t		ac_majflt;		Major Pagefaults
	comp_t		ac_swaps;		Number of Swaps
	char		ac_comm[ACCT_COMM];	Command Name
};
*/

type Status uint8

const (
	Fork   Status = C.AFORK
	Su     Status = C.ASU
	Compat Status = C.ACOMPAT
	Core   Status = C.ACORE
	Xsig   Status = C.AXSIG
)

type Acct struct {
	Comm     string
	Utime    time.Duration
	Stime    time.Duration
	Etime    time.Duration
	Btime    time.Time
	Uid      string
	Gid      string
	Mem      int64
	Io       int64
	Flags    Status
	ExitCode uint32
	Pid      uint32
	Ppid     uint32
}

func (a *Acct) Properties() (m map[string]string) {
	m = map[string]string{
		"comm":             a.Comm,
		"uid":              a.Uid,
		"gid":              a.Gid,
		"system_ns":        strconv.FormatInt(a.Stime.Nanoseconds(), 10),
		"elapsed_ns":       strconv.FormatInt(a.Etime.Nanoseconds(), 10),
		"user_ns":          strconv.FormatInt(a.Utime.Nanoseconds(), 10),
		"started_datetime": a.Btime.Format(time.RFC3339Nano),
		"memory_bytes":     strconv.FormatUint(uint64(a.Mem), 10),
		"io_bytes":         strconv.FormatInt(a.Io, 10),
		"flags":            a.Flags.String(),
		"pid_pid":          strconv.FormatUint(uint64(a.Pid), 10),
		"ppid_pid":         strconv.FormatUint(uint64(a.Ppid), 10),
		"exitcode":         strconv.FormatUint(uint64(a.ExitCode), 10),
	}
	return
}

func (s Status) String() string {
	allstatus := []string{}
	if s&Compat != 0 {
		allstatus = append(allstatus, "compatmode")
	}
	if s&Core != 0 {
		allstatus = append(allstatus, "dumpedcore")
	}
	if s&Fork != 0 {
		allstatus = append(allstatus, "forked")
	}
	if s&Su != 0 {
		allstatus = append(allstatus, "superuser")
	}
	if s&Xsig != 0 {
		allstatus = append(allstatus, "killedbysignal")
	}
	return strings.Join(allstatus, ",")
}

func Comp2Int(c C.comp_t) int64 {
	return int64(C.cvt(c))
}

func Comm(b *C.char) string {
	l := int(C.strnlen(b, COMMLEN))
	if l == 0 {
		return ""
	}
	temp := make([]byte, l+1)
	C.strncat((*C.char)(unsafe.Pointer(&temp[0])), b, C.size_t(l))
	return C.GoString((*C.char)(unsafe.Pointer(&temp[0])))
}

func MakeAcct(buf []byte, tick int64) (dest Acct) {
	p := (*C.struct_acct_v3)(unsafe.Pointer(&buf[0]))
	uid := strconv.FormatUint(uint64(p.ac_uid), 10)
	gid := strconv.FormatUint(uint64(p.ac_gid), 10)
	usr, err := user.LookupId(uid)
	username := uid
	if err == nil {
		username = usr.Username
	}
	grp, err := user.LookupGroupId(gid)
	groupname := gid
	if err == nil {
		groupname = grp.Name
	}
	dest = Acct{
		Comm:     Comm(&p.ac_comm[0]),
		Utime:    time.Duration(Comp2Int(p.ac_utime)*1000/tick) * time.Millisecond,
		Stime:    time.Duration(Comp2Int(p.ac_stime)*1000/tick) * time.Millisecond,
		Etime:    time.Duration(int64(float64(p.ac_etime)*1000)/tick) * time.Millisecond,
		Btime:    time.Unix(int64(p.ac_btime), 0).UTC(),
		Uid:      username,
		Gid:      groupname,
		Mem:      Comp2Int(p.ac_mem),
		Io:       Comp2Int(p.ac_io),
		Flags:    Status(p.ac_flag),
		ExitCode: uint32(p.ac_exitcode),
		Pid:      uint32(p.ac_pid),
		Ppid:     uint32(p.ac_ppid),
	}
	return
}
