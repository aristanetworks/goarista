// Copyright (c) 2015 Arista Networks, Inc.  All rights reserved.
// Arista Networks, Inc. Confidential and Proprietary.

package test_test

import (
	"testing"
)

type builtinCompare struct {
	a uint32
	b string
}

type deepEqualTestCase struct {
	a, b  interface{}
	equal bool
	diff  string
}

var deepEqualNullMapString map[string]interface{}

func getDeepEqualTests(t *testing.T) []deepEqualTestCase {
	return []deepEqualTestCase{
		{
			a:     nil,
			b:     nil,
			equal: true,
		}, {
			a:     uint8(5),
			b:     uint8(5),
			equal: true,
		}, {
			a:     nil,
			b:     uint8(5),
			equal: false,
			diff:  "one value is nil and the other is of type: uint8",
		}, {
			a:     uint8(5),
			b:     nil,
			equal: false,
			diff:  "one value is nil and the other is of type: uint8",
		}, {
			a:     uint16(1),
			b:     uint16(2),
			equal: false,
			diff:  "Uints different: 1, 2",
		}, {
			a:     int8(1),
			b:     int16(1),
			equal: false,
			diff:  "types are different: int8 vs int16",
		}, {
			a:     true,
			b:     true,
			equal: true,
		}, {
			a:     float32(3.1415),
			b:     float32(3.1415),
			equal: true,
		}, {
			a:     float32(3.1415),
			b:     float32(3.1416),
			equal: false,
			diff:  "Floats different: 3.1415, 3.1416",
		}, {
			a:     float64(3.14159265),
			b:     float64(3.14159265),
			equal: true,
		}, {
			a:     float64(3.14159265),
			b:     float64(3.14159266),
			equal: false,
			diff:  "Floats different: 3.14159265, 3.14159266",
		}, {
			a:     deepEqualNullMapString,
			b:     deepEqualNullMapString,
			equal: true,
		}, {
			a:     &deepEqualNullMapString,
			b:     &deepEqualNullMapString,
			equal: true,
		}, {
			a:     deepEqualNullMapString,
			b:     &deepEqualNullMapString,
			equal: false,
			diff:  "types are different: map[string]interface {} vs *map[string]interface {}",
		}, {
			a:     &deepEqualNullMapString,
			b:     deepEqualNullMapString,
			equal: false,
			diff:  "types are different: *map[string]interface {} vs map[string]interface {}",
		}, {
			a:     map[string]interface{}{"a": uint32(42)},
			b:     map[string]interface{}{"a": uint32(42)},
			equal: true,
		}, {
			a:     map[string]interface{}{"a": int32(42)},
			b:     map[string]interface{}{"a": int32(51)},
			equal: false,
			diff:  "for key \"a\" in map, values are different: Ints different: 42, 51",
		}, {
			a:     map[string]interface{}{"a": uint32(42)},
			b:     map[string]interface{}{},
			equal: false,
			diff:  "Maps have different size: 1 != 0",
		}, {
			a:     map[string]interface{}{},
			b:     map[string]interface{}{"a": uint32(42)},
			equal: false,
			diff:  "Maps have different size: 0 != 1",
		}, {
			a:     map[string]interface{}{"a": uint64(42), "b": "extra"},
			b:     map[string]interface{}{"a": uint64(42)},
			equal: false,
			diff:  "Maps have different size: 2 != 1",
		}, {
			a:     map[string]interface{}{"a": uint64(42)},
			b:     map[string]interface{}{"a": uint64(42), "b": "extra"},
			equal: false,
			diff:  "Maps have different size: 1 != 2",
		}, {
			a:     map[uint32]interface{}{uint32(42): "foo"},
			b:     map[uint32]interface{}{uint32(42): "foo"},
			equal: true,
		}, {
			a:     map[uint32]interface{}{uint32(42): "foo"},
			b:     map[uint32]interface{}{uint32(51): "foo"},
			equal: false,
			diff:  "key uint32(42) in map is missing in the second map",
		}, {
			a:     map[uint32]interface{}{uint32(42): "foo"},
			b:     map[uint32]interface{}{uint32(42): "foo", uint32(51): "bar"},
			equal: false,
			diff:  "Maps have different size: 1 != 2",
		}, {
			a:     map[uint32]interface{}{uint32(42): "foo"},
			b:     map[uint64]interface{}{uint64(42): "foo"},
			equal: false,
			diff:  "types are different: map[uint32]interface {} vs map[uint64]interface {}",
		}, {
			a:     map[uint64]interface{}{uint64(42): "foo"},
			b:     map[uint64]interface{}{uint64(42): "foo"},
			equal: true,
		}, {
			a:     map[uint64]interface{}{uint64(42): "foo"},
			b:     map[uint64]interface{}{uint64(51): "foo"},
			equal: false,
			diff:  "key uint64(42) in map is missing in the second map",
		}, {
			a:     map[uint64]interface{}{uint64(42): "foo"},
			b:     map[uint64]interface{}{uint64(42): "foo", uint64(51): "bar"},
			equal: false,
			diff:  "Maps have different size: 1 != 2",
		}, {
			a:     map[uint64]interface{}{uint64(42): "foo"},
			b:     map[interface{}]interface{}{uint32(42): "foo"},
			equal: false,
			diff:  "types are different: map[uint64]interface {} vs map[interface {}]interface {}",
		}, {
			a:     map[interface{}]interface{}{"a": uint32(42)},
			b:     map[string]interface{}{"a": uint32(42)},
			equal: false,
			diff:  "types are different: map[interface {}]interface {} vs map[string]interface {}",
		}, {
			a:     map[interface{}]interface{}{},
			b:     map[interface{}]interface{}{},
			equal: true,
		}, {
			a:     &map[interface{}]interface{}{},
			b:     &map[interface{}]interface{}{},
			equal: true,
		}, {
			a:     map[interface{}]interface{}{&map[string]interface{}{"a": "foo", "b": int16(8)}: "foo"},
			b:     map[interface{}]interface{}{&map[string]interface{}{"a": "foo", "b": int16(8)}: "foo"},
			equal: true,
		}, {
			a:     map[interface{}]interface{}{&map[string]interface{}{"a": "foo", "b": uint32(8)}: "foo"},
			b:     map[interface{}]interface{}{&map[string]interface{}{"a": "foo", "b": uint32(8)}: "fox"},
			equal: false,
			diff:  "for key *map[string]interface {}{\"a\":\"foo\", \"b\":uint32(8)} in map, values are different: Strings different: \"foo\" vs \"fox\"",
		}, {
			a:     map[interface{}]interface{}{&map[string]interface{}{"a": "foo", "b": uint32(8)}: "foo"},
			b:     map[interface{}]interface{}{&map[string]interface{}{"a": "foo", "b": uint32(5)}: "foo"},
			equal: false,
			diff:  "key *map[string]interface {}{\"a\":\"foo\", \"b\":uint32(8)} in map is missing in the second map",
		}, {
			a:     map[interface{}]interface{}{&map[string]interface{}{"a": "foo", "b": uint32(8)}: "foo"},
			b:     map[interface{}]interface{}{&map[string]interface{}{"a": "foo"}: "foo"},
			equal: false,
			diff:  "key *map[string]interface {}{\"a\":\"foo\", \"b\":uint32(8)} in map is missing in the second map",
		}, {
			a: map[interface{}]interface{}{
				&map[string]interface{}{"a": "foo", "b": int16(8)}: "foo",
				&map[string]interface{}{"a": "foo", "b": int8(8)}:  "foo",
			},
			b: map[interface{}]interface{}{
				&map[string]interface{}{"a": "foo", "b": int16(8)}: "foo",
				&map[string]interface{}{"a": "foo", "b": int8(8)}:  "foo",
			},
			equal: true,
		}, {
			a: map[interface{}]interface{}{
				&map[string]interface{}{"a": "foo", "b": int16(8)}: "foo",
				&map[string]interface{}{"a": "foo", "b": int8(8)}:  "foo",
			},
			b: map[interface{}]interface{}{
				&map[string]interface{}{"a": "foo", "b": int16(8)}: "foo",
				&map[string]interface{}{"a": "foo", "b": int8(5)}:  "foo",
			},
			equal: false,
			diff:  "key *map[string]interface {}{\"a\":\"foo\", \"b\":int8(8)} in map is missing in the second map",
		}, {
			a: map[interface{}]interface{}{
				&map[string]interface{}{"a": "foo", "b": int16(8)}: "foo",
				&map[string]interface{}{"a": "foo", "b": int8(8)}:  "foo",
			},
			b: map[interface{}]interface{}{
				&map[string]interface{}{"a": "foo", "b": int16(8)}: "foo",
				&map[string]interface{}{"a": "foo", "b": int32(8)}: "foo",
			},
			equal: false,
			diff:  "key *map[string]interface {}{\"a\":\"foo\", \"b\":int8(8)} in map is missing in the second map",
		}, {
			a: map[interface{}]interface{}{
				&map[string]interface{}{"a": "foo", "b": int16(8)}: "foo",
				&map[string]interface{}{"a": "foo", "b": int8(8)}:  "foo",
			},
			b: map[interface{}]interface{}{
				&map[string]interface{}{"a": "foo", "b": int16(8)}: "foo",
			},
			equal: false,
			diff:  "Maps have different size: 2 != 1",
		}, {
			a: map[interface{}]interface{}{
				&map[string]interface{}{"a": "foo", "b": int16(8)}: "foo",
			},
			b: map[interface{}]interface{}{
				&map[string]interface{}{"a": "foo", "b": int16(8)}: "foo",
				&map[string]interface{}{"a": "foo", "b": int8(8)}:  "foo",
			},
			equal: false,
			diff:  "Maps have different size: 1 != 2",
		}, {
			a:     []string{},
			b:     []string{},
			equal: true,
		}, {
			a:     []string{"foo", "bar"},
			b:     []string{"foo", "bar"},
			equal: true,
		}, {
			a:     []string{"foo", "bar"},
			b:     []string{"foo"},
			equal: false,
			diff:  "Arrays have different size: 2 != 1",
		}, {
			a:     []string{"foo"},
			b:     []string{"foo", "bar"},
			equal: false,
			diff:  "Arrays have different size: 1 != 2",
		}, {
			a:     []string{"foo", "bar"},
			b:     []string{"bar", "foo"},
			equal: false,
			diff:  "In arrays, values are different at index 0: Strings different: \"foo\" vs \"bar\"",
		}, {
			a:     &[]string{},
			b:     []string{},
			equal: false,
			diff:  "types are different: *[]string vs []string",
		}, {
			a:     &[]string{},
			b:     &[]string{},
			equal: true,
		}, {
			a:     &[]string{"foo", "bar"},
			b:     &[]string{"foo", "bar"},
			equal: true,
		}, {
			a:     &[]string{"foo", "bar"},
			b:     &[]string{"foo"},
			equal: false,
			diff:  "Arrays have different size: 2 != 1",
		}, {
			a:     &[]string{"foo"},
			b:     &[]string{"foo", "bar"},
			equal: false,
			diff:  "Arrays have different size: 1 != 2",
		}, {
			a:     &[]string{"foo", "bar"},
			b:     &[]string{"bar", "foo"},
			equal: false,
			diff:  "In arrays, values are different at index 0: Strings different: \"foo\" vs \"bar\"",
		}, {
			a:     []uint32{42, 51},
			b:     []uint32{42, 51},
			equal: true,
		}, {
			a:     []uint32{42, 51},
			b:     []uint32{42, 88},
			equal: false,
			diff:  "In arrays, values are different at index 1: Uints different: 51, 88",
		}, {
			a:     []uint32{42, 51},
			b:     []uint32{42},
			equal: false,
			diff:  "Arrays have different size: 2 != 1",
		}, {
			a:     []uint32{42, 51},
			b:     []uint64{42, 51},
			equal: false,
			diff:  "types are different: []uint32 vs []uint64",
		}, {
			a:     []uint64{42, 51},
			b:     []uint32{42, 51},
			equal: false,
			diff:  "types are different: []uint64 vs []uint32",
		}, {
			a:     []uint64{42, 51},
			b:     []uint64{42, 51},
			equal: true,
		}, {
			a:     []uint64{42, 51},
			b:     []uint64{42},
			equal: false,
			diff:  "Arrays have different size: 2 != 1",
		}, {
			a:     []uint64{42, 51},
			b:     []uint64{42, 88},
			equal: false,
			diff:  "In arrays, values are different at index 1: Uints different: 51, 88",
		}, {
			a:     []interface{}{"foo", uint32(42)},
			b:     []interface{}{"foo", uint32(42)},
			equal: true,
		}, {
			a:     []interface{}{"foo", uint32(42)},
			b:     []interface{}{"foo"},
			equal: false,
			diff:  "Arrays have different size: 2 != 1",
		}, {
			a:     []interface{}{"foo"},
			b:     []interface{}{"foo", uint32(42)},
			equal: false,
			diff:  "Arrays have different size: 1 != 2",
		}, {
			a:     []interface{}{"foo", uint32(42)},
			b:     []interface{}{"foo", uint8(42)},
			equal: false,
			diff:  "In arrays, values are different at index 1: types are different: uint32 vs uint8",
		}, {
			a:     []interface{}{"foo", "bar"},
			b:     []string{"foo", "bar"},
			equal: false,
			diff:  "types are different: []interface {} vs []string",
		}, {
			a:     &[]interface{}{"foo", uint32(42)},
			b:     &[]interface{}{"foo", uint32(42)},
			equal: true,
		}, {
			a:     &[]interface{}{"foo", uint32(42)},
			b:     []interface{}{"foo", uint32(42)},
			equal: false,
			diff:  "types are different: *[]interface {} vs []interface {}",
		}, {
			a:     comparable{a: 42},
			b:     comparable{a: 42},
			equal: true,
		}, {
			a:     comparable{a: 42, t: t},
			b:     comparable{a: 42},
			equal: true,
		}, {
			a:     comparable{a: 42},
			b:     comparable{a: 42, t: t},
			equal: true,
		}, {
			a:     comparable{a: 42},
			b:     comparable{a: 51},
			equal: false,
			diff:  "Comparable types are different: test_test.comparable{a:0x2a, t:(*testing.T)(nil)} vs test_test.comparable{a:0x33, t:(*testing.T)(nil)}",
		}, {
			a:     builtinCompare{a: 42, b: "foo"},
			b:     builtinCompare{a: 42, b: "foo"},
			equal: true,
		}, {
			a:     builtinCompare{a: 42, b: "foo"},
			b:     builtinCompare{a: 42, b: "bar"},
			equal: false,
			diff:  "Structs types are different: test_test.builtinCompare{a:0x2a, b:\"foo\"} vs test_test.builtinCompare{a:0x2a, b:\"bar\"}",
		}, {
			a:     map[int8]int8{2: 3, 3: 4},
			b:     map[int8]int8{2: 3, 3: 4},
			equal: true,
		}, {
			a:     map[int8]int8{2: 3, 3: 4},
			b:     map[int8]int8{2: 3, 3: 5},
			equal: false,
			diff:  "for key int8(3) in map, values are different: Ints different: 4, 5",
		}}
}
