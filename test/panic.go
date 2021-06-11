// Copyright (c) 2015 Arista Networks, Inc.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the COPYING file.

package test

import (
	"fmt"
	"runtime"
	"testing"
)

// ShouldPanic will test is a function is panicking
func ShouldPanic(t *testing.T, fn func()) {
	t.Helper()
	defer func() {
		t.Helper()
		if r := recover(); r == nil {
			t.Errorf("%sThe function %p should have panicked",
				getCallerInfo(), fn)
		}
	}()

	fn()
}

// ShouldPanicWith will test is a function is panicking with a specific message
func ShouldPanicWith(t *testing.T, msg interface{}, fn func()) {
	t.Helper()
	defer func() {
		t.Helper()
		if r := recover(); r == nil {
			t.Errorf("%sThe function %p should have panicked with %#v",
				getCallerInfo(), fn, msg)
		} else if d := Diff(msg, r); len(d) != 0 {
			t.Errorf("%sThe function %p panicked with the wrong message.\n"+
				"Expected: %#v\nReceived: %#v\nDiff:%s",
				getCallerInfo(), fn, msg, r, d)
		}
	}()

	fn()
}

// ShouldPanicWithStr a function panics with a specific string.
// If the function panics with an error, the error's message is compared to the string.
func ShouldPanicWithStr(t *testing.T, msg string, fn func()) {
	t.Helper()
	defer func() {
		t.Helper()
		r := recover()
		if r == nil {
			t.Errorf("%sThe function %p should have panicked with %#v",
				getCallerInfo(), fn, msg)
			return
		}
		gotStr, ok := r.(string)
		if !ok {
			gotErr, ok := r.(error)
			if !ok {
				t.Errorf("%sThe function paniced with not string/error: %#v", getCallerInfo(), r)
			}
			gotStr = gotErr.Error()
		}
		if d := Diff(msg, gotStr); len(d) != 0 {
			t.Errorf("%sThe function %p panicked with the wrong message.\n"+
				"Expected: %#v\nReceived: %#v\nDiff:%s",
				getCallerInfo(), fn, msg, gotStr, d)
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
