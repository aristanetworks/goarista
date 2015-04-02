// Copyright (c) 2014 Arista Networks, Inc.  All rights reserved.
// Arista Networks, Inc. Confidential and Proprietary.

package test

import (
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
		if actual := DeepEqual(test.a, test.b); actual != test.equal {
			t.Errorf("DeepEqual returned %v but we wanted %v for %#v == %#v",
				actual, test.equal, test.a, test.b)
		}
	}
}
