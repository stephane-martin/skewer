package accounting

// #include <sys/unistd.h>
// #include <sys/types.h>
// #include <sys/acct.h>
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
)

/*
struct acct {
	char	  ac_comm[10]; command name
	comp_t	  ac_utime;    user time
	comp_t	  ac_stime;    system time
	comp_t	  ac_etime;    elapsed time
	u_int32_t ac_btime;    starting time
	uid_t	  ac_uid;      user id
	gid_t	  ac_gid;      group id
	u_int16_t ac_mem;      average memory usage
	comp_t	  ac_io;       count of IO blocks
	dev_t	  ac_tty;      controlling tty

#define	AFORK	0x01	   fork'd but not exec'd
#define	ASU	0x02           used super-user permissions
#define	ACOMPAT	0x04       used compatibility mode
#define	ACORE	0x08       dumped core
#define	AXSIG	0x10       killed by a signal
	u_int8_t  ac_flag;     accounting flags
};
*/

var Ahz int64 = C.AHZ
var Ssize int = C.sizeof_struct_acct

func Tick() int64 {
	return Ahz
}

type Status uint8

const (
	Fork   Status = C.AFORK
	Su     Status = C.ASU
	Compat Status = C.ACOMPAT
	Core   Status = C.ACORE
	Xsig   Status = C.AXSIG
)

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

type Acct struct {
	Comm  string
	Utime time.Duration
	Stime time.Duration
	Etime time.Duration
	Btime time.Time
	Uid   string
	Gid   string
	Mem   uint16
	Io    int64
	Flags Status
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
	}
	return
}

func Comp2Int(c C.comp_t) int64 {
	return int64(C.cvt(c))
}

func Comm(b *C.char) string {
	l := int(C.strnlen(b, 10))
	if l == 0 {
		return ""
	}
	temp := make([]byte, l+1)
	C.strlcpy((*C.char)(unsafe.Pointer(&temp[0])), b, C.size_t(l+1))
	return C.GoString((*C.char)(unsafe.Pointer(&temp[0])))
}

func MakeAcct(buf []byte, tick int64) (dest Acct) {
	p := (*C.struct_acct)(unsafe.Pointer(&buf[0]))
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
		Comm:  Comm(&p.ac_comm[0]),
		Utime: time.Duration(Comp2Int(p.ac_utime)*1000/tick) * time.Millisecond,
		Stime: time.Duration(Comp2Int(p.ac_stime)*1000/tick) * time.Millisecond,
		Etime: time.Duration(Comp2Int(p.ac_etime)*1000/tick) * time.Millisecond,
		Btime: time.Unix(int64(p.ac_btime), 0).UTC(),
		Uid:   username,
		Gid:   groupname,
		Mem:   uint16(p.ac_mem),
		Io:    Comp2Int(p.ac_io),
		Flags: Status(p.ac_flag),
	}
	return
}
