package sys

import (
	"os/user"
	"strconv"
	"strings"
)

func LookupUid(uid string, gid string) (int, int, error) {

	curuser, err := user.Current()
	if err != nil {
		return 0, 0, err
	}

	uid = strings.TrimSpace(uid)
	gid = strings.TrimSpace(gid)

	if len(uid) == 0 {
		uid = curuser.Username
	}
	if len(gid) == 0 {
		gid = curuser.Gid
	}

	var u *user.User
	var g *user.Group

	_, err = strconv.Atoi(uid)
	if err == nil {
		u, err = user.LookupId(uid)
		if err != nil {
			return 0, 0, err
		}
	} else {
		u, err = user.Lookup(uid)
		if err != nil {
			return 0, 0, err
		}
	}

	_, err = strconv.Atoi(gid)
	if err == nil {
		g, err = user.LookupGroupId(gid)
		if err != nil {
			return 0, 0, err
		}
	} else {
		g, err = user.LookupGroup(gid)
		if err != nil {
			return 0, 0, err
		}
	}
	numuid, err := strconv.Atoi(u.Uid)
	if err != nil {
		return 0, 0, err
	}
	numgid, err := strconv.Atoi(g.Gid)
	if err != nil {
		return 0, 0, err
	}
	return numuid, numgid, nil

}
