// Copyright (c) 2023 Arista Networks, Inc.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the COPYING file.

package sliceutils

import (
	"testing"

	"github.com/aristanetworks/goarista/test"
)

func TestToAnySlice(t *testing.T) {
	in := []int{1, 2, 3}
	exp := []any{1, 2, 3}
	got := ToAnySlice(in)
	if d := test.Diff(got, exp); d != "" {
		t.Fatalf("expected: %v, got %v, diff: %s", exp, got, d)
	}

	in2 := []string{"a", "b", "c"}
	exp = []any{"a", "b", "c"}
	got = ToAnySlice(in2)
	if d := test.Diff(got, exp); d != "" {
		t.Fatalf("expected: %v, got %v, diff: %s", exp, got, d)
	}

}
