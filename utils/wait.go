package utils

import (
	"runtime"
	"time"
)

type ExpWait struct {
	nb uint
}

func (e *ExpWait) Wait() {
	if e.nb < 22 {
		runtime.Gosched()
	} else if e.nb < 24 {
		time.Sleep(1000000)
	} else if e.nb < 26 {
		time.Sleep(10000000)
	} else {
		time.Sleep(100000000)
	}
	e.nb++
}
