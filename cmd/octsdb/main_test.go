// Copyright (c) 2016 Arista Networks, Inc.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the COPYING file.

package main

import (
	"math"
	"testing"

	"github.com/aristanetworks/goarista/test"
	pb "github.com/openconfig/gnmi/proto/gnmi"
)

func TestParseValue(t *testing.T) { // Because parsing JSON sucks.
	var staticValueMap map[string]int64
	staticValueMap = make(map[string]int64)
	testcases := []struct {
		input          string
		staticValueMap map[string]int64
		expected       interface{}
	}{
		{"42", staticValueMap, []interface{}{int64(42)}},
		{"-42", staticValueMap, []interface{}{int64(-42)}},
		{"42.42", staticValueMap, []interface{}{float64(42.42)}},
		{"-42.42", staticValueMap, []interface{}{float64(-42.42)}},
		{`"foo"`, staticValueMap, []interface{}(nil)},
		{"9223372036854775807", staticValueMap, []interface{}{int64(math.MaxInt64)}},
		{"-9223372036854775808", staticValueMap, []interface{}{int64(math.MinInt64)}},
		{"9223372036854775808", staticValueMap, []interface{}{uint64(math.MaxInt64) + 1}},
		{"[1,3,5,7,9]", staticValueMap,
			[]interface{}{int64(1), int64(3), int64(5), int64(7), int64(9)}},
		{"[1,9223372036854775808,0,-9223372036854775808]", staticValueMap, []interface{}{
			int64(1),
			uint64(math.MaxInt64) + 1,
			int64(0),
			int64(math.MinInt64)},
		},
		{`[1,{"value":9},5,7,9]`, staticValueMap,
			[]interface{}{int64(1), int64(9), int64(5), int64(7), int64(9)}},
		{`"intfOperUp"`, map[string]int64{"intfOperUp": 1}, []interface{}{int64(1)}},
		{`"default"`, map[string]int64{"default": 0}, []interface{}{int64(0)}},
	}
	for i, tcase := range testcases {
		actual := parseValue(&pb.Update{
			Val: &pb.TypedValue{
				Value: &pb.TypedValue_JsonVal{JsonVal: []byte(tcase.input)},
			},
		}, tcase.staticValueMap)
		if d := test.Diff(tcase.expected, actual); d != "" {
			t.Errorf("#%d: %s: %#v vs %#v", i, d, tcase.expected, actual)
		}
	}
}
