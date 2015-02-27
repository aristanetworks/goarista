// Copyright (c) 2015 Arista Networks, Inc.  All rights reserved.
// Arista Networks, Inc. Confidential and Proprietary.

package test_test // yes!

import (
	"testing"

	. "arista/test"
)

func TestDiff(t *testing.T) {
	testcases := append(getDeepEqualTests(t),
		[]deepEqualTestCase{{
			a:     map[int8]int(nil),
			b:     map[int8]int(nil),
			equal: true,
		}, {
			a:     map[int8]int16(nil),
			b:     map[int8]int(nil),
			equal: false,
			diff:  "types are different: map[int8]int16 vs map[int8]int",
		}, {
			a:     map[int8]int{int8(3): 2, int8(4): 6},
			b:     map[int8]int{int8(3): 2, int8(4): 6},
			equal: true,
		}, {
			a:     map[int8]int{int8(3): 2, int8(4): 5},
			b:     map[int8]int{int8(3): 2, int8(4): 6},
			equal: false,
			diff:  "for key int8(4) in map, values are different: Ints different: 5, 6",
		}, {
			a:     map[int8]int{int8(3): 2, int8(2): 6},
			b:     map[int8]int{int8(3): 2, int8(4): 6},
			equal: false,
			diff:  "key int8(2) in map is missing in the second map",
		}}...,
	)

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

func BenchmarkDeepEqual(b *testing.B) {
	for i := 0; i < b.N; i++ {
		DeepEqual(benchEqual, benchEqual)
	}
}

func BenchmarkDiff(b *testing.B) {
	for i := 0; i < b.N; i++ {
		Diff(benchEqual, benchEqual)
	}
}
