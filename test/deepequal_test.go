// Copyright (C) 2014  Arista Networks, Inc.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the COPYING file.

package test

import (
	"reflect"
	"testing"
)

type comparableStruct struct {
	a uint32
	t *testing.T
}

func (c comparableStruct) Equal(v interface{}) bool {
	other, ok := v.(comparableStruct)
	// Deliberately ignore t.
	return ok && c.a == other.a
}

func TestDeepEqual(t *testing.T) {
	testcases := getDeepEqualTests(t)
	for _, test := range testcases {
		equal := len(test.diff) == 0
		if actual := DeepEqual(test.a, test.b); actual != equal {
			t.Errorf("DeepEqual returned %t but we wanted %t for %#v == %#v",
				actual, equal, test.a, test.b)
		}
	}
}

func TestForcePublic(t *testing.T) {
	c := comparableStruct{a: 42}
	v := reflect.ValueOf(c)
	// Without the call to forceExport(), the following line would crash with
	// "panic: reflect.Value.Interface: cannot return value obtained from
	// unexported field or method".
	a := forceExport(v.FieldByName("a")).Interface()
	if i, ok := a.(uint32); !ok {
		t.Fatalf("Should have gotten a uint32 but got a %T", a)
	} else if i != 42 {
		t.Fatalf("Should have gotten 42 but got a %d", i)
	}
}
