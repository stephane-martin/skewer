// +build linux

package sys

/*
#include <unistd.h>
#include <sys/types.h>
#include <pwd.h>
#include <stdlib.h>
*/
import "C"

func GetTick() int64 {
	return int64(C.sysconf(C._SC_CLK_TCK))
}
