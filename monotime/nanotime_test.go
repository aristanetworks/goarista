// Copyright (C) 2016  Arista Networks, Inc.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the COPYING file.

// Package monotime provides a fast monotonic clock source.
package monotime_test

import (
	"testing"
	"time"

	. "github.com/aristanetworks/goarista/monotime"
)

func TestNow(t *testing.T) {
	for i := 0; i < 100; i++ {
		t1 := Now()
		t2 := Now()
		// I honestly thought that we needed >= here, but in some environments
		// two consecutive calls can return the same value!
		if t1 > t2 {
			t.Fatalf("t1=%d should have been less than or equal to t2=%d", t1, t2)
		}
	}
}

func TestSince(t *testing.T) {
	for i := 0; i < 100; i++ {
		t1 := Now()
		time.Sleep(1)
		dur := Since(t1)
		if dur <= 0 {
			t.Fatalf("dur=%v should have been greater than 0", dur)
		}
		// avg value here is 5 nanoseconds but let's be safe.
		if dur >= 10*time.Millisecond {
			t.Fatalf("dur=%v was too large", dur)
		}
	}
}
