// Copyright (c) 2019 Arista Networks, Inc.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the COPYING file.

package key

import (
	"fmt"
	"testing"
)

func TestMapEqual(t *testing.T) {
	tests := []struct {
		a      *Map
		b      *Map
		result bool
	}{{ // empty
		a:      &Map{},
		b:      &Map{normal: map[interface{}]interface{}{}, custom: map[uint64]entry{}, length: 0},
		result: true,
	}, { // length check
		a:      &Map{},
		b:      &Map{normal: map[interface{}]interface{}{}, custom: map[uint64]entry{}, length: 1},
		result: false,
	}, { // map[string]interface{}
		a:      &Map{normal: map[interface{}]interface{}{"a": 1}, length: 1},
		b:      NewMap("a", 1),
		result: true,
	}, { // differing keys in normal
		a:      &Map{normal: map[interface{}]interface{}{"a": "b"}, length: 1},
		b:      NewMap("b", "b"),
		result: false,
	}, { // differing values in normal
		a:      &Map{normal: map[interface{}]interface{}{"a": "b"}, length: 1},
		b:      NewMap("a", false),
		result: false,
	}, { // multiple entries
		a:      &Map{normal: map[interface{}]interface{}{"a": 1, "b": true}, length: 2},
		b:      NewMap("a", 1, "b", true),
		result: true,
	}, { // nested maps in values
		a: &Map{
			normal: map[interface{}]interface{}{"a": map[string]interface{}{"b": 3}},
			length: 1},
		b:      NewMap("a", map[string]interface{}{"b": 3}),
		result: true,
	}, { // differing nested maps in values
		a: &Map{
			normal: map[interface{}]interface{}{"a": map[string]interface{}{"b": 3}},
			length: 1},
		b:      NewMap("a", map[string]interface{}{"b": 4}),
		result: false,
	}, { // map with map as key
		a:      NewMap(New(map[string]interface{}{"a": 123}), "b"),
		b:      NewMap(New(map[string]interface{}{"a": 123}), "b"),
		result: true,
	}, {
		a:      NewMap(New(map[string]interface{}{"a": 123}), "a"),
		b:      NewMap(New(map[string]interface{}{"a": 123}), "b"),
		result: false,
	}, {
		a:      NewMap(New(map[string]interface{}{"a": 123}), "b"),
		b:      NewMap(New(map[string]interface{}{"b": 123}), "b"),
		result: false,
	}, {
		a:      NewMap(New(map[string]interface{}{"a": 1, "b": 2}), "c"),
		b:      NewMap(New(map[string]interface{}{"a": 1, "b": 2}), "c"),
		result: true,
	}, { // maps with keys that hash to same buckets
		a: &Map{length: 3, custom: map[uint64]entry{
			1234567890: entry{k: dumbHashable{dumb: "hashable1"}, v: 1,
				next: &entry{k: dumbHashable{dumb: "hashable2"}, v: 2,
					next: &entry{k: dumbHashable{dumb: "hashable3"}, v: 3}}}}},
		b: NewMap(
			dumbHashable{dumb: "hashable3"}, 3,
			dumbHashable{dumb: "hashable2"}, 2,
			dumbHashable{dumb: "hashable1"}, 1),
		result: true,
	}, { // maps with map as value
		a: &Map{normal: map[interface{}]interface{}{
			"foo": &Map{normal: map[interface{}]interface{}{"a": 1}, length: 1}}, length: 1},
		b: &Map{normal: map[interface{}]interface{}{
			"foo": &Map{normal: map[interface{}]interface{}{"a": 1}, length: 1}}, length: 1},
		result: true,
	}}

	for _, tcase := range tests {
		if tcase.a.Equal(tcase.b) != tcase.result {
			t.Errorf("%v and %v are not equal", tcase.a, tcase.b)
		}
	}
}

type dumbHashable struct {
	dumb interface{}
}

func (d dumbHashable) Equal(other interface{}) bool {
	if o, ok := other.(dumbHashable); ok {
		return d.dumb == o.dumb
	}
	return false
}

func (d dumbHashable) Hash() uint64 {
	return 1234567890
}

func TestMapSet(t *testing.T) {
	tests := []struct {
		m      *Map
		k      interface{}
		v      interface{}
		result *Map
	}{{
		m:      &Map{},
		k:      nil,
		v:      nil,
		result: &Map{},
	}, {
		m:      &Map{},
		k:      "a",
		v:      1,
		result: &Map{normal: map[interface{}]interface{}{"a": 1}, length: 1},
	}, {
		m:      &Map{normal: map[interface{}]interface{}{"a": 1}, length: 1},
		k:      "a",
		v:      1,
		result: &Map{normal: map[interface{}]interface{}{"a": 1}, length: 1},
	}, {
		m: &Map{},
		k: dumbHashable{dumb: "hashable1"},
		v: 42,
		result: &Map{length: 1, custom: map[uint64]entry{
			1234567890: entry{k: dumbHashable{dumb: "hashable1"}, v: 42}}},
	}, {
		m: &Map{length: 1, custom: map[uint64]entry{
			1234567890: entry{k: dumbHashable{dumb: "hashable1"}, v: 42}}},
		k: dumbHashable{dumb: "hashable1"},
		v: 0,
		result: &Map{length: 1, custom: map[uint64]entry{
			1234567890: entry{k: dumbHashable{dumb: "hashable1"}, v: 0}}},
	}, {
		m: &Map{},
		k: dumbHashable{dumb: "hashable2"},
		v: 42,
		result: &Map{length: 1, custom: map[uint64]entry{
			1234567890: entry{k: dumbHashable{dumb: "hashable2"}, v: 42}}},
	}, {
		m: &Map{length: 1, custom: map[uint64]entry{
			1234567890: entry{k: dumbHashable{dumb: "hashable2"}, v: 42}}},
		k: dumbHashable{dumb: "hashable3"},
		v: 42,
		result: &Map{length: 2, custom: map[uint64]entry{
			1234567890: entry{k: dumbHashable{dumb: "hashable2"}, v: 42,
				next: &entry{k: dumbHashable{dumb: "hashable3"}, v: 42}}}},
	}, {
		m: &Map{length: 2, custom: map[uint64]entry{
			1234567890: entry{k: dumbHashable{dumb: "hashable2"}, v: 42,
				next: &entry{k: dumbHashable{dumb: "hashable3"}, v: 42}}}},
		k: dumbHashable{dumb: "hashable4"},
		v: 42,
		result: &Map{length: 3, custom: map[uint64]entry{
			1234567890: entry{k: dumbHashable{dumb: "hashable2"}, v: 42,
				next: &entry{k: dumbHashable{dumb: "hashable3"}, v: 42,
					next: &entry{k: dumbHashable{dumb: "hashable4"}, v: 42}}}}},
	}}

	for _, tcase := range tests {
		setmap := tcase.m
		setmap.Set(tcase.k, tcase.v)
		if !setmap.Equal(tcase.result) {
			t.Errorf("set map %#v does not equal expected result %#v", setmap, tcase.result)
		}
	}
}

func TestMapSetGet(t *testing.T) {
	m := Map{}
	tests := []struct {
		setkey interface{}
		getkey interface{}
		val    interface{}
		found  bool
	}{{
		setkey: "a",
		getkey: "a",
		val:    1,
		found:  true,
	}, {
		setkey: "b",
		getkey: "b",
		val:    1,
		found:  true,
	}, {
		setkey: 42,
		getkey: 42,
		val:    "foobar",
		found:  true,
	}, {
		setkey: dumbHashable{dumb: "hashable1"},
		getkey: dumbHashable{dumb: "hashable1"},
		val:    1,
		found:  true,
	}, {
		getkey: dumbHashable{dumb: "hashable2"},
		val:    nil,
		found:  false,
	}, {
		setkey: dumbHashable{dumb: "hashable2"},
		getkey: dumbHashable{dumb: "hashable2"},
		val:    2,
		found:  true,
	}, {
		getkey: dumbHashable{dumb: "hashable42"},
		val:    nil,
		found:  false,
	}, {
		setkey: New(map[string]interface{}{"a": 1}),
		getkey: New(map[string]interface{}{"a": 1}),
		val:    "foo",
		found:  true,
	}, {
		getkey: New(map[string]interface{}{"a": 2}),
		val:    nil,
		found:  false,
	}, {
		setkey: New(map[string]interface{}{"a": 2}),
		getkey: New(map[string]interface{}{"a": 2}),
		val:    "bar",
		found:  true,
	}}
	for _, tcase := range tests {
		if tcase.setkey != nil {
			m.Set(tcase.setkey, tcase.val)
		}
		val, found := m.Get(tcase.getkey)
		if found != tcase.found {
			t.Errorf("found is %t, but expected found %t", found, tcase.found)
		}
		if val != tcase.val {
			t.Errorf("val is %v for key %v, but expected val %v", val, tcase.getkey, tcase.val)
		}
	}

}

func TestMapDel(t *testing.T) {
	tests := []struct {
		m   *Map
		del interface{}
		exp *Map
	}{{
		m:   &Map{},
		del: "a",
		exp: &Map{},
	}, {
		m:   &Map{},
		del: New(map[string]interface{}{"a": 1}),
		exp: &Map{},
	}, {
		m:   &Map{normal: map[interface{}]interface{}{"a": true}, length: 1},
		del: "a",
		exp: &Map{},
	}, {
		m: &Map{length: 1, custom: map[uint64]entry{
			1234567890: entry{k: dumbHashable{dumb: "hashable1"}, v: 42}}},
		del: dumbHashable{dumb: "hashable1"},
		exp: &Map{},
	}, {
		m: &Map{length: 3, custom: map[uint64]entry{
			1234567890: entry{k: dumbHashable{dumb: "hashable2"}, v: 42,
				next: &entry{k: dumbHashable{dumb: "hashable3"}, v: 42,
					next: &entry{k: dumbHashable{dumb: "hashable4"}, v: 42}}}}},
		del: dumbHashable{dumb: "hashable2"},
		exp: &Map{length: 2, custom: map[uint64]entry{
			1234567890: entry{k: dumbHashable{dumb: "hashable3"}, v: 42,
				next: &entry{k: dumbHashable{dumb: "hashable4"}, v: 42}}}},
	}, {
		m: &Map{length: 3, custom: map[uint64]entry{
			1234567890: entry{k: dumbHashable{dumb: "hashable2"}, v: 42,
				next: &entry{k: dumbHashable{dumb: "hashable3"}, v: 42,
					next: &entry{k: dumbHashable{dumb: "hashable4"}, v: 42}}}}},
		del: dumbHashable{dumb: "hashable3"},
		exp: &Map{length: 2, custom: map[uint64]entry{
			1234567890: entry{k: dumbHashable{dumb: "hashable2"}, v: 42,
				next: &entry{k: dumbHashable{dumb: "hashable4"}, v: 42}}}},
	}, {
		m: &Map{length: 3, custom: map[uint64]entry{
			1234567890: entry{k: dumbHashable{dumb: "hashable2"}, v: 42,
				next: &entry{k: dumbHashable{dumb: "hashable3"}, v: 42,
					next: &entry{k: dumbHashable{dumb: "hashable4"}, v: 42}}}}},
		del: dumbHashable{dumb: "hashable4"},
		exp: &Map{length: 2, custom: map[uint64]entry{
			1234567890: entry{k: dumbHashable{dumb: "hashable2"}, v: 42,
				next: &entry{k: dumbHashable{dumb: "hashable3"}, v: 42}}}},
	}, {
		m: &Map{length: 3, custom: map[uint64]entry{
			1234567890: entry{k: dumbHashable{dumb: "hashable2"}, v: 42,
				next: &entry{k: dumbHashable{dumb: "hashable3"}, v: 42,
					next: &entry{k: dumbHashable{dumb: "hashable4"}, v: 42}}}}},
		del: dumbHashable{dumb: "hashable5"},
		exp: &Map{length: 3, custom: map[uint64]entry{
			1234567890: entry{k: dumbHashable{dumb: "hashable2"}, v: 42,
				next: &entry{k: dumbHashable{dumb: "hashable3"}, v: 42,
					next: &entry{k: dumbHashable{dumb: "hashable4"}, v: 42}}}}},
	}}

	for _, tcase := range tests {
		tcase.m.Del(tcase.del)
		if !tcase.m.Equal(tcase.exp) {
			t.Errorf("map %#v after del of element %v does not equal expected %#v",
				tcase.m, tcase.del, tcase.exp)
		}
	}
}

func contains(elementlist []interface{}, element interface{}) bool {
	for _, el := range elementlist {
		if el == element {
			return true
		}
	}
	return false
}

func TestMapIter(t *testing.T) {
	tests := []struct {
		m     *Map
		elems []interface{}
	}{{
		m:     &Map{},
		elems: []interface{}{},
	}, {
		m:     &Map{normal: map[interface{}]interface{}{"a": true}, length: 1},
		elems: []interface{}{"a"},
	}, {
		m: &Map{length: 1, custom: map[uint64]entry{
			1234567890: entry{k: dumbHashable{dumb: "hashable1"}, v: 42}}},
		elems: []interface{}{dumbHashable{dumb: "hashable1"}},
	}, {
		m: &Map{length: 1, custom: map[uint64]entry{
			1234567890: entry{k: dumbHashable{dumb: "hashable1"}, v: 42}}},
		elems: []interface{}{dumbHashable{dumb: "hashable1"}},
	}, {
		m: &Map{length: 3, custom: map[uint64]entry{
			1234567890: entry{k: dumbHashable{dumb: "hashable2"}, v: 42,
				next: &entry{k: dumbHashable{dumb: "hashable3"}, v: 42,
					next: &entry{k: dumbHashable{dumb: "hashable4"}, v: 42}}}}},
		elems: []interface{}{dumbHashable{dumb: "hashable2"},
			dumbHashable{dumb: "hashable3"}, dumbHashable{dumb: "hashable4"}},
	}, {
		m: NewMap(
			New(map[string]interface{}{"a": 123}), "b",
			New(map[string]interface{}{"c": 456}), "d",
			dumbHashable{dumb: "hashable1"}, 1,
			dumbHashable{dumb: "hashable2"}, 2,
			dumbHashable{dumb: "hashable3"}, 3,
			"x", true,
			"y", false,
			"z", nil,
		),
		elems: []interface{}{
			New(map[string]interface{}{"a": 123}), New(map[string]interface{}{"c": 456}),
			dumbHashable{dumb: "hashable1"}, dumbHashable{dumb: "hashable2"},
			dumbHashable{dumb: "hashable3"}, "x", "y", "z"},
	}}
	for _, tcase := range tests {
		count := 0
		iterfunc := func(k, v interface{}) error {
			if !contains(tcase.elems, k) {
				return fmt.Errorf("map %#v should not contain element %v", tcase.m, k)
			}
			count++
			return nil
		}
		if err := tcase.m.Iter(iterfunc); err != nil {
			t.Errorf("unexpected error %v", err)
		}

		expectedcount := len(tcase.elems)
		if count != expectedcount || tcase.m.length != expectedcount {
			t.Errorf("found %d elements in map %#v when expected %d", count, tcase.m, expectedcount)
		}
	}
}
