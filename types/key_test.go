// Copyright (c) 2015 Arista Networks, Inc.  All rights reserved.
// Arista Networks, Inc. Confidential and Proprietary.

package types_test

import (
	"testing"

	. "github.com/aristanetworks/goarista/types"
)

func TestKeyEqual(t *testing.T) {
	tests := []struct {
		a      Key
		b      Key
		result bool
	}{{
		a:      NewKey(3),
		b:      NewKey(3),
		result: true,
	}, {
		a:      NewKey(3),
		b:      NewKey(4),
		result: false,
	}, {
		a:      NewKey("foo"),
		b:      NewKey("foo"),
		result: true,
	}, {
		a:      NewKey("foo"),
		b:      NewKey("bar"),
		result: false,
	}, {
		a:      NewKey(&map[string]interface{}{}),
		b:      NewKey("bar"),
		result: false,
	}, {
		a:      NewKey(&map[string]interface{}{}),
		b:      NewKey(&map[string]interface{}{}),
		result: true,
	}, {
		a:      NewKey(&map[string]interface{}{"a": 3}),
		b:      NewKey(&map[string]interface{}{}),
		result: false,
	}, {
		a:      NewKey(&map[string]interface{}{"a": 3}),
		b:      NewKey(&map[string]interface{}{"b": 4}),
		result: false,
	}, {
		a:      NewKey(&map[string]interface{}{"a": 3}),
		b:      NewKey(&map[string]interface{}{"a": 4}),
		result: false,
	}, {
		a:      NewKey(&map[string]interface{}{"a": 3}),
		b:      NewKey(&map[string]interface{}{"a": 3}),
		result: true,
	}}

	for _, tcase := range tests {
		if tcase.a.Equal(tcase.b) != tcase.result {
			t.Errorf("Wrong result for case:\na: %#v\nb: %#v\nresult: %#v",
				tcase.a,
				tcase.b,
				tcase.result)
		}
	}
}
