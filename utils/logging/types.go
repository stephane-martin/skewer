package logging

//go:generate msgp

import "time"

type Record struct {
	Time time.Time
	Lvl  int
	Msg  string
	Ctx  map[string]string
}
