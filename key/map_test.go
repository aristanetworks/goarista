// Copyright (c) 2019 Arista Networks, Inc.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the COPYING file.

package key

import (
	"fmt"
	"math/rand"
	"strings"
	"testing"
)

func (m *Map) debug() string {
	var buf strings.Builder
	for hash, entry := range m.custom {
		fmt.Fprintf(&buf, "%d: ", hash)
		first := true
		_ = entryIter(entry, func(k, v interface{}) error {
			if !first {
				buf.WriteString(" -> ")
			}
			first = false
			fmt.Fprintf(&buf, "{%v:%v}", k, v)
			return nil
		})
		buf.WriteByte('\n')
	}
	return buf.String()
}

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
	}, { // maps with keys that hash to same buckets in different order
		a: NewMap(
			dumbHashable{dumb: "hashable1"}, 1,
			dumbHashable{dumb: "hashable2"}, 2,
			dumbHashable{dumb: "hashable3"}, 3),
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

func TestMapEntry(t *testing.T) {
	m := NewMap()
	verifyPresent := func(k, v interface{}) {
		t.Helper()
		if got, ok := m.Get(k); !ok || got != v {
			t.Errorf("Get(%v): expected %v, got %v", k, v, got)
		}
	}
	verifyAbsent := func(k interface{}) {
		t.Helper()
		if got, ok := m.Get(k); ok {
			t.Errorf("Get(%v): expected not found, got %v", k, got)
		}
	}

	// create entry list 1 -> 2 -> 3
	for i := 1; i <= 3; i++ {
		m.Set(dumbHashable{i}, 0)
		if m.Len() != i {
			t.Errorf("expected len %d, got %d", i, m.Len())
		}
		verifyPresent(dumbHashable{i}, 0)
	}
	if len(m.custom) != 1 {
		t.Errorf("expected custom map to have 1 entry list, got %d", len(m.custom))
	}
	if m.Len() != 3 {
		t.Errorf("expected len of 3, got %d", m.Len())
	}

	// overwrite list members
	for i := 1; i <= 3; i++ {
		m.Set(dumbHashable{i}, i)
		verifyPresent(dumbHashable{i}, i)
	}
	if m.Len() != 3 {
		t.Errorf("expected len of 3, got %d", m.Len())
	}
	t.Log(m.debug())

	// delete nonexistant member
	m.Del(dumbHashable{4})
	if m.Len() != 3 {
		t.Errorf("expected len of 3, got %d", m.Len())
	}

	// Check that iter works
	i := 1
	_ = m.Iter(func(k, v interface{}) error {
		exp := dumbHashable{i}
		if k != exp {
			t.Errorf("expected key %v got %v", exp, k)
		}
		if v != i {
			t.Errorf("expected val %d got %v", i, v)
		}
		i++
		return nil
	})

	// delete middle of list
	m.Del(dumbHashable{2})
	verifyPresent(dumbHashable{1}, 1)
	verifyAbsent(dumbHashable{2})
	verifyPresent(dumbHashable{3}, 3)
	if m.Len() != 2 {
		t.Errorf("expected len of 2, got %d", m.Len())
	}

	// delete end of list
	m.Del(dumbHashable{3})
	verifyPresent(dumbHashable{1}, 1)
	verifyAbsent(dumbHashable{3})
	if m.Len() != 1 {
		t.Errorf("expected len of 1, got %d", m.Len())
	}

	m.Set(dumbHashable{2}, 2)
	// delete head of list with next member
	m.Del(dumbHashable{1})
	verifyAbsent(dumbHashable{1})
	verifyPresent(dumbHashable{2}, 2)
	if m.Len() != 1 {
		t.Errorf("expected len of 1, got %d", m.Len())
	}

	// delete final list member
	m.Del(dumbHashable{2})
	verifyAbsent(dumbHashable{2})
	if m.Len() != 0 {
		t.Errorf("expected len of 0, got %d", m.Len())
	}

	if len(m.custom) != 0 {
		t.Errorf("expected m.custom to be empty, but got len %d", len(m.custom))
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
		m:   NewMap(),
		del: "a",
		exp: NewMap(),
	}, {
		m:   NewMap(),
		del: New(map[string]interface{}{"a": 1}),
		exp: NewMap(),
	}, {
		m:   NewMap("a", true),
		del: "a",
		exp: NewMap(),
	}, {
		m:   NewMap(dumbHashable{dumb: "hashable1"}, 42),
		del: dumbHashable{dumb: "hashable1"},
		exp: NewMap(),
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
	equal := func(v interface{}) bool { return element == v }
	if comp, ok := element.(Comparable); ok {
		equal = func(v interface{}) bool { return comp.Equal(v) }
	}
	for _, el := range elementlist {
		if equal(el) {
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
		m:     NewMap(),
		elems: []interface{}{},
	}, {
		m:     NewMap("a", true),
		elems: []interface{}{"a"},
	}, {
		m:     NewMap(dumbHashable{dumb: "hashable1"}, 42),
		elems: []interface{}{dumbHashable{dumb: "hashable1"}},
	}, {
		m: NewMap(dumbHashable{dumb: "hashable2"}, 42,
			dumbHashable{dumb: "hashable3"}, 42,
			dumbHashable{dumb: "hashable4"}, 42),
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

func TestMapString(t *testing.T) {
	for _, tc := range []struct {
		m *Map
		s string
	}{{
		m: NewMap(),
		s: "key.Map[]",
	}, {
		m: NewMap("1", "2"),
		s: "key.Map[1:2]",
	}, {
		m: NewMap(
			"3", "4",
			"1", "2",
		),
		s: "key.Map[1:2 3:4]",
	}, {
		m: NewMap(
			New(map[string]interface{}{"key1": uint32(1), "key2": uint32(2)}), "foobar",
			New(map[string]interface{}{"key1": uint32(3), "key2": uint32(4)}), "bazquux",
		),
		s: "key.Map[map[key1:1 key2:2]:foobar map[key1:3 key2:4]:bazquux]",
	}} {
		t.Run(tc.s, func(t *testing.T) {
			out := tc.m.String()
			if out != tc.s {
				t.Errorf("expected %q got %q", tc.s, out)
			}
		})
	}
}

func BenchmarkMapGrow(b *testing.B) {
	keys := make([]Key, 150)
	for j := 0; j < len(keys); j++ {
		keys[j] = New(map[string]interface{}{
			"foobar": 100,
			"baz":    j,
		})
	}
	b.Run("map[key.Key]interface{}", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			m := make(map[Key]interface{})
			for j := 0; j < len(keys); j++ {
				m[keys[j]] = "foobar"
			}
			if len(m) != len(keys) {
				b.Fatal(m)
			}
		}
	})
	b.Run("key.Map", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			m := NewMap()
			for j := 0; j < len(keys); j++ {
				m.Set(keys[j], "foobar")
			}
			if m.Len() != len(keys) {
				b.Fatal(m)
			}
		}
	})
}

func BenchmarkMapGet(b *testing.B) {
	keys := make([]Key, 150)
	for j := 0; j < len(keys); j++ {
		keys[j] = New(map[string]interface{}{
			"foobar": 100,
			"baz":    j,
		})
	}
	keysRandomOrder := make([]Key, len(keys))
	copy(keysRandomOrder, keys)
	rand.Shuffle(len(keysRandomOrder), func(i, j int) {
		keysRandomOrder[i], keysRandomOrder[j] = keysRandomOrder[j], keysRandomOrder[i]
	})
	b.Run("map[key.Key]interface{}", func(b *testing.B) {
		m := make(map[Key]interface{})
		for j := 0; j < len(keys); j++ {
			m[keys[j]] = "foobar"
		}
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			for _, k := range keysRandomOrder {
				_, ok := m[k]
				if !ok {
					b.Fatal("didn't find key")
				}
			}
		}
	})
	b.Run("key.Map", func(b *testing.B) {
		m := NewMap()
		for j := 0; j < len(keys); j++ {
			m.Set(keys[j], "foobar")
		}
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			for _, k := range keysRandomOrder {
				_, ok := m.Get(k)
				if !ok {
					b.Fatal("didn't find key")
				}
			}
		}
	})
}
