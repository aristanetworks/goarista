// Copyright (c) 2019 Arista Networks, Inc.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the COPYING file.

//go:build go1.18

package key

import (
	"errors"
	"fmt"
	"math/rand"
	"strings"
	"testing"
)

func (m *MapOf[V]) debug() string {
	var buf strings.Builder
	for hash, entry := range m.custom {
		fmt.Fprintf(&buf, "%d: ", hash)
		first := true
		_ = entryOfIter[V](&entry, func(k interface{}, v V) error {
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

func TestMapOfEqual(t *testing.T) {
	tests := []struct {
		a      *MapOf[any]
		b      *MapOf[any]
		result bool
	}{{ // empty
		a:      &MapOf[any]{},
		b:      &MapOf[any]{normal: map[interface{}]interface{}{}, custom: map[uint64]entryOf[any]{}, length: 0},
		result: true,
	}, { // length check
		a:      &MapOf[any]{},
		b:      &MapOf[any]{normal: map[interface{}]interface{}{}, custom: map[uint64]entryOf[any]{}, length: 1},
		result: false,
	}, { // map[string]interface{}
		a:      &MapOf[any]{normal: map[interface{}]interface{}{"a": 1}, length: 1},
		b:      NewMapOf(KV[any]{"a", 1}),
		result: true,
	}, { // differing keys in normal
		a:      &MapOf[any]{normal: map[interface{}]interface{}{"a": "b"}, length: 1},
		b:      NewMapOf[any](KV[any]{"b", "b"}),
		result: false,
	}, { // differing values in normal
		a:      &MapOf[any]{normal: map[interface{}]interface{}{"a": "b"}, length: 1},
		b:      NewMapOf[any](KV[any]{"a", false}),
		result: false,
	}, { // multiple entries
		a:      &MapOf[any]{normal: map[interface{}]interface{}{"a": 1, "b": true}, length: 2},
		b:      NewMapOf[any](KV[any]{"a", 1}, KV[any]{"b", true}),
		result: true,
	}, { // nested maps in values
		a: &MapOf[any]{
			normal: map[interface{}]interface{}{"a": map[string]interface{}{"b": 3}},
			length: 1},
		b:      NewMapOf[any](KV[any]{"a", map[string]interface{}{"b": 3}}),
		result: true,
	}, { // differing nested maps in values
		a: &MapOf[any]{
			normal: map[interface{}]interface{}{"a": map[string]interface{}{"b": 3}},
			length: 1},
		b:      NewMapOf[any](KV[any]{"a", map[string]interface{}{"b": 4}}),
		result: false,
	}, { // map with map as key
		a:      NewMapOf[any](KV[any]{New(map[string]interface{}{"a": 123}), "b"}),
		b:      NewMapOf[any](KV[any]{New(map[string]interface{}{"a": 123}), "b"}),
		result: true,
	}, {
		a:      NewMapOf[any](KV[any]{New(map[string]interface{}{"a": 123}), "a"}),
		b:      NewMapOf[any](KV[any]{New(map[string]interface{}{"a": 123}), "b"}),
		result: false,
	}, {
		a:      NewMapOf[any](KV[any]{New(map[string]interface{}{"a": 123}), "b"}),
		b:      NewMapOf[any](KV[any]{New(map[string]interface{}{"b": 123}), "b"}),
		result: false,
	}, {
		a:      NewMapOf[any](KV[any]{New(map[string]interface{}{"a": 1, "b": 2}), "c"}),
		b:      NewMapOf[any](KV[any]{New(map[string]interface{}{"a": 1, "b": 2}), "c"}),
		result: true,
	}, { // maps with keys that hash to same buckets in different order
		a: NewMapOf[any](
			KV[any]{dumbHashable{dumb: "hashable1"}, 1},
			KV[any]{dumbHashable{dumb: "hashable2"}, 2},
			KV[any]{dumbHashable{dumb: "hashable3"}, 3}),
		b: NewMapOf[any](
			KV[any]{dumbHashable{dumb: "hashable3"}, 3},
			KV[any]{dumbHashable{dumb: "hashable2"}, 2},
			KV[any]{dumbHashable{dumb: "hashable1"}, 1}),
		result: true,
	}, { // maps with map as value
		a: &MapOf[any]{normal: map[interface{}]interface{}{
			"foo": &MapOf[any]{normal: map[interface{}]interface{}{"a": 1}, length: 1}}, length: 1},
		b: &MapOf[any]{normal: map[interface{}]interface{}{
			"foo": &MapOf[any]{normal: map[interface{}]interface{}{"a": 1}, length: 1}}, length: 1},
		result: true,
	}}

	for _, tcase := range tests {
		if tcase.a.Equal(tcase.b) != tcase.result {
			t.Errorf("%v and %v are not equal", tcase.a, tcase.b)
		}
	}
}

func TestMapOfEntry(t *testing.T) {
	m := NewMapOf[any]()
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

func TestMapOfSetGet(t *testing.T) {
	m := MapOf[interface{}]{}
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

func TestMapOfDel(t *testing.T) {
	tests := []struct {
		m   *MapOf[interface{}]
		del interface{}
		exp *MapOf[interface{}]
	}{{
		m:   NewMapOf[interface{}](),
		del: "a",
		exp: NewMapOf[interface{}](),
	}, {
		m:   NewMapOf[interface{}](),
		del: New(map[string]interface{}{"a": 1}),
		exp: NewMapOf[interface{}](),
	}, {
		m:   NewMapOf(KV[interface{}]{"a", true}),
		del: "a",
		exp: NewMapOf[interface{}](),
	}, {
		m:   NewMapOf(KV[interface{}]{dumbHashable{dumb: "hashable1"}, 42}),
		del: dumbHashable{dumb: "hashable1"},
		exp: NewMapOf[interface{}](),
	}}

	for _, tcase := range tests {
		tcase.m.Del(tcase.del)
		if !tcase.m.Equal(tcase.exp) {
			t.Errorf("map %#v after del of element %v does not equal expected %#v",
				tcase.m, tcase.del, tcase.exp)
		}
	}
}

func TestMapOfIter(t *testing.T) {
	tests := []struct {
		m     *MapOf[any]
		elems []interface{}
	}{{
		m:     NewMapOf[any](),
		elems: []interface{}{},
	}, {
		m:     NewMapOf[any](KV[any]{"a", true}),
		elems: []interface{}{"a"},
	}, {
		m:     NewMapOf[any](KV[any]{dumbHashable{dumb: "hashable1"}, 42}),
		elems: []interface{}{dumbHashable{dumb: "hashable1"}},
	}, {
		m: NewMapOf[any](
			KV[any]{dumbHashable{dumb: "hashable2"}, 42},
			KV[any]{dumbHashable{dumb: "hashable3"}, 42},
			KV[any]{dumbHashable{dumb: "hashable4"}, 42}),
		elems: []interface{}{dumbHashable{dumb: "hashable2"},
			dumbHashable{dumb: "hashable3"}, dumbHashable{dumb: "hashable4"}},
	}, {
		m: NewMapOf[any](
			KV[any]{New(map[string]interface{}{"a": 123}), "b"},
			KV[any]{New(map[string]interface{}{"c": 456}), "d"},
			KV[any]{dumbHashable{dumb: "hashable1"}, 1},
			KV[any]{dumbHashable{dumb: "hashable2"}, 2},
			KV[any]{dumbHashable{dumb: "hashable3"}, 3},
			KV[any]{"x", true},
			KV[any]{"y", false},
			KV[any]{"z", nil},
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

func TestMapOfIterDel(t *testing.T) {
	// Deleting from standard go maps while iterating is safe. Since a MapOf contains maps,
	// deleting from a MapOf while iterating is also safe.
	m := NewMapOf[any](
		KV[any]{"1", "2"},
		KV[any]{New("1"), "keyVal"},
		KV[any]{New(map[string]interface{}{"key1": "val1", "key2": 2}), "mapVal"},
		KV[any]{dumbHashable{dumb: "dumbkey"}, "dumbHashVal"},
	)
	if err := m.Iter(func(k, v interface{}) error {
		m.Del(k)
		if _, ok := m.Get(k); ok {
			t.Errorf("key %v should not exist", k)
		}
		return nil
	}); err != nil {
		t.Error(err)
	}
	if m.Len() != 0 {
		t.Errorf("map elements should all be deleted, but found %d elements", m.Len())
	}
}

func TestMapOfKeys(t *testing.T) {
	m := NewMapOf[any](
		KV[any]{"1", "2"},
		KV[any]{New("1"), "keyVal"},
		KV[any]{New(map[string]interface{}{"key1": "val1", "key2": 2}), "mapVal"},
		KV[any]{dumbHashable{dumb: "dumbkey"}, "dumbHashVal"},
	)
	if len(m.Keys()) != m.Len() {
		t.Errorf("len(m.Keys()) %d != expected len(m) %d", len(m.Keys()), m.Len())
	}
	for _, key := range m.Keys() {
		if _, ok := m.Get(key); !ok {
			t.Errorf("could not find key %s in map m %s", key, m)
		}
	}
}

func TestMapOfValues(t *testing.T) {
	m := NewMapOf[any](
		KV[any]{"1", "2"},
		KV[any]{New("1"), "keyVal"},
		KV[any]{New(map[string]interface{}{"key1": "val1", "key2": 2}), "mapVal"},
		KV[any]{dumbHashable{dumb: "dumbkey"}, "dumbHashVal"},
	)
	if len(m.Values()) != m.Len() {
		t.Errorf("len(m.Values()) %d != expected len(m) %d", len(m.Values()), m.Len())
	}
	for _, value := range m.Values() {
		found := false
		if err := m.Iter(func(k, v interface{}) error {
			if v == value {
				found = true
				return errors.New("found")
			}
			return nil
		}); err != nil {
			if err.Error() == "found" {
				found = true
			}
		}
		if !found {
			t.Errorf("could not find value %s in map m %s", value, m)
		}
	}
}

func TestMapOfString(t *testing.T) {
	for _, tc := range []struct {
		m *MapOf[any]
		s string
	}{{
		m: NewMapOf[any](),
		s: "key.MapOf[]",
	}, {
		m: NewMapOf[any](KV[any]{"1", "2"}),
		s: "key.MapOf[1:2]",
	}, {
		m: NewMapOf[any](KV[any]{
			"3", "4"}, KV[any]{

			"1", "2"}),

		s: "key.MapOf[1:2 3:4]",
	}, {
		m: NewMapOf[any](KV[any]{
			New(map[string]interface{}{"key1": uint32(1), "key2": uint32(2)}), "foobar"}, KV[any]{

			New(map[string]interface{}{"key1": uint32(3), "key2": uint32(4)}), "bazquux"}),

		s: "key.MapOf[map[key1:1 key2:2]:foobar map[key1:3 key2:4]:bazquux]",
	}} {
		t.Run(tc.s, func(t *testing.T) {
			out := tc.m.String()
			if out != tc.s {
				t.Errorf("expected %q got %q", tc.s, out)
			}
		})
	}
}

func TestMapOfKeyString(t *testing.T) {
	for _, tc := range []struct {
		m *MapOf[any]
		s string
	}{{
		m: NewMapOf[any](
			KV[any]{New(uint32(42)), true},
			KV[any]{New("foo"), "bar"},
			KV[any]{New(map[string]interface{}{"hello": "world"}), "yolo"},
			KV[any]{New(map[string]interface{}{"key1": uint32(1), "key2": uint32(2)}), "foobar"}),

		s: "1_2=foobar_42=true_foo=bar_world=yolo",
	}} {
		t.Run(tc.s, func(t *testing.T) {
			out := tc.m.KeyString()
			if out != tc.s {
				t.Errorf("expected %q got %q", tc.s, out)
			}
		})
	}
}

func BenchmarkMapOfGrow(b *testing.B) {
	keys := make([]Key, 150)
	for j := 0; j < len(keys); j++ {
		keys[j] = New(map[string]interface{}{
			"foobar": 100,
			"baz":    j,
		})
	}
	v := "foobar"
	b.Run("key.MapOf[any]", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			m := NewMapOf[any]()
			for j := 0; j < len(keys); j++ {
				m.Set(keys[j], v)
			}
		}
	})
	b.Run("key.MapOf[string]", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			m := NewMapOf[string]()
			for j := 0; j < len(keys); j++ {
				m.Set(keys[j], v)
			}
		}
	})
}

func BenchmarkMapOfGet(b *testing.B) {
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
	v := "foobar"
	b.Run("key.MapOf[any]", func(b *testing.B) {
		m := NewMapOf[any]()
		for j := 0; j < len(keys); j++ {
			m.Set(keys[j], v)
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
	b.Run("key.MapOf[string]", func(b *testing.B) {
		m := NewMapOf[string]()
		for j := 0; j < len(keys); j++ {
			m.Set(keys[j], v)
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
