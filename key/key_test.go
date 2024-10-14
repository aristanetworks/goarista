// Copyright (c) 2015 Arista Networks, Inc.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the COPYING file.

package key_test

import (
	"encoding/json"
	"fmt"
	"hash/maphash"
	"strconv"
	"testing"

	. "github.com/aristanetworks/goarista/key"
	"github.com/aristanetworks/goarista/test"
	"github.com/aristanetworks/goarista/value"
)

type compareMe struct {
	i int
}

func (c compareMe) Equal(other interface{}) bool {
	o, ok := other.(compareMe)
	return ok && c == o
}

func (c compareMe) String() string {
	return fmt.Sprintf("compareMe{%d}", c.i)
}

type customKey struct {
	i int
}

var _ value.Value = customKey{}

func (c customKey) String() string {
	return fmt.Sprintf("customKey=%d", c.i)
}

func (c customKey) MarshalJSON() ([]byte, error) {
	return nil, nil
}

func (c customKey) ToBuiltin() interface{} {
	return c.i
}

var (
	nilIntf = interface{}(nil)
	nilMap  = map[string]interface{}(nil)
	nilArr  = []interface{}(nil)
	nilPath = Path(nil)

	nilPtr Pointer
	nilVal value.Value
)

func TestKeyEqual(t *testing.T) {
	tests := []struct {
		a      Key
		b      Key
		result bool
	}{{
		a:      New("foo"),
		b:      New("foo"),
		result: true,
	}, {
		a:      New("foo"),
		b:      New("bar"),
		result: false,
	}, {
		a:      New([]interface{}{}),
		b:      New("bar"),
		result: false,
	}, {
		a:      New([]interface{}{}),
		b:      New([]interface{}{}),
		result: true,
	}, {
		a:      New([]interface{}{"a", "b"}),
		b:      New([]interface{}{"a"}),
		result: false,
	}, {
		a:      New([]interface{}{"a", "b"}),
		b:      New([]interface{}{"b", "a"}),
		result: false,
	}, {
		a:      New([]interface{}{"a", "b"}),
		b:      New([]interface{}{"a", "b"}),
		result: true,
	}, {
		a:      New([]interface{}{"a", map[string]interface{}{"b": "c"}}),
		b:      New([]interface{}{"a", map[string]interface{}{"c": "b"}}),
		result: false,
	}, {
		a:      New([]interface{}{"a", map[string]interface{}{"b": "c"}}),
		b:      New([]interface{}{"a", map[string]interface{}{"b": "c"}}),
		result: true,
	}, {
		a:      New(map[string]interface{}{}),
		b:      New("bar"),
		result: false,
	}, {
		a:      New(map[string]interface{}{}),
		b:      New(map[string]interface{}{}),
		result: true,
	}, {
		a:      New(map[string]interface{}{"a": uint32(3)}),
		b:      New(map[string]interface{}{}),
		result: false,
	}, {
		a:      New(map[string]interface{}{"a": uint32(3)}),
		b:      New(map[string]interface{}{"b": uint32(4)}),
		result: false,
	}, {
		a:      New(map[string]interface{}{"a": uint32(4), "b": uint32(5)}),
		b:      New(map[string]interface{}{"a": uint32(4)}),
		result: false,
	}, {
		a:      New(map[string]interface{}{"a": uint32(3)}),
		b:      New(map[string]interface{}{"a": uint32(4)}),
		result: false,
	}, {
		a:      New(map[string]interface{}{"a": uint32(3)}),
		b:      New(map[string]interface{}{"a": uint32(3)}),
		result: true,
	}, {
		a:      New(map[string]interface{}{"a": NewMap(New("b"), uint32(3))}),
		b:      New(map[string]interface{}{"a": NewMap(New("b"), uint32(4))}),
		result: false,
	}, {
		a:      New(map[string]interface{}{"a": NewMap(New("b"), uint32(4), New("c"), uint32(5))}),
		b:      New(map[string]interface{}{"a": NewMap(New("b"), uint32(4))}),
		result: false,
	}, {
		a:      New(map[string]interface{}{"a": NewMap(New("b"), uint32(4), New("c"), uint32(5))}),
		b:      New(map[string]interface{}{"a": NewMap(New("b"), uint32(4), New("c"), uint32(5))}),
		result: true,
	}, {
		a:      New(map[string]interface{}{"a": NewMap(New("b"), uint32(4))}),
		b:      New(map[string]interface{}{"a": NewMap(New("b"), uint32(4))}),
		result: true,
	}, {
		a:      New(map[string]interface{}{"a": compareMe{i: 3}}),
		b:      New(map[string]interface{}{"a": compareMe{i: 3}}),
		result: true,
	}, {
		a:      New(map[string]interface{}{"a": compareMe{i: 3}}),
		b:      New(map[string]interface{}{"a": compareMe{i: 4}}),
		result: false,
	}, {
		a:      New(customKey{i: 42}),
		b:      New(customKey{i: 42}),
		result: true,
	}, {
		a:      New(nil),
		b:      New(nil),
		result: true,
	}, {
		a:      New(nil),
		b:      New(nilIntf),
		result: true,
	}, {
		a:      New(nil),
		b:      New(nilPtr),
		result: true,
	}, {
		a:      New(nil),
		b:      New(nilVal),
		result: true,
	}, {
		a:      New(nil),
		b:      New(nilMap),
		result: false,
	}, {
		a:      New(nilMap),
		b:      New(map[string]interface{}{}),
		result: true,
	}, {
		a:      New(nil),
		b:      New(nilArr),
		result: false,
	}, {
		a:      New(nilArr),
		b:      New([]interface{}{}),
		result: true,
	}, {
		a:      New(nil),
		b:      New(nilPath),
		result: false,
	}, {
		a:      New(nilPath),
		b:      New(Path{}),
		result: true,
	}, {
		a:      New([]byte{0x0}),
		b:      New([]byte{}),
		result: false,
	}, {
		a:      New([]byte{0x1, 0x2}),
		b:      New([]byte{0x1, 0x2}),
		result: true,
	}, {
		a:      New([]byte{0x2, 0x1}),
		b:      New([]byte{0x1, 0x2}),
		result: false,
	}, {
		a:      New(string([]byte{0x1, 0x2})),
		b:      New([]byte{0x1, 0x2}),
		result: false,
	}, {
		a:      New(map[string]interface{}{"key1": []byte{0x1}}),
		b:      New(map[string]interface{}{"key1": []byte{0x1}}),
		result: true,
	}, {
		a:      New(map[string]interface{}{"key1": []byte{0x1}}),
		b:      New(map[string]interface{}{"key1": []byte{0x2}}),
		result: false,
	}}

	for _, tcase := range tests {
		t.Run(fmt.Sprintf("%s_%s", tcase.a.String(), tcase.b.String()), func(t *testing.T) {
			if tcase.a.Equal(tcase.b) != tcase.result {
				t.Errorf("Wrong result for case:\na: %#v\nb: %#v\nresult: %#v",
					tcase.a,
					tcase.b,
					tcase.result)
			}
			seed := maphash.MakeSeed()
			aHash := Hash(seed, tcase.a)
			bHash := Hash(seed, tcase.b)
			if tcase.result {
				if aHash != bHash {
					t.Errorf("Equal keys have different hash: %x vs. %x", aHash, bHash)
				}
			} else {
				if aHash == bHash {
					// This should be very unlikely
					t.Logf("Unequal keys have the same hash: %x", aHash)
				}
			}
		})
	}

	if New("a").Equal(32) {
		t.Error("Wrong result for different types case")
	}
}

func TestGetFromMap(t *testing.T) {
	tests := []struct {
		k     Key
		m     *Map
		v     interface{}
		found bool
	}{{
		k:     New(nil),
		m:     NewMap(New(nil), nil),
		v:     nil,
		found: true,
	}, {
		k:     New("a"),
		m:     NewMap(New("a"), "b"),
		v:     "b",
		found: true,
	}, {
		k:     New(uint32(35)),
		m:     NewMap(New(uint32(35)), "c"),
		v:     "c",
		found: true,
	}, {
		k:     New(uint32(37)),
		m:     NewMap(New(uint32(36)), "c"),
		found: false,
	}, {
		k:     New(uint32(37)),
		m:     NewMap(),
		found: false,
	}, {
		k: New([]interface{}{"a", "b"}),
		m: NewMap(New([]interface{}{"a", "b"}), "foo"),

		v:     "foo",
		found: true,
	}, {
		k: New([]interface{}{"a", "b"}),
		m: NewMap(New([]interface{}{"a", "b", "c"}), "foo"),

		found: false,
	}, {
		k: New([]interface{}{"a", map[string]interface{}{"b": "c"}}),
		m: NewMap(New([]interface{}{"a", map[string]interface{}{"b": "c"}}), "foo"),

		v:     "foo",
		found: true,
	}, {
		k: New([]interface{}{"a", map[string]interface{}{"b": "c"}}),
		m: NewMap(New([]interface{}{"a", map[string]interface{}{"c": "b"}}), "foo"),

		found: false,
	}, {
		k: New(map[string]interface{}{"a": "b", "c": uint64(4)}),
		m: NewMap(New(map[string]interface{}{"a": "b", "c": uint64(4)}), "foo"),

		v:     "foo",
		found: true,
	}, {
		k: New(map[string]interface{}{"a": "b", "c": uint64(4)}),
		m: NewMap(New(map[string]interface{}{"a": "b", "c": uint64(5)}), "foo"),

		found: false,
	}, {
		k:     New(customKey{i: 42}),
		m:     NewMap(New(customKey{i: 42}), "c"),
		v:     "c",
		found: true,
	}, {
		k:     New(customKey{i: 42}),
		m:     NewMap(New(customKey{i: 43}), "c"),
		found: false,
	}, {
		k: New(map[string]interface{}{
			"damn": NewMap(New(map[string]interface{}{"a": uint32(42),
				"b": uint32(51)}), true)}),
		m: NewMap(New(map[string]interface{}{
			"damn": NewMap(New(map[string]interface{}{"a": uint32(42),
				"b": uint32(51)}), true)}), "foo"),

		v:     "foo",
		found: true,
	}, {
		k: New(map[string]interface{}{
			"damn": NewMap(New(map[string]interface{}{"a": uint32(42),
				"b": uint32(52)}), true)}),
		m: NewMap(New(map[string]interface{}{
			"damn": NewMap(New(map[string]interface{}{"a": uint32(42),
				"b": uint32(51)}), true)}), "foo"),

		found: false,
	}, {
		k: New(map[string]interface{}{
			"nested": map[string]interface{}{
				"a": uint32(42), "b": uint32(51)}}),
		m: NewMap(New(map[string]interface{}{
			"nested": map[string]interface{}{
				"a": uint32(42), "b": uint32(51)}}), "foo"),

		v:     "foo",
		found: true,
	}, {
		k: New(map[string]interface{}{
			"nested": map[string]interface{}{
				"a": uint32(42), "b": uint32(52)}}),
		m: NewMap(New(map[string]interface{}{
			"nested": map[string]interface{}{
				"a": uint32(42), "b": uint32(51)}}), "foo"),

		found: false,
	}, {
		k: New(map[string]interface{}{
			"nested": []byte{0x1, 0x2},
		}),
		m: NewMap(New(map[string]interface{}{
			"nested": []byte{0x1, 0x2},
		}), "foo"),
		v:     "foo",
		found: true,
	}, {
		k: New(map[string]interface{}{
			"nested": []byte{0x1, 0x2},
		}),
		m: NewMap(New(map[string]interface{}{
			"nested": []byte{0x1, 0x3},
		}), "foo"),
		found: false,
	}}

	for _, tcase := range tests {
		v, ok := tcase.m.Get(tcase.k)
		if tcase.found != ok {
			t.Errorf("Wrong retrieval result for case:\nk: %#v\nm: %#v\nv: %#v",
				tcase.k,
				tcase.m,
				tcase.v)
		} else if tcase.found && !ok {
			t.Errorf("Unable to retrieve value for case:\nk: %#v\nm: %#v\nv: %#v",
				tcase.k,
				tcase.m,
				tcase.v)
		} else if tcase.found && !test.DeepEqual(tcase.v, v) {
			t.Errorf("Wrong result for case:\nk: %#v\nm: %#v\nv: %#v",
				tcase.k,
				tcase.m,
				tcase.v)
		}
	}
}

func TestDeleteFromMap(t *testing.T) {
	tests := []struct {
		k Key
		m *Map
		r *Map
	}{{
		k: New("a"),
		m: NewMap(New("a"), "b"),
		r: NewMap(),
	}, {
		k: New("b"),
		m: NewMap(New("a"), "b"),
		r: NewMap(New("a"), "b"),
	}, {
		k: New("a"),
		m: NewMap(),
		r: NewMap(),
	}, {
		k: New(uint32(35)),
		m: NewMap(New(uint32(35)), "c"),
		r: NewMap(),
	}, {
		k: New(uint32(36)),
		m: NewMap(New(uint32(35)), "c"),
		r: NewMap(New(uint32(35)), "c"),
	}, {
		k: New(uint32(37)),
		m: NewMap(),
		r: NewMap(),
	}, {
		k: New(map[string]interface{}{"a": "b", "c": uint64(4)}),
		m: NewMap(New(map[string]interface{}{"a": "b", "c": uint64(4)}), "foo"),

		r: NewMap(),
	}, {
		k: New(customKey{i: 42}),
		m: NewMap(New(customKey{i: 42}), "c"),
		r: NewMap(),
	}, {
		k: New([]byte{0x1, 0x2}),
		m: NewMap(New([]byte{0x1, 0x2}), "a", New([]byte{0x1}), "b"),
		r: NewMap(New([]byte{0x1}), "b"),
	}}

	for _, tcase := range tests {
		tcase.m.Del(tcase.k)
		if !test.DeepEqual(tcase.m, tcase.r) {
			t.Errorf("Wrong result for case:\nk: %#v\nm: %#v\nr: %#v",
				tcase.k,
				tcase.m,
				tcase.r)
		}
	}
}

func TestSetToMap(t *testing.T) {
	tests := []struct {
		k Key
		v interface{}
		m *Map
		r *Map
	}{{
		k: New("a"),
		v: "c",
		m: NewMap(New("a"), "b"),
		r: NewMap(New("a"), "c"),
	}, {
		k: New("b"),
		v: uint64(56),
		m: NewMap(New("a"), "b"),
		r: NewMap(New("a"), "b",
			New("b"), uint64(56)),
	}, {
		k: New("a"),
		v: "foo",
		m: NewMap(),
		r: NewMap(New("a"), "foo"),
	}, {
		k: New(uint32(35)),
		v: "d",
		m: NewMap(New(uint32(35)), "c"),
		r: NewMap(New(uint32(35)), "d"),
	}, {
		k: New(uint32(36)),
		v: true,
		m: NewMap(New(uint32(35)), "c"),
		r: NewMap(New(uint32(35)), "c",
			New(uint32(36)), true),
	}, {
		k: New(uint32(37)),
		v: false,
		m: NewMap(New(uint32(36)), "c"),
		r: NewMap(New(uint32(36)), "c",
			New(uint32(37)), false),
	}, {
		k: New(uint32(37)),
		v: "foobar",
		m: NewMap(),
		r: NewMap(New(uint32(37)), "foobar"),
	}, {
		k: New(map[string]interface{}{"a": "b", "c": uint64(4)}),
		v: "foobar",
		m: NewMap(New(map[string]interface{}{"a": "b", "c": uint64(4)}), "foo"),

		r: NewMap(New(map[string]interface{}{"a": "b", "c": uint64(4)}), "foobar"),
	}, {
		k: New(map[string]interface{}{"a": "b", "c": uint64(7)}),
		v: "foobar",
		m: NewMap(New(map[string]interface{}{"a": "b", "c": uint64(4)}), "foo"),

		r: NewMap(New(map[string]interface{}{"a": "b", "c": uint64(4)}), "foo",
			New(map[string]interface{}{"a": "b", "c": uint64(7)}), "foobar"),
	}, {
		k: New(map[string]interface{}{"a": "b", "d": uint64(6)}),
		v: "barfoo",
		m: NewMap(New(map[string]interface{}{"a": "b", "c": uint64(4)}), "foo"),

		r: NewMap(New(map[string]interface{}{"a": "b", "c": uint64(4)}), "foo",
			New(map[string]interface{}{"a": "b", "d": uint64(6)}), "barfoo"),
	}, {
		k: New(customKey{i: 42}),
		v: "foo",
		m: NewMap(),
		r: NewMap(New(customKey{i: 42}), "foo"),
	}}

	for i, tcase := range tests {
		tcase.m.Set(tcase.k, tcase.v)
		if !test.DeepEqual(tcase.m, tcase.r) {
			t.Errorf("Wrong result for case %d:\nk: %#v\nm: %#v\nr: %#v",
				i,
				tcase.k,
				tcase.m,
				tcase.r)
		}
	}
}

func TestGoString(t *testing.T) {
	tcases := []struct {
		in  Key
		out string
	}{{
		in:  New(nil),
		out: "key.New(nil)",
	}, {
		in:  New(uint8(1)),
		out: "key.New(uint8(1))",
	}, {
		in:  New(uint16(1)),
		out: "key.New(uint16(1))",
	}, {
		in:  New(uint32(1)),
		out: "key.New(uint32(1))",
	}, {
		in:  New(uint64(1)),
		out: "key.New(uint64(1))",
	}, {
		in:  New(int8(1)),
		out: "key.New(int8(1))",
	}, {
		in:  New(int16(1)),
		out: "key.New(int16(1))",
	}, {
		in:  New(int32(1)),
		out: "key.New(int32(1))",
	}, {
		in:  New(int64(1)),
		out: "key.New(int64(1))",
	}, {
		in:  New(float32(1)),
		out: "key.New(float32(1))",
	}, {
		in:  New(float64(1)),
		out: "key.New(float64(1))",
	}, {
		in:  New(map[string]interface{}{"foo": true}),
		out: `key.New(map[string]interface {}{"foo":true})`,
	}}
	for i, tcase := range tcases {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			if out := fmt.Sprintf("%#v", tcase.in); out != tcase.out {
				t.Errorf("Wanted Go representation %q but got %q", tcase.out, out)
			}
		})
	}
}

func TestMisc(t *testing.T) {
	k := New(map[string]interface{}{"foo": true})
	js, err := json.Marshal(k)
	if err != nil {
		t.Error("JSON encoding failed:", err)
	} else if expected := `{"foo":true}`; string(js) != expected {
		t.Errorf("Wanted JSON %q but got %q", expected, js)
	}

	test.ShouldPanic(t, func() { New(42) })

	k = New(customKey{i: 42})
	if expected, str := "customKey=42", k.String(); expected != str {
		t.Errorf("Wanted string representation %q but got %q", expected, str)
	}
}

func TestTryNew(t *testing.T) {
	k, err := TryNew(42)
	if k != nil {
		t.Error("expected nil key for unsupported type")
	}
	if err == nil {
		t.Error("expected error unsupported type")
	}
	k, err = TryNew("foo")
	if err != nil {
		t.Errorf("expected no error but got %v", err)
	} else if expected, str := "foo", k.String(); expected != str {
		t.Errorf("Wanted string representation %q but got %q", expected, str)
	}
}

func BenchmarkSetToMapWithStringKey(b *testing.B) {
	m := NewMap(New("a"), true,
		New("a1"), true,
		New("a2"), true,
		New("a3"), true,
		New("a4"), true,
		New("a5"), true,
		New("a6"), true,
		New("a7"), true,
		New("a8"), true,
		New("a9"), true,
		New("a10"), true,
		New("a11"), true,
		New("a12"), true,
		New("a13"), true,
		New("a14"), true,
		New("a15"), true,
		New("a16"), true,
		New("a17"), true,
		New("a18"), true)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		m.Set(New(strconv.Itoa(i)), true)
	}
}

func BenchmarkSetToMapWithUint64Key(b *testing.B) {
	m := NewMap(New(uint64(1)), true,
		New(uint64(2)), true,
		New(uint64(3)), true,
		New(uint64(4)), true,
		New(uint64(5)), true,
		New(uint64(6)), true,
		New(uint64(7)), true,
		New(uint64(8)), true,
		New(uint64(9)), true,
		New(uint64(10)), true,
		New(uint64(11)), true,
		New(uint64(12)), true,
		New(uint64(13)), true,
		New(uint64(14)), true,
		New(uint64(15)), true,
		New(uint64(16)), true,
		New(uint64(17)), true,
		New(uint64(18)), true,
		New(uint64(19)), true)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		m.Set(New(uint64(i)), true)
	}
}

func BenchmarkGetFromMapWithMapKey(b *testing.B) {
	m := NewMap(New(map[string]interface{}{"a": true}), true,
		New(map[string]interface{}{"b": true}), true,
		New(map[string]interface{}{"c": true}), true,
		New(map[string]interface{}{"d": true}), true,
		New(map[string]interface{}{"e": true}), true,
		New(map[string]interface{}{"f": true}), true,
		New(map[string]interface{}{"g": true}), true,
		New(map[string]interface{}{"h": true}), true,
		New(map[string]interface{}{"i": true}), true,
		New(map[string]interface{}{"j": true}), true,
		New(map[string]interface{}{"k": true}), true,
		New(map[string]interface{}{"l": true}), true,
		New(map[string]interface{}{"m": true}), true,
		New(map[string]interface{}{"n": true}), true,
		New(map[string]interface{}{"o": true}), true,
		New(map[string]interface{}{"p": true}), true,
		New(map[string]interface{}{"q": true}), true,
		New(map[string]interface{}{"r": true}), true,
		New(map[string]interface{}{"s": true}), true)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		key := New(map[string]interface{}{string(rune('a' + i%19)): true})
		_, found := m.Get(key)
		if !found {
			b.Fatalf("WTF: %#v", key)
		}
	}
}

func mkKey(i int) Key {
	return New(map[string]interface{}{
		"foo": map[string]interface{}{
			"aaaa1": uint32(0),
			"aaaa2": uint32(0),
			"aaaa3": uint32(i),
		},
		"bar": map[string]interface{}{
			"nested": uint32(42),
		},
	})
}

func BenchmarkBigMapWithCompositeKeys(b *testing.B) {
	const size = 10000
	m := NewMap()
	for i := 0; i < size; i++ {
		m.Set(mkKey(i), true)
	}
	k := mkKey(0)
	submap := k.Key().(map[string]interface{})["foo"].(map[string]interface{})
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		submap["aaaa3"] = uint32(i)
		_, found := m.Get(k)
		if found != (i < size) {
			b.Fatalf("WTF: %#v", k)
		}
	}
}

func BenchmarkKeyTypes(b *testing.B) {
	benches := []struct {
		val interface{}
	}{
		{
			val: "foo",
		},
		{
			val: int8(-12),
		},
		{
			val: int16(123),
		},
		{
			val: int32(123),
		},
		{
			val: int64(123456),
		},
		{
			val: uint8(12),
		},
		{
			val: uint16(123),
		},
		{
			val: uint32(123),
		},
		{
			val: uint64(123456),
		},
		{
			val: float32(123456.12),
		},
		{
			val: float64(123456.12),
		},
		{
			val: true,
		},
		{
			val: map[string]interface{}{"foo": uint32(42), "bar": uint32(42), "baz": uint32(42)},
		},
		{
			val: []interface{}{"foo", "bar", "baz"},
		},
	}

	for _, bench := range benches {
		var k Key
		b.Run(fmt.Sprintf("%T", bench.val), func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				// create the key.Key and call some function here
				if k = New(bench.val); k == nil {
					b.Fatalf("expect to get key.Key, but got nil")
				}
				if !k.Equal(New(bench.val)) {
					b.Fatalf("k is not equal to itself: %v", bench.val)
				}
			}
		})
	}
}
