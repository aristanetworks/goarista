// Copyright (c) 2015 Arista Networks, Inc.  All rights reserved.
// Arista Networks, Inc. Confidential and Proprietary.

package test

import (
	"testing"
)

func TestShouldPanic(t *testing.T) {
	fn := func() { panic("Here we are") }

	ShouldPanic(t, fn)
}

func TestShouldPanicWithString(t *testing.T) {
	fn := func() { panic("Here we are") }

	ShouldPanicWith(t, "Here we are", fn)
}

func TestShouldPanicWithInt(t *testing.T) {
	fn := func() { panic(42) }

	ShouldPanicWith(t, 42, fn)
}

func TestShouldPanicWithStruct(t *testing.T) {
	fn := func() { panic(struct{ foo string }{foo: "panic"}) }

	ShouldPanicWith(t, struct{ foo string }{foo: "panic"}, fn)
}
