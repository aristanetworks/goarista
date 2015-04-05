// Copyright (c) 2015 Arista Networks, Inc.  All rights reserved.
// Arista Networks, Inc. Confidential and Proprietary.

package test

import (
	"testing"
)

type builtinCompare struct {
	a uint32
	b string
}

type complexCompare struct {
	m map[builtinCompare]int8
	p *complexCompare
}

type partialCompare struct {
	a uint32
	b string `deepequal:"ignore"`
}

type keyInterface interface {
	Key() interface{}
}

type keyImpl struct {
	k interface{}
}

func (k keyImpl) Key() interface{} {
	return k.k
}

func mkKey(v interface{}) keyInterface {
	return keyImpl{k: v}
}

type deepEqualTestCase struct {
	a, b interface{}
	diff string
}

var deepEqualNullMapString map[string]interface{}

func getDeepEqualTests(t *testing.T) []deepEqualTestCase {
	recursive := &complexCompare{}
	recursive.p = recursive
	return []deepEqualTestCase{{
		a: nil,
		b: nil,
	}, {
		a: uint8(5),
		b: uint8(5),
	}, {
		a:    nil,
		b:    uint8(5),
		diff: "one value is nil and the other is of type: uint8",
	}, {
		a:    uint8(5),
		b:    nil,
		diff: "one value is nil and the other is of type: uint8",
	}, {
		a:    uint16(1),
		b:    uint16(2),
		diff: "Uints different: 1, 2",
	}, {
		a:    int8(1),
		b:    int16(1),
		diff: "types are different: int8 vs int16",
	}, {
		a: true,
		b: true,
	}, {
		a: float32(3.1415),
		b: float32(3.1415),
	}, {
		a:    float32(3.1415),
		b:    float32(3.1416),
		diff: "Floats different: 3.1415, 3.1416",
	}, {
		a: float64(3.14159265),
		b: float64(3.14159265),
	}, {
		a:    float64(3.14159265),
		b:    float64(3.14159266),
		diff: "Floats different: 3.14159265, 3.14159266",
	}, {
		a: deepEqualNullMapString,
		b: deepEqualNullMapString,
	}, {
		a: &deepEqualNullMapString,
		b: &deepEqualNullMapString,
	}, {
		a:    deepEqualNullMapString,
		b:    &deepEqualNullMapString,
		diff: "types are different: map[string]interface {} vs *map[string]interface {}",
	}, {
		a:    &deepEqualNullMapString,
		b:    deepEqualNullMapString,
		diff: "types are different: *map[string]interface {} vs map[string]interface {}",
	}, {
		a: map[string]interface{}{"a": uint32(42)},
		b: map[string]interface{}{"a": uint32(42)},
	}, {
		a:    map[string]interface{}{"a": int32(42)},
		b:    map[string]interface{}{"a": int32(51)},
		diff: `for key "a" in map, values are different: Ints different: 42, 51`,
	}, {
		a:    map[string]interface{}{"a": uint32(42)},
		b:    map[string]interface{}{},
		diff: "Maps have different size: 1 != 0",
	}, {
		a:    map[string]interface{}{},
		b:    map[string]interface{}{"a": uint32(42)},
		diff: "Maps have different size: 0 != 1",
	}, {
		a:    map[string]interface{}{"a": uint64(42), "b": "extra"},
		b:    map[string]interface{}{"a": uint64(42)},
		diff: "Maps have different size: 2 != 1",
	}, {
		a:    map[string]interface{}{"a": uint64(42)},
		b:    map[string]interface{}{"a": uint64(42), "b": "extra"},
		diff: "Maps have different size: 1 != 2",
	}, {
		a: map[uint32]interface{}{uint32(42): "foo"},
		b: map[uint32]interface{}{uint32(42): "foo"},
	}, {
		a:    map[uint32]interface{}{uint32(42): "foo"},
		b:    map[uint32]interface{}{uint32(51): "foo"},
		diff: "key uint32(42) in map is missing in the second map",
	}, {
		a:    map[uint32]interface{}{uint32(42): "foo"},
		b:    map[uint32]interface{}{uint32(42): "foo", uint32(51): "bar"},
		diff: "Maps have different size: 1 != 2",
	}, {
		a:    map[uint32]interface{}{uint32(42): "foo"},
		b:    map[uint64]interface{}{uint64(42): "foo"},
		diff: "types are different: map[uint32]interface {} vs map[uint64]interface {}",
	}, {
		a: map[uint64]interface{}{uint64(42): "foo"},
		b: map[uint64]interface{}{uint64(42): "foo"},
	}, {
		a:    map[uint64]interface{}{uint64(42): "foo"},
		b:    map[uint64]interface{}{uint64(51): "foo"},
		diff: "key uint64(42) in map is missing in the second map",
	}, {
		a:    map[uint64]interface{}{uint64(42): "foo"},
		b:    map[uint64]interface{}{uint64(42): "foo", uint64(51): "bar"},
		diff: "Maps have different size: 1 != 2",
	}, {
		a: map[uint64]interface{}{uint64(42): "foo"},
		b: map[interface{}]interface{}{uint32(42): "foo"},
		diff: "types are different: map[uint64]interface {} vs" +
			" map[interface {}]interface {}",
	}, {
		a: map[interface{}]interface{}{"a": uint32(42)},
		b: map[string]interface{}{"a": uint32(42)},
		diff: "types are different: map[interface {}]interface {} vs" +
			" map[string]interface {}",
	}, {
		a: map[interface{}]interface{}{},
		b: map[interface{}]interface{}{},
	}, {
		a: &map[interface{}]interface{}{},
		b: &map[interface{}]interface{}{},
	}, {
		a: map[interface{}]interface{}{
			&map[string]interface{}{"a": "foo", "b": int16(8)}: "foo"},
		b: map[interface{}]interface{}{
			&map[string]interface{}{"a": "foo", "b": int16(8)}: "foo"},
	}, {
		a: map[interface{}]interface{}{
			&map[string]interface{}{"a": "foo", "b": uint32(8)}: "foo"},
		b: map[interface{}]interface{}{
			&map[string]interface{}{"a": "foo", "b": uint32(8)}: "fox"},
		diff: `for complex key *map[string]interface {}{"a":"foo", "b":uint32(8)}` +
			` in map, values are different: Strings different: "foo" vs "fox"`,
	}, {
		a: map[interface{}]interface{}{
			&map[string]interface{}{"a": "foo", "b": uint32(8)}: "foo"},
		b: map[interface{}]interface{}{
			&map[string]interface{}{"a": "foo", "b": uint32(5)}: "foo"},
		diff: `complex key *map[string]interface {}{"a":"foo", "b":uint32(8)}` +
			` in map is missing in the second map`,
	}, {
		a: map[interface{}]interface{}{
			&map[string]interface{}{"a": "foo", "b": uint32(8)}: "foo"},
		b: map[interface{}]interface{}{
			&map[string]interface{}{"a": "foo"}: "foo"},
		diff: `complex key *map[string]interface {}{"a":"foo", "b":uint32(8)}` +
			` in map is missing in the second map`,
	}, {
		a: map[interface{}]interface{}{
			&map[string]interface{}{"a": "foo", "b": int16(8)}: "foo",
			&map[string]interface{}{"a": "foo", "b": int8(8)}:  "foo",
		},
		b: map[interface{}]interface{}{
			&map[string]interface{}{"a": "foo", "b": int16(8)}: "foo",
			&map[string]interface{}{"a": "foo", "b": int8(8)}:  "foo",
		},
	}, {
		a: map[interface{}]interface{}{
			&map[string]interface{}{"a": "foo", "b": int16(8)}: "foo",
			&map[string]interface{}{"a": "foo", "b": int8(8)}:  "foo",
		},
		b: map[interface{}]interface{}{
			&map[string]interface{}{"a": "foo", "b": int16(8)}: "foo",
			&map[string]interface{}{"a": "foo", "b": int8(5)}:  "foo",
		},
		diff: `complex key *map[string]interface {}{"a":"foo", "b":int8(8)}` +
			` in map is missing in the second map`,
	}, {
		a: map[interface{}]interface{}{
			&map[string]interface{}{"a": "foo", "b": int16(8)}: "foo",
			&map[string]interface{}{"a": "foo", "b": int8(8)}:  "foo",
		},
		b: map[interface{}]interface{}{
			&map[string]interface{}{"a": "foo", "b": int16(8)}: "foo",
			&map[string]interface{}{"a": "foo", "b": int32(8)}: "foo",
		},
		diff: `complex key *map[string]interface {}{"a":"foo", "b":int8(8)}` +
			` in map is missing in the second map`,
	}, {
		a: map[interface{}]interface{}{
			&map[string]interface{}{"a": "foo", "b": int16(8)}: "foo",
			&map[string]interface{}{"a": "foo", "b": int8(8)}:  "foo",
		},
		b: map[interface{}]interface{}{
			&map[string]interface{}{"a": "foo", "b": int16(8)}: "foo",
		},
		diff: "Maps have different size: 2 != 1",
	}, {
		a: map[interface{}]interface{}{
			&map[string]interface{}{"a": "foo", "b": int16(8)}: "foo",
		},
		b: map[interface{}]interface{}{
			&map[string]interface{}{"a": "foo", "b": int16(8)}: "foo",
			&map[string]interface{}{"a": "foo", "b": int8(8)}:  "foo",
		},
		diff: "Maps have different size: 1 != 2",
	}, {
		a: []string{},
		b: []string{},
	}, {
		a: []string{"foo", "bar"},
		b: []string{"foo", "bar"},
	}, {
		a:    []string{"foo", "bar"},
		b:    []string{"foo"},
		diff: "Arrays have different size: 2 != 1",
	}, {
		a:    []string{"foo"},
		b:    []string{"foo", "bar"},
		diff: "Arrays have different size: 1 != 2",
	}, {
		a: []string{"foo", "bar"},
		b: []string{"bar", "foo"},
		diff: `In arrays, values are different at index 0:` +
			` Strings different: "foo" vs "bar"`,
	}, {
		a:    &[]string{},
		b:    []string{},
		diff: "types are different: *[]string vs []string",
	}, {
		a: &[]string{},
		b: &[]string{},
	}, {
		a: &[]string{"foo", "bar"},
		b: &[]string{"foo", "bar"},
	}, {
		a:    &[]string{"foo", "bar"},
		b:    &[]string{"foo"},
		diff: "Arrays have different size: 2 != 1",
	}, {
		a:    &[]string{"foo"},
		b:    &[]string{"foo", "bar"},
		diff: "Arrays have different size: 1 != 2",
	}, {
		a: &[]string{"foo", "bar"},
		b: &[]string{"bar", "foo"},
		diff: `In arrays, values are different at index 0:` +
			` Strings different: "foo" vs "bar"`,
	}, {
		a: []uint32{42, 51},
		b: []uint32{42, 51},
	}, {
		a:    []uint32{42, 51},
		b:    []uint32{42, 88},
		diff: "In arrays, values are different at index 1: Uints different: 51, 88",
	}, {
		a:    []uint32{42, 51},
		b:    []uint32{42},
		diff: "Arrays have different size: 2 != 1",
	}, {
		a:    []uint32{42, 51},
		b:    []uint64{42, 51},
		diff: "types are different: []uint32 vs []uint64",
	}, {
		a:    []uint64{42, 51},
		b:    []uint32{42, 51},
		diff: "types are different: []uint64 vs []uint32",
	}, {
		a: []uint64{42, 51},
		b: []uint64{42, 51},
	}, {
		a:    []uint64{42, 51},
		b:    []uint64{42},
		diff: "Arrays have different size: 2 != 1",
	}, {
		a:    []uint64{42, 51},
		b:    []uint64{42, 88},
		diff: "In arrays, values are different at index 1: Uints different: 51, 88",
	}, {
		a: []interface{}{"foo", uint32(42)},
		b: []interface{}{"foo", uint32(42)},
	}, {
		a:    []interface{}{"foo", uint32(42)},
		b:    []interface{}{"foo"},
		diff: "Arrays have different size: 2 != 1",
	}, {
		a:    []interface{}{"foo"},
		b:    []interface{}{"foo", uint32(42)},
		diff: "Arrays have different size: 1 != 2",
	}, {
		a: []interface{}{"foo", uint32(42)},
		b: []interface{}{"foo", uint8(42)},
		diff: "In arrays, values are different at index 1:" +
			" types are different: uint32 vs uint8",
	}, {
		a:    []interface{}{"foo", "bar"},
		b:    []string{"foo", "bar"},
		diff: "types are different: []interface {} vs []string",
	}, {
		a: &[]interface{}{"foo", uint32(42)},
		b: &[]interface{}{"foo", uint32(42)},
	}, {
		a:    &[]interface{}{"foo", uint32(42)},
		b:    []interface{}{"foo", uint32(42)},
		diff: "types are different: *[]interface {} vs []interface {}",
	}, {
		a: comparableStruct{a: 42},
		b: comparableStruct{a: 42},
	}, {
		a: comparableStruct{a: 42, t: t},
		b: comparableStruct{a: 42},
	}, {
		a: comparableStruct{a: 42},
		b: comparableStruct{a: 42, t: t},
	}, {
		a: comparableStruct{a: 42},
		b: comparableStruct{a: 51},
		diff: "Comparable types are different: test.comparableStruct{a:" +
			"uint32(42), t:*nil} vs test.comparableStruct{a:uint32(51), t:*nil}",
	}, {
		a: builtinCompare{a: 42, b: "foo"},
		b: builtinCompare{a: 42, b: "foo"},
	}, {
		a:    builtinCompare{a: 42, b: "foo"},
		b:    builtinCompare{a: 42, b: "bar"},
		diff: `attributes "b" are different: Strings different: "foo" vs "bar"`,
	}, {
		a: map[int8]int8{2: 3, 3: 4},
		b: map[int8]int8{2: 3, 3: 4},
	}, {
		a:    map[int8]int8{2: 3, 3: 4},
		b:    map[int8]int8{2: 3, 3: 5},
		diff: "for key int8(3) in map, values are different: Ints different: 4, 5",
	}, {
		a: complexCompare{},
		b: complexCompare{},
	}, {
		a: complexCompare{
			m: map[builtinCompare]int8{builtinCompare{1, "foo"}: 42}},
		b: complexCompare{
			m: map[builtinCompare]int8{builtinCompare{1, "foo"}: 42}},
	}, {
		a: complexCompare{
			m: map[builtinCompare]int8{builtinCompare{1, "foo"}: 42}},
		b: complexCompare{
			m: map[builtinCompare]int8{builtinCompare{1, "foo"}: 51}},
		diff: `attributes "m" are different: for key test.builtinCompare{a:uint32(1),` +
			` b:"foo"} in map, values are different: Ints different: 42, 51`,
	}, {
		a: complexCompare{
			m: map[builtinCompare]int8{builtinCompare{1, "foo"}: 42}},
		b: complexCompare{
			m: map[builtinCompare]int8{builtinCompare{1, "bar"}: 42}},
		diff: `attributes "m" are different: key test.builtinCompare{a:uint32(1),` +
			` b:"foo"} in map is missing in the second map`,
	}, {
		a: recursive,
		b: recursive,
	}, {
		a: complexCompare{p: recursive},
		b: complexCompare{p: recursive},
	}, {
		a: complexCompare{p: &complexCompare{p: recursive}},
		b: complexCompare{p: &complexCompare{p: recursive}},
	}, {
		a: []complexCompare{complexCompare{p: &complexCompare{p: recursive}}},
		b: []complexCompare{complexCompare{p: &complexCompare{p: recursive}}},
	}, {
		a: []complexCompare{complexCompare{p: &complexCompare{p: recursive}}},
		b: []complexCompare{complexCompare{p: &complexCompare{p: nil}}},
		diff: `In arrays, values are different at index 0: attributes "p" are` +
			` different: attributes "p" are different: one value is nil and ` +
			`the other is not nil: *test.complexCompare{m:map[test.` +
			`builtinCompare]int8{}, p:*test.complexCompare{` +
			`<circular dependency>}}`,
	}, {
		a: partialCompare{a: 42},
		b: partialCompare{a: 42},
	}, {
		a:    partialCompare{a: 42},
		b:    partialCompare{a: 51},
		diff: `attributes "a" are different: Uints different: 42, 51`,
	}, {
		a: partialCompare{a: 42, b: "foo"},
		b: partialCompare{a: 42, b: "bar"},
	}, {
		a: map[*builtinCompare]uint32{&builtinCompare{1, "foo"}: 42},
		b: map[*builtinCompare]uint32{&builtinCompare{1, "foo"}: 42},
	}, {
		a: map[*builtinCompare]uint32{&builtinCompare{1, "foo"}: 42},
		b: map[*builtinCompare]uint32{&builtinCompare{2, "foo"}: 42},
		diff: `complex key *test.builtinCompare{a:uint32(1), b:"foo"}` +
			` in map is missing in the second map`,
	}, {
		a: map[*builtinCompare]uint32{&builtinCompare{1, "foo"}: 42},
		b: map[*builtinCompare]uint32{&builtinCompare{1, "foo"}: 51},
		diff: `for complex key *test.builtinCompare{a:uint32(1), b:"foo"}` +
			` in map, values are different: Uints different: 42, 51`,
	}, {
		a: mkKey("a"),
		b: mkKey("a"),
	}, {
		a: map[keyInterface]string{mkKey("a"): "b"},
		b: map[keyInterface]string{mkKey("a"): "b"},
	}, {
		a: map[keyInterface]string{mkKey(&map[string]interface{}{"a": true}): "b"},
		b: map[keyInterface]string{mkKey(&map[string]interface{}{"a": true}): "b"},
	}}
}
