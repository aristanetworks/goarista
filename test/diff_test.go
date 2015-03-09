// Copyright (c) 2015 Arista Networks, Inc.  All rights reserved.
// Arista Networks, Inc. Confidential and Proprietary.

package test_test // yes!

import (
	"testing"

	. "arista/test"
)

func TestDiff(t *testing.T) {
	testcases := getDeepEqualTests(t)

	for _, test := range testcases {
		diff := Diff(test.a, test.b)
		if test.diff != diff {
			t.Errorf("Diff returned different diff\n"+
				"Diff    : %q\nExpected: %q\nFor %#v == %#v",
				diff, test.diff, test.a, test.b)
		}
	}
}

var benchEqual = map[string]interface{}{
	"foo": "bar",
	"bar": map[string]interface{}{
		"foo": "bar",
		"bar": map[string]interface{}{
			"foo": "bar",
		},
		"foo2": []uint32{1, 2, 5, 78, 23, 236, 346, 3456},
	},
}

var benchDiff = map[string]interface{}{
	"foo": "bar",
	"bar": map[string]interface{}{
		"foo": "bar",
		"bar": map[string]interface{}{
			"foo": "bar",
		},
		"foo2": []uint32{1, 2, 5, 78, 23, 236, 346, 3457},
	},
}

func BenchmarkEqualDeepEqual(b *testing.B) {
	for i := 0; i < b.N; i++ {
		DeepEqual(benchEqual, benchEqual)
	}
}

func BenchmarkEqualDiff(b *testing.B) {
	for i := 0; i < b.N; i++ {
		Diff(benchEqual, benchEqual)
	}
}

func BenchmarkDiffDeepEqual(b *testing.B) {
	for i := 0; i < b.N; i++ {
		DeepEqual(benchEqual, benchDiff)
	}
}

func BenchmarkDiffDiff(b *testing.B) {
	for i := 0; i < b.N; i++ {
		Diff(benchEqual, benchDiff)
	}
}
