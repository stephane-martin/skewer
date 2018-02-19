// +build linux

package scomp

import (
	seccomp "github.com/seccomp/libseccomp-golang"
)

// deriveComposeA composes functions f0 and f1 into one function, that takes the parameters from f0 and returns the results from f1.
func deriveComposeA(f0 func([]string, *seccomp.ScmpFilter) (*seccomp.ScmpFilter, error), f1 func(*seccomp.ScmpFilter) (*seccomp.ScmpFilter, error)) func([]string, *seccomp.ScmpFilter) (*seccomp.ScmpFilter, error) {
	return func(v_0_0 []string, v_0_1 *seccomp.ScmpFilter) (*seccomp.ScmpFilter, error) {
		v_1_0, err0 := f0(v_0_0, v_0_1)
		if err0 != nil {
			return nil, err0
		}
		v_2_0, err1 := f1(v_1_0)
		if err1 != nil {
			return nil, err1
		}
		return v_2_0, nil
	}
}

// deriveComposeB composes functions f0, f1 and f2 into one function, that takes the parameters from f0 and returns the results from f2.
func deriveComposeB(f0 func([]string, *seccomp.ScmpFilter) (*seccomp.ScmpFilter, error), f1 func(*seccomp.ScmpFilter) (*seccomp.ScmpFilter, error), f2 func(*seccomp.ScmpFilter) (*seccomp.ScmpFilter, error)) func([]string, *seccomp.ScmpFilter) (*seccomp.ScmpFilter, error) {
	return func(v_0_0 []string, v_0_1 *seccomp.ScmpFilter) (*seccomp.ScmpFilter, error) {
		v_1_0, err0 := f0(v_0_0, v_0_1)
		if err0 != nil {
			return nil, err0
		}
		v_2_0, err1 := f1(v_1_0)
		if err1 != nil {
			return nil, err1
		}
		v_3_0, err2 := f2(v_2_0)
		if err2 != nil {
			return nil, err2
		}
		return v_3_0, nil
	}
}

// deriveComposeC composes functions f0 and f1 into one function, that takes the parameters from f0 and returns the results from f1.
func deriveComposeC(f0 func() (*seccomp.ScmpFilter, error), f1 func(*seccomp.ScmpFilter) (*seccomp.ScmpFilter, error)) func() (*seccomp.ScmpFilter, error) {
	return func() (*seccomp.ScmpFilter, error) {
		v_1_0, err0 := f0()
		if err0 != nil {
			return nil, err0
		}
		v_2_0, err1 := f1(v_1_0)
		if err1 != nil {
			return nil, err1
		}
		return v_2_0, nil
	}
}
