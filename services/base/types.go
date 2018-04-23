package base

import (
	"strings"

	"github.com/stephane-martin/skewer/utils/eerrors"
)

type Types int

const (
	TCP Types = iota
	UDP
	RELP
	DirectRELP
	Journal
	Store
	Accounting
	KafkaSource
	Configuration
	Graylog
	Filesystem
	HTTPServer
	MacOS
)

var Names2Types = map[string]Types{
	"skewer-tcp":         TCP,
	"skewer-udp":         UDP,
	"skewer-relp":        RELP,
	"skewer-directrelp":  DirectRELP,
	"skewer-journal":     Journal,
	"skewer-store":       Store,
	"skewer-accounting":  Accounting,
	"skewer-kafkasource": KafkaSource,
	"skewer-conf":        Configuration,
	"skewer-graylog":     Graylog,
	"skewer-files":       Filesystem,
	"skewer-httpserver":  HTTPServer,
	"skewer-macos":       MacOS,
}

var ErrNotFound = eerrors.New("not found")

func Name(t Types, cfnd bool) (string, error) {
	if cfnd {
		if name, ok := Types2ConfinedNames[t]; ok {
			return name, nil
		}
		return "", ErrNotFound
	}
	if name, ok := Types2Names[t]; ok {
		return name, nil
	}
	return "", ErrNotFound
}

func Type(name string) (t Types, cfnd bool, err error) {
	name = strings.ToLower(strings.TrimSpace(name))
	if strings.HasPrefix(name, "confined-") {
		name = name[9:]
		cfnd = true
	}
	if t, ok := Names2Types[name]; ok {
		return t, cfnd, nil
	}
	return -1, false, ErrNotFound
}

var Types2Names map[Types]string
var Types2ConfinedNames map[Types]string

var Handles []ServiceHandle
var HandlesMap map[ServiceHandle]uintptr

type HandleType uint8

const (
	Binder HandleType = iota
	Logger
)

type ServiceHandle struct {
	Service string
	Type    HandleType
}

func init() {
	Types2Names = map[Types]string{}
	Types2ConfinedNames = map[Types]string{}
	for k, v := range Names2Types {
		Types2Names[v] = k
		Types2ConfinedNames[v] = "confined-" + k
	}

	Handles = []ServiceHandle{
		{"child", Binder},
		{Types2Names[TCP], Binder},
		{Types2Names[UDP], Binder},
		{Types2Names[RELP], Binder},
		{Types2Names[DirectRELP], Binder},
		{Types2Names[Store], Binder},
		{Types2Names[Graylog], Binder},
		{Types2Names[HTTPServer], Binder},
		{"child", Logger},
		{Types2Names[TCP], Logger},
		{Types2Names[UDP], Logger},
		{Types2Names[RELP], Logger},
		{Types2Names[DirectRELP], Logger},
		{Types2Names[Journal], Logger},
		{Types2Names[Configuration], Logger},
		{Types2Names[Store], Logger},
		{Types2Names[Accounting], Logger},
		{Types2Names[KafkaSource], Logger},
		{Types2Names[Graylog], Logger},
		{Types2Names[Filesystem], Logger},
		{Types2Names[HTTPServer], Logger},
		{Types2Names[MacOS], Logger},
	}

	HandlesMap = map[ServiceHandle]uintptr{}
	for i, h := range Handles {
		HandlesMap[h] = uintptr(i + 3)
	}
}

func LoggerHdl(typ Types) uintptr {
	return HandlesMap[ServiceHandle{Types2Names[typ], Logger}]
}

func BinderHdl(typ Types) uintptr {
	return HandlesMap[ServiceHandle{Types2Names[typ], Binder}]
}
