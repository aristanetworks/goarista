// Copyright (c) 2020 Arista Networks, Inc.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the COPYING file.

// +build !go1.14

package key_test

import (
	"fmt"
	"strconv"
	"testing"

	"github.com/aristanetworks/goarista/key"
	. "github.com/aristanetworks/goarista/key"
	"github.com/aristanetworks/goarista/path"
	"github.com/aristanetworks/goarista/test"
)

func TestMkiKeyEqual(t *testing.T) {
	tests := []struct {
		a      Key
		b      Key
		result bool
	}{{
		a:      New(map[string]interface{}{"a": map[Key]interface{}{New("b"): 3}}),
		b:      New(map[string]interface{}{"a": map[Key]interface{}{New("b"): 4}}),
		result: false,
	}, {
		a:      New(map[string]interface{}{"a": map[Key]interface{}{New("b"): 4, New("c"): 5}}),
		b:      New(map[string]interface{}{"a": map[Key]interface{}{New("b"): 4}}),
		result: false,
	}, {
		a:      New(map[string]interface{}{"a": map[Key]interface{}{New("b"): 4, New("c"): 5}}),
		b:      New(map[string]interface{}{"a": map[Key]interface{}{New("b"): 4, New("c"): 5}}),
		result: true,
	}, {
		a:      New(map[string]interface{}{"a": map[Key]interface{}{New("b"): 4}}),
		b:      New(map[string]interface{}{"a": map[Key]interface{}{New("b"): 4}}),
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

	if New("a").Equal(32) {
		t.Error("Wrong result for different types case")
	}
}

func TestMkiGetFromMap(t *testing.T) {
	tests := []struct {
		k     Key
		m     map[Key]interface{}
		v     interface{}
		found bool
	}{{
		k:     New(nil),
		m:     map[Key]interface{}{New(nil): nil},
		v:     nil,
		found: true,
	}, {
		k:     New("a"),
		m:     map[Key]interface{}{New("a"): "b"},
		v:     "b",
		found: true,
	}, {
		k:     New(uint32(35)),
		m:     map[Key]interface{}{New(uint32(35)): "c"},
		v:     "c",
		found: true,
	}, {
		k:     New(uint32(37)),
		m:     map[Key]interface{}{New(uint32(36)): "c"},
		found: false,
	}, {
		k:     New(uint32(37)),
		m:     map[Key]interface{}{},
		found: false,
	}, {
		k: New([]interface{}{"a", "b"}),
		m: map[Key]interface{}{
			New([]interface{}{"a", "b"}): "foo",
		},
		v:     "foo",
		found: true,
	}, {
		k: New([]interface{}{"a", "b"}),
		m: map[Key]interface{}{
			New([]interface{}{"a", "b", "c"}): "foo",
		},
		found: false,
	}, {
		k: New([]interface{}{"a", map[string]interface{}{"b": "c"}}),
		m: map[Key]interface{}{
			New([]interface{}{"a", map[string]interface{}{"b": "c"}}): "foo",
		},
		v:     "foo",
		found: true,
	}, {
		k: New([]interface{}{"a", map[string]interface{}{"b": "c"}}),
		m: map[Key]interface{}{
			New([]interface{}{"a", map[string]interface{}{"c": "b"}}): "foo",
		},
		found: false,
	}, {
		k: New(map[string]interface{}{"a": "b", "c": uint64(4)}),
		m: map[Key]interface{}{
			New(map[string]interface{}{"a": "b", "c": uint64(4)}): "foo",
		},
		v:     "foo",
		found: true,
	}, {
		k: New(map[string]interface{}{"a": "b", "c": uint64(4)}),
		m: map[Key]interface{}{
			New(map[string]interface{}{"a": "b", "c": uint64(5)}): "foo",
		},
		found: false,
	}, {
		k:     New(customKey{i: 42}),
		m:     map[Key]interface{}{New(customKey{i: 42}): "c"},
		v:     "c",
		found: true,
	}, {
		k:     New(customKey{i: 42}),
		m:     map[Key]interface{}{New(customKey{i: 43}): "c"},
		found: false,
	}, {
		k: New(map[string]interface{}{
			"damn": map[Key]interface{}{
				New(map[string]interface{}{"a": uint32(42),
					"b": uint32(51)}): true}}),
		m: map[Key]interface{}{
			New(map[string]interface{}{
				"damn": map[Key]interface{}{
					New(map[string]interface{}{"a": uint32(42),
						"b": uint32(51)}): true}}): "foo",
		},
		v:     "foo",
		found: true,
	}, {
		k: New(map[string]interface{}{
			"damn": map[Key]interface{}{
				New(map[string]interface{}{"a": uint32(42),
					"b": uint32(52)}): true}}),
		m: map[Key]interface{}{
			New(map[string]interface{}{
				"damn": map[Key]interface{}{
					New(map[string]interface{}{"a": uint32(42),
						"b": uint32(51)}): true}}): "foo",
		},
		found: false,
	}, {
		k: New(map[string]interface{}{
			"nested": map[string]interface{}{
				"a": uint32(42), "b": uint32(51)}}),
		m: map[Key]interface{}{
			New(map[string]interface{}{
				"nested": map[string]interface{}{
					"a": uint32(42), "b": uint32(51)}}): "foo",
		},
		v:     "foo",
		found: true,
	}, {
		k: New(map[string]interface{}{
			"nested": map[string]interface{}{
				"a": uint32(42), "b": uint32(52)}}),
		m: map[Key]interface{}{
			New(map[string]interface{}{
				"nested": map[string]interface{}{
					"a": uint32(42), "b": uint32(51)}}): "foo",
		},
		found: false,
	}}

	for _, tcase := range tests {
		v, ok := tcase.m[tcase.k]
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

func TestMkiDeleteFromMap(t *testing.T) {
	tests := []struct {
		k Key
		m map[Key]interface{}
		r map[Key]interface{}
	}{{
		k: New("a"),
		m: map[Key]interface{}{New("a"): "b"},
		r: map[Key]interface{}{},
	}, {
		k: New("b"),
		m: map[Key]interface{}{New("a"): "b"},
		r: map[Key]interface{}{New("a"): "b"},
	}, {
		k: New("a"),
		m: map[Key]interface{}{},
		r: map[Key]interface{}{},
	}, {
		k: New(uint32(35)),
		m: map[Key]interface{}{New(uint32(35)): "c"},
		r: map[Key]interface{}{},
	}, {
		k: New(uint32(36)),
		m: map[Key]interface{}{New(uint32(35)): "c"},
		r: map[Key]interface{}{New(uint32(35)): "c"},
	}, {
		k: New(uint32(37)),
		m: map[Key]interface{}{},
		r: map[Key]interface{}{},
	}, {
		k: New(map[string]interface{}{"a": "b", "c": uint64(4)}),
		m: map[Key]interface{}{
			New(map[string]interface{}{"a": "b", "c": uint64(4)}): "foo",
		},
		r: map[Key]interface{}{},
	}, {
		k: New(customKey{i: 42}),
		m: map[Key]interface{}{New(customKey{i: 42}): "c"},
		r: map[Key]interface{}{},
	}, {
		k: New([]byte{0x1, 0x2}),
		m: map[Key]interface{}{New([]byte{0x1, 0x2}): "a", New([]byte{0x1}): "b"},
		r: map[Key]interface{}{New([]byte{0x1}): "b"},
	}}

	for _, tcase := range tests {
		delete(tcase.m, tcase.k)
		if !test.DeepEqual(tcase.m, tcase.r) {
			t.Errorf("Wrong result for case:\nk: %#v\nm: %#v\nr: %#v",
				tcase.k,
				tcase.m,
				tcase.r)
		}
	}
}

func TestMkiSetToMap(t *testing.T) {
	tests := []struct {
		k Key
		v interface{}
		m map[Key]interface{}
		r map[Key]interface{}
	}{{
		k: New("a"),
		v: "c",
		m: map[Key]interface{}{New("a"): "b"},
		r: map[Key]interface{}{New("a"): "c"},
	}, {
		k: New("b"),
		v: uint64(56),
		m: map[Key]interface{}{New("a"): "b"},
		r: map[Key]interface{}{
			New("a"): "b",
			New("b"): uint64(56),
		},
	}, {
		k: New("a"),
		v: "foo",
		m: map[Key]interface{}{},
		r: map[Key]interface{}{New("a"): "foo"},
	}, {
		k: New(uint32(35)),
		v: "d",
		m: map[Key]interface{}{New(uint32(35)): "c"},
		r: map[Key]interface{}{New(uint32(35)): "d"},
	}, {
		k: New(uint32(36)),
		v: true,
		m: map[Key]interface{}{New(uint32(35)): "c"},
		r: map[Key]interface{}{
			New(uint32(35)): "c",
			New(uint32(36)): true,
		},
	}, {
		k: New(uint32(37)),
		v: false,
		m: map[Key]interface{}{New(uint32(36)): "c"},
		r: map[Key]interface{}{
			New(uint32(36)): "c",
			New(uint32(37)): false,
		},
	}, {
		k: New(uint32(37)),
		v: "foobar",
		m: map[Key]interface{}{},
		r: map[Key]interface{}{New(uint32(37)): "foobar"},
	}, {
		k: New(map[string]interface{}{"a": "b", "c": uint64(4)}),
		v: "foobar",
		m: map[Key]interface{}{
			New(map[string]interface{}{"a": "b", "c": uint64(4)}): "foo",
		},
		r: map[Key]interface{}{
			New(map[string]interface{}{"a": "b", "c": uint64(4)}): "foobar",
		},
	}, {
		k: New(map[string]interface{}{"a": "b", "c": uint64(7)}),
		v: "foobar",
		m: map[Key]interface{}{
			New(map[string]interface{}{"a": "b", "c": uint64(4)}): "foo",
		},
		r: map[Key]interface{}{
			New(map[string]interface{}{"a": "b", "c": uint64(4)}): "foo",
			New(map[string]interface{}{"a": "b", "c": uint64(7)}): "foobar",
		},
	}, {
		k: New(map[string]interface{}{"a": "b", "d": uint64(6)}),
		v: "barfoo",
		m: map[Key]interface{}{
			New(map[string]interface{}{"a": "b", "c": uint64(4)}): "foo",
		},
		r: map[Key]interface{}{
			New(map[string]interface{}{"a": "b", "c": uint64(4)}): "foo",
			New(map[string]interface{}{"a": "b", "d": uint64(6)}): "barfoo",
		},
	}, {
		k: New(customKey{i: 42}),
		v: "foo",
		m: map[Key]interface{}{},
		r: map[Key]interface{}{New(customKey{i: 42}): "foo"},
	}}

	for i, tcase := range tests {
		tcase.m[tcase.k] = tcase.v
		if !test.DeepEqual(tcase.m, tcase.r) {
			t.Errorf("Wrong result for case %d:\nk: %#v\nm: %#v\nr: %#v",
				i,
				tcase.k,
				tcase.m,
				tcase.r)
		}
	}
}

func BenchmarkMkiSetToMapWithStringKey(b *testing.B) {
	m := map[Key]interface{}{
		New("a"):   true,
		New("a1"):  true,
		New("a2"):  true,
		New("a3"):  true,
		New("a4"):  true,
		New("a5"):  true,
		New("a6"):  true,
		New("a7"):  true,
		New("a8"):  true,
		New("a9"):  true,
		New("a10"): true,
		New("a11"): true,
		New("a12"): true,
		New("a13"): true,
		New("a14"): true,
		New("a15"): true,
		New("a16"): true,
		New("a17"): true,
		New("a18"): true,
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		m[New(strconv.Itoa(i))] = true
	}
}

func BenchmarkMkiSetToMapWithUint64Key(b *testing.B) {
	m := map[Key]interface{}{
		New(uint64(1)):  true,
		New(uint64(2)):  true,
		New(uint64(3)):  true,
		New(uint64(4)):  true,
		New(uint64(5)):  true,
		New(uint64(6)):  true,
		New(uint64(7)):  true,
		New(uint64(8)):  true,
		New(uint64(9)):  true,
		New(uint64(10)): true,
		New(uint64(11)): true,
		New(uint64(12)): true,
		New(uint64(13)): true,
		New(uint64(14)): true,
		New(uint64(15)): true,
		New(uint64(16)): true,
		New(uint64(17)): true,
		New(uint64(18)): true,
		New(uint64(19)): true,
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		m[New(uint64(i))] = true
	}
}

func BenchmarkMkiGetFromMapWithMapKey(b *testing.B) {
	m := map[Key]interface{}{
		New(map[string]interface{}{"a": true}): true,
		New(map[string]interface{}{"b": true}): true,
		New(map[string]interface{}{"c": true}): true,
		New(map[string]interface{}{"d": true}): true,
		New(map[string]interface{}{"e": true}): true,
		New(map[string]interface{}{"f": true}): true,
		New(map[string]interface{}{"g": true}): true,
		New(map[string]interface{}{"h": true}): true,
		New(map[string]interface{}{"i": true}): true,
		New(map[string]interface{}{"j": true}): true,
		New(map[string]interface{}{"k": true}): true,
		New(map[string]interface{}{"l": true}): true,
		New(map[string]interface{}{"m": true}): true,
		New(map[string]interface{}{"n": true}): true,
		New(map[string]interface{}{"o": true}): true,
		New(map[string]interface{}{"p": true}): true,
		New(map[string]interface{}{"q": true}): true,
		New(map[string]interface{}{"r": true}): true,
		New(map[string]interface{}{"s": true}): true,
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		key := New(map[string]interface{}{string('a' + i%19): true})
		_, found := m[key]
		if !found {
			b.Fatalf("WTF: %#v", key)
		}
	}
}

func BenchmarkMkiBigMapWithCompositeKeys(b *testing.B) {
	const size = 10000
	m := make(map[Key]interface{}, size)
	for i := 0; i < size; i++ {
		m[mkKey(i)] = true
	}
	k := mkKey(0)
	submap := k.Key().(map[string]interface{})["foo"].(map[string]interface{})
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		submap["aaaa3"] = uint32(i)
		_, found := m[k]
		if found != (i < size) {
			b.Fatalf("WTF: %#v", k)
		}
	}
}

func TestMkiPathAsKey(t *testing.T) {
	a := newPathKey("foo", path.Wildcard, map[string]interface{}{
		"bar": map[Key]interface{}{
			// Should be able to embed a path key and value
			newPathKey("path", "to", "something"): path.New("else"),
		},
	})
	m := map[Key]string{
		a: "thats a complex key!",
	}
	if s, ok := m[a]; !ok {
		t.Error("complex key not found in map")
	} else if s != "thats a complex key!" {
		t.Errorf("incorrect value in map: %s", s)
	}

	// preserve custom path implementations
	b := key.New(customPath("/foo/bar"))
	if _, ok := b.Key().(customPath); !ok {
		t.Errorf("customPath implementation not preserved: %T", b.Key())
	}
}

func TestMkiPointerAsKey(t *testing.T) {
	a := key.NewPointer(path.New("foo", path.Wildcard, map[string]interface{}{
		"bar": map[Key]interface{}{
			// Should be able to embed pointer key.
			key.New(key.NewPointer(path.New("baz"))):
			// Should be able to embed pointer value.
			key.NewPointer(path.New("baz")),
		},
	}))
	m := map[Key]string{
		key.New(a): "a",
	}
	if s, ok := m[key.New(a)]; !ok {
		t.Error("pointer to path not keyed in map")
	} else if s != "a" {
		t.Errorf("pointer to path not mapped to correct value in map: %s", s)
	}

	// Ensure that we preserve custom pointer implementations.
	b := key.New(pointer("/foo/bar"))
	if _, ok := b.Key().(pointer); !ok {
		t.Errorf("pointer implementation not preserved: %T", b.Key())
	}
}
func TestMkiStringifyCollection(t *testing.T) {
	for name, tcase := range map[string]struct {
		input  map[Key]interface{}
		output string
	}{
		"empty": {
			input:  map[Key]interface{}{},
			output: "map[]",
		},
		"single": {
			input: map[Key]interface{}{
				New("foobar"): uint32(42),
			},
			output: "map[foobar:42]",
		},
		"double": {
			input: map[Key]interface{}{
				New("foobar"): uint32(42),
				New("baz"):    uint32(11),
			},
			output: "map[baz:11 foobar:42]",
		},
		"map keys": {
			input: map[Key]interface{}{
				New(map[string]interface{}{"foo": uint32(1), "bar": uint32(2)}): uint32(42),
				New(map[string]interface{}{"foo": uint32(3), "bar": uint32(4)}): uint32(11),
			},
			output: "map[map[bar:2 foo:1]:42 map[bar:4 foo:3]:11]",
		},
		"string map in key map in string map in key map": {
			input: map[Key]interface{}{
				New(map[string]interface{}{"coll": map[Key]interface{}{
					New(map[string]interface{}{"one": "two"}):    uint64(22),
					New(map[string]interface{}{"three": "four"}): uint64(33),
				}}): uint32(42),
			},
			output: "map[map[coll:map[map[one:two]:22 map[three:four]:33]]:42]",
		},
		"mixed types": {
			input: map[Key]interface{}{
				New(uint32(42)):    true,
				New(float64(0.25)): 0.1,
				New(float32(0.5)):  0.2,
				New("foo"):         "bar",
				New(map[string]interface{}{"hello": "world"}): "yolo",
			},
			output: "map[0.25:0.1 0.5:0.2 42:true foo:bar map[hello:world]:yolo]",
		}} {
		t.Run(name, func(t *testing.T) {
			got := StringifyCollection(tcase.input)
			if got != tcase.output {
				t.Errorf("expected: %q\ngot: %q", tcase.output, got)
			}
		})
	}
}

func TestMkiStringifyCollectionSameAsFmt(t *testing.T) {
	keyMap := map[Key]interface{}{
		New("bar"): uint32(2),
		New("foo"): uint32(1),
	}
	strMap := map[string]interface{}{
		"bar": uint32(2),
		"foo": uint32(1),
	}

	got := StringifyCollection(keyMap)
	exp := fmt.Sprint(strMap)

	if got != exp {
		t.Errorf("expected Fmt formatting to match StringifyCollection: exp: %s\ngot:%s", exp, got)
	}
}
