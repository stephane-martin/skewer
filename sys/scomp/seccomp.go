// +build linux

package scomp

//go:generate goderive .

import (
	"fmt"
	"syscall"

	seccomp "github.com/seccomp/libseccomp-golang"
	"github.com/stephane-martin/skewer/services/base"
	"github.com/stephane-martin/skewer/utils"
)

var action seccomp.ScmpAction = seccomp.ActTrap

//var action seccomp.ScmpAction = seccomp.ActKill

var socketDomains []int = []int{syscall.AF_LOCAL, syscall.AF_UNIX, syscall.AF_INET, syscall.AF_INET6, syscall.AF_NETLINK, syscall.AF_PACKET}
var socketTypes []int = []int{syscall.SOCK_STREAM, syscall.SOCK_DGRAM, syscall.SOCK_SEQPACKET}
var socketOpts []int = []int{syscall.SOCK_NONBLOCK, syscall.SOCK_CLOEXEC, 0, syscall.SOCK_NONBLOCK | syscall.SOCK_CLOEXEC}

var hostnameAllowed []string = []string{
	"sethostname",
	"setdomainname",
}

var mountAllowed []string = []string{
	"mount",
	"umount",
	"umount2",
}

var changeUidAllowed []string = []string{
	"setuid",
	"setgid",
	"setpgid",
	"setsid",
	"setreuid",
	"setregid",
	"setgroups",
	"setresuid",
	"setresgid",
	"setfsuid",
	"setfsgid",
	"capset",
}

var signalsAllowed []string = []string{
	"signalfd",
	"signalfd4",
	"sigaction",
	"rt_sigaction",
	//"rt_sigprocmask",
	//"sigaltstack",
}

var forkAllowed []string = []string{
	"vfork",
	"execve",
	"execveat",
	"prctl",
	"arch_prctl",
	//"prlimit",
	"prlimit64",
	"setrlimit",
	"acct",
	"set_tid_address",
}

var listenNetworkAllowed []string = []string{
	"accept",
	"accept4",
	"bind",
	"listen",
}

var baseAllowed []string = []string{
	"fork",
	"clone",
	"unshare",
	"setns",
	"chroot",
	"ioctl",
	"pivot_root",
	"seccomp",
	"brk",
	//"sbrk",
	"read",
	"write",
	"open",
	"close",
	"stat",
	"fstat",
	"lstat",
	"poll",
	"lseek",
	"mmap",
	"munmap",
	"mprotect",
	"pread64",
	"pwrite64",
	"readv",
	"writev",
	"access",
	"pipe",
	"select",
	"sched_yield",
	"mremap",
	"msync",
	"mincore",
	"madvise",
	"shmget",
	"shmat",
	"shmctl",
	"dup",
	"dup2",
	"pause",
	"nanosleep",
	"getitimer",
	"alarm",
	"setitimer",
	"getpid",
	"sendfile",
	//"socket",
	"connect",
	//"accept",
	"recvfrom",
	"sendto",
	"recvmsg",
	"sendmsg",
	//"bind",
	//"listen",
	"getsockname",
	"getpeername",
	//"socketpair",
	"getsockopt",
	"setsockopt",
	"exit",
	"wait4",
	"kill",
	"uname",
	"semget",
	"semop",
	"semctl",
	"shmdt",
	"msgget",
	"msgsnd",
	"msgrcv",
	"msgctl",
	"fcntl",
	"flock",
	"fsync",
	"fdatasync",
	"truncate",
	"ftruncate",
	"getdents",
	"getcwd",
	"chdir",
	"fchdir",
	"rename",
	"mkdir",
	"rmdir",
	"creat",
	"link",
	"unlink",
	"symlink",
	"readlink",
	"chmod",
	"fchmod",
	"chown",
	"fchown",
	"lchown",
	"umask",
	"gettimeofday",
	"getrlimit",
	"getrusage",
	"sysinfo",
	"times",
	//"ptrace",
	"getuid",
	"syslog",
	"getgid",
	//"setuid",
	//"setgid",
	"geteuid",
	"getegid",
	//"setpgid",
	"getpgid",
	"getpgrp",
	//"setsid",
	//"setreuid",
	//"setregid",
	"getgroups",
	//"setgroups",
	//"setresuid",
	"getresuid",
	//"setresgid",
	"getresgid",
	"getpgid",
	//"setfsuid",
	//"setfsgid",
	"getsid",
	"capget",
	//"capset",
	"rt_sigpending",
	"rt_sigtimedwait",
	"rt_sigqueueinfo",
	"rt_sigsuspend",
	"rt_sigreturn",
	"rt_sigprocmask",
	"sigaltstack",
	"utime",
	//"mknod",
	//"uselib",
	//"personality",
	"ustat",
	"statfs",
	"fstatfs",
	"sysfs",
	"getpriority",
	"setpriority",
	"sched_setparam",
	"sched_getparam",
	"sched_setscheduler",
	"sched_getscheduler",
	"sched_get_priority_max",
	"sched_get_priority_min",
	"sched_rr_get_interval",
	"mlock",
	//"mlock2",
	"munlock",
	"mlockall",
	"munlockall",
	//"vhangup",
	//"modify_ldt",
	//"_sysctl",
	//"prctl",
	//"arch_prctl",
	//"adjtimex",
	//"setrlimit",
	"sync",
	//"acct",
	//"settimeofday",
	//"mount",
	//"umount2",
	//"swapon",
	//"swapoff",
	//"reboot",
	//"sethostname",
	//"setdomainname",
	//"iopl",
	//"ioperm",
	//"create_module",
	//"init_module",
	//"delete_module",
	//"get_kernel_syms",
	//"query_module",
	//"quotactl",
	//"nfsservctl",
	//"getpmsg",
	//"putpmsg",
	//"afs_syscall",
	//"tuxcall",
	//"security",
	"gettid",
	"readahead",
	//"setxattr",
	//"lsetxattr",
	//"fsetxattr",
	"getxattr",
	"lgetxattr",
	"fgetxattr",
	"listxattr",
	"flistxattr",
	"llistxattr",
	//"removexattr",
	//"lremovexattr",
	//"fremovexattr",
	"tkill",
	"time",
	"futex",
	"sched_setaffinity",
	"sched_getaffinity",
	"set_thread_area",
	"get_thread_area",
	//"io_setup",
	//"io_destroy",
	//"io_getevents",
	//"io_submit",
	//"io_submit",
	//"lookup_dcookie",
	"epoll_create",
	"epoll_ctl",
	"epoll_wait",
	"getdents64",
	//"set_tid_address",
	//"restart_syscall",
	"semtimedop",
	"timer_create",
	"timer_settime",
	"timer_gettime",
	"timer_getoverrun",
	"timer_delete",
	"clock_gettime",
	//"clock_settime",
	//"stime",
	"clock_getres",
	"clock_nanosleep",
	"exit_group",
	"epoll_wait",
	"epoll_ctl",
	"tgkill",
	"utimes",
	//"vserver",
	//"mbind",
	//"set_mempolicy",
	//"get_mempolicy",
	"mq_open",
	"mq_unlink",
	"mq_timedsend",
	"mq_timedreceive",
	"mq_notify",
	"mq_getsetattr",
	//"kexec_load",
	"waitid",
	"add_key",
	"request_key",
	"keyctl",
	"ioprio_set",
	"ioprio_get",
	"inotify_init",
	"inotify_add_watch",
	"inotify_rm_watch",
	//"migrate_pages",
	"openat",
	"mkdirat",
	//"mknodat",
	"fchownat",
	"futimesat",
	"newfstatat",
	"unlinkat",
	"renameat",
	"linkat",
	"symlinkat",
	"readlinkat",
	"fchmodat",
	"faccessat",
	"pselect6",
	"ppoll",
	//"unshare",
	"set_robust_list",
	"get_robust_list",
	"splice",
	"tee",
	"sync_file_range",
	"vmsplice",
	//"move_pages",
	"utimensat",
	"epoll_pwait",
	//"signalfd",
	"timerfd_create",
	"eventfd",
	"fallocate",
	"timerfd_settime",
	"timerfd_gettime",
	//"accept4",
	//"signalfd4",
	"eventfd2",
	"epoll_create1",
	"dup3",
	"pipe2",
	"inotify_init1",
	"preadv",
	"pwritev",
	"rt_tgsigqueueinfo",
	//"perf_event_open",
	"recvmmsg",
	"fanotify_init",
	"fanotify_mark",
	//"prlimit64",
	//"name_to_handle_at",
	//"open_by_handle_at",
	//"clock_adjtime",
	"syncfs",
	"sendmmsg",
	//"setns",
	"getcpu",
	//"process_vm_readv",
	//"process_vm_writev",
	//"kcmp",
	//"finit_module",
	"sched_setattr",
	"sched_getattr",
	"renameat2",
	"getrandom",
}

func buildSimpleFilter(alloweds []string, filter *seccomp.ScmpFilter) (*seccomp.ScmpFilter, error) {
	if filter == nil {
		filter, _ = seccomp.NewFilter(action)
	}
	if alloweds == nil {
		return filter, nil
	}
	for _, allowed := range alloweds {
		call, err := seccomp.GetSyscallFromName(allowed)
		if err != nil {
			return nil, err
		}
		err = filter.AddRule(call, seccomp.ActAllow)
		if err != nil {
			return nil, err
		}
	}
	return filter, nil
}

func socketFilter(filter *seccomp.ScmpFilter) (*seccomp.ScmpFilter, error) {
	var err error

	socketCall, _ := seccomp.GetSyscallFromName("socket")
	socketPairCall, _ := seccomp.GetSyscallFromName("socketpair")
	for _, domain := range socketDomains {
		for _, typ := range socketTypes {
			for _, opt := range socketOpts {
				cond1, err1 := seccomp.MakeCondition(0, seccomp.CompareEqual, uint64(domain))
				cond2, err2 := seccomp.MakeCondition(1, seccomp.CompareEqual, uint64(typ|opt))
				if err1 == nil && err2 == nil {
					err = filter.AddRuleConditional(socketCall, seccomp.ActAllow, []seccomp.ScmpCondition{cond1, cond2})
					if err != nil {
						return nil, err
					}
					err = filter.AddRuleConditional(socketPairCall, seccomp.ActAllow, []seccomp.ScmpCondition{cond1, cond2})
					if err != nil {
						return nil, err
					}
				} else if err1 != nil {
					return nil, err1
				} else {
					return nil, err2
				}
			}
		}
	}

	return filter, nil
}

func parentFilter() (*seccomp.ScmpFilter, error) {
	filter, _ := seccomp.NewFilter(action)

	addSimple := func(allowed []string) func() error {
		return func() error {
			_, err := buildSimpleFilter(allowed, filter)
			return err
		}
	}

	addSocket := func() error {
		_, err := socketFilter(filter)
		return err
	}

	err := utils.Chain(
		addSimple(baseAllowed),
		addSimple(hostnameAllowed),
		addSimple(changeUidAllowed),
		addSimple(mountAllowed),
		addSimple(signalsAllowed),
		addSimple(listenNetworkAllowed),
		addSimple(forkAllowed),
		addSocket,
	)
	if err == nil {
		return filter, nil
	}
	return nil, err
}

func applyFilter(filter *seccomp.ScmpFilter) (*seccomp.ScmpFilter, error) {
	err := filter.SetNoNewPrivsBit(true)

	if err != nil {
		return nil, err
	}

	if !filter.IsValid() {
		return nil, fmt.Errorf("filter is not valid")
	}

	err = filter.Load()
	if err != nil {
		return nil, err
	}
	return filter, nil
}

func SetupSeccomp(t base.Types) (err error) {
	switch t {

	case base.TCP, base.UDP, base.RELP, base.Graylog, base.Journal, base.Filesystem:
		_, err = deriveComposeA(buildSimpleFilter, applyFilter)(baseAllowed, nil)

	case base.DirectRELP, base.Store, base.KafkaSource, base.Configuration:
		_, err = deriveComposeB(buildSimpleFilter, socketFilter, applyFilter)(baseAllowed, nil)

	default:
		_, err = deriveComposeC(parentFilter, applyFilter)()
		//applyFilter(parentFilter())
	}
	return err
}
