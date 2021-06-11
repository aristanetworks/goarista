// Copyright (c) 2015 Arista Networks, Inc.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the COPYING file.

package test

import (
	"errors"
	"testing"
)

func TestShouldPanic(t *testing.T) {
	ShouldPanic(t, func() { panic("Here we are") })

	ShouldPanicWith(t, "Here we are", func() { panic("Here we are") })
	ShouldPanicWith(t, 42, func() { panic(42) })
	ShouldPanicWith(t, struct{ foo string }{foo: "panic"},
		func() { panic(struct{ foo string }{foo: "panic"}) })

	ShouldPanicWithStr(t, "foo", func() { panic("foo") })
	ShouldPanicWithStr(t, "foo", func() { panic(errors.New("foo")) })
}
