// Copyright (c) 2015 Arista Networks, Inc.  All rights reserved.
// Arista Networks, Inc. Confidential and Proprietary.

package test

import (
	"fmt"
	"runtime"
	"testing"
)

// ShouldPanic will test is a function is panicking
func ShouldPanic(t *testing.T, fn func()) {

	defer func() {
		if r := recover(); r == nil {
			t.Errorf("%sThe function %#v should have panicked",
				getCallerInfo(),
				fn)
		}
	}()

	fn()
}

// ShouldPanicWith will test is a function is panicking with a specific message
func ShouldPanicWith(t *testing.T, msg interface{}, fn func()) {

	defer func() {
		if r := recover(); r == nil {
			t.Errorf("%sThe function %#v should have panicked",
				getCallerInfo(),
				fn)
		} else if d := Diff(msg, r); len(d) != 0 {
			t.Errorf("%sThe function %#v panicked with the wrong message.\n"+
				"Expected: %#v\nReceived: %#v\nDiff:%s",
				getCallerInfo(),
				fn, msg, r, d)
		}
	}()

	fn()
}

func getCallerInfo() string {
	_, file, line, ok := runtime.Caller(4)
	if !ok {
		return ""
	}
	return fmt.Sprintf("%s:%d\n", file, line)
}
