// Copyright (c) 2015 Arista Networks, Inc.  All rights reserved.
// Arista Networks, Inc. Confidential and Proprietary.

package key_test

import (
	"fmt"
	"testing"

	. "github.com/aristanetworks/goarista/key"
	"github.com/aristanetworks/goarista/test"
)

func TestKeyEqual(t *testing.T) {
	tests := []struct {
		a      Key
		b      Key
		result bool
	}{{
		a:      NewKey("foo"),
		b:      NewKey("foo"),
		result: true,
	}, {
		a:      NewKey("foo"),
		b:      NewKey("bar"),
		result: false,
	}, {
		a:      NewKey(map[string]interface{}{}),
		b:      NewKey("bar"),
		result: false,
	}, {
		a:      NewKey(map[string]interface{}{}),
		b:      NewKey(map[string]interface{}{}),
		result: true,
	}, {
		a:      NewKey(map[string]interface{}{"a": 3}),
		b:      NewKey(map[string]interface{}{}),
		result: false,
	}, {
		a:      NewKey(map[string]interface{}{"a": 3}),
		b:      NewKey(map[string]interface{}{"b": 4}),
		result: false,
	}, {
		a:      NewKey(map[string]interface{}{"a": 3}),
		b:      NewKey(map[string]interface{}{"a": 4}),
		result: false,
	}, {
		a:      NewKey(map[string]interface{}{"a": 3}),
		b:      NewKey(map[string]interface{}{"a": 3}),
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

	if NewKey("a").Equal(32) == true {
		t.Error("Wrong result for different types case")
	}
}

func TestIsHashable(t *testing.T) {
	tests := []struct {
		k interface{}
		h bool
	}{{
		true,
		true,
	}, {
		false,
		true,
	}, {
		uint8(3),
		true,
	}, {
		uint16(3),
		true,
	}, {
		uint32(3),
		true,
	}, {
		uint64(3),
		true,
	}, {
		int8(3),
		true,
	}, {
		int16(3),
		true,
	}, {
		int32(3),
		true,
	}, {
		int64(3),
		true,
	}, {
		float32(3.2),
		true,
	}, {
		float64(3.3),
		true,
	}, {
		"foobar",
		true,
	}, {
		map[string]interface{}{"foo": "bar"},
		false,
	}}

	for _, tcase := range tests {
		if NewKey(tcase.k).IsHashable() != tcase.h {
			t.Errorf("Wrong result for case:\nk: %#v",
				tcase.k)

		}
	}
}

func TestGetFromMap(t *testing.T) {
	tests := []struct {
		k     Key
		m     map[Key]interface{}
		v     interface{}
		found bool
	}{{
		k:     NewKey("a"),
		m:     map[Key]interface{}{NewKey("a"): "b"},
		v:     "b",
		found: true,
	}, {
		k:     NewKey(uint32(35)),
		m:     map[Key]interface{}{NewKey(uint32(35)): "c"},
		v:     "c",
		found: true,
	}, {
		k:     NewKey(uint32(37)),
		m:     map[Key]interface{}{NewKey(uint32(36)): "c"},
		found: false,
	}, {
		k:     NewKey(uint32(37)),
		m:     map[Key]interface{}{},
		found: false,
	}, {
		k: NewKey(map[string]interface{}{"a": "b", "c": uint64(4)}),
		m: map[Key]interface{}{
			NewKey(map[string]interface{}{"a": "b", "c": uint64(4)}): "foo",
		},
		v:     "foo",
		found: true,
	}}

	for _, tcase := range tests {
		v, ok := tcase.k.GetFromMap(tcase.m)
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
		m map[Key]interface{}
		r map[Key]interface{}
	}{{
		k: NewKey("a"),
		m: map[Key]interface{}{NewKey("a"): "b"},
		r: map[Key]interface{}{},
	}, {
		k: NewKey("b"),
		m: map[Key]interface{}{NewKey("a"): "b"},
		r: map[Key]interface{}{NewKey("a"): "b"},
	}, {
		k: NewKey("a"),
		m: map[Key]interface{}{},
		r: map[Key]interface{}{},
	}, {
		k: NewKey(uint32(35)),
		m: map[Key]interface{}{NewKey(uint32(35)): "c"},
		r: map[Key]interface{}{},
	}, {
		k: NewKey(uint32(36)),
		m: map[Key]interface{}{NewKey(uint32(35)): "c"},
		r: map[Key]interface{}{NewKey(uint32(35)): "c"},
	}, {
		k: NewKey(uint32(37)),
		m: map[Key]interface{}{NewKey(uint32(36)): "c"},
		r: map[Key]interface{}{NewKey(uint32(36)): "c"},
	}, {
		k: NewKey(uint32(37)),
		m: map[Key]interface{}{},
		r: map[Key]interface{}{},
	}, {
		k: NewKey(map[string]interface{}{"a": "b", "c": uint64(4)}),
		m: map[Key]interface{}{
			NewKey(map[string]interface{}{"a": "b", "c": uint64(4)}): "foo",
		},
		r: map[Key]interface{}{},
	}}

	for _, tcase := range tests {
		tcase.k.DeleteFromMap(tcase.m)
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
		m map[Key]interface{}
		r map[Key]interface{}
	}{{
		k: NewKey("a"),
		v: "c",
		m: map[Key]interface{}{NewKey("a"): "b"},
		r: map[Key]interface{}{NewKey("a"): "c"},
	}, {
		k: NewKey("b"),
		v: uint64(56),
		m: map[Key]interface{}{NewKey("a"): "b"},
		r: map[Key]interface{}{
			NewKey("a"): "b",
			NewKey("b"): uint64(56),
		},
	}, {
		k: NewKey("a"),
		v: "foo",
		m: map[Key]interface{}{},
		r: map[Key]interface{}{NewKey("a"): "foo"},
	}, {
		k: NewKey(uint32(35)),
		v: "d",
		m: map[Key]interface{}{NewKey(uint32(35)): "c"},
		r: map[Key]interface{}{NewKey(uint32(35)): "d"},
	}, {
		k: NewKey(uint32(36)),
		v: true,
		m: map[Key]interface{}{NewKey(uint32(35)): "c"},
		r: map[Key]interface{}{
			NewKey(uint32(35)): "c",
			NewKey(uint32(36)): true,
		},
	}, {
		k: NewKey(uint32(37)),
		v: false,
		m: map[Key]interface{}{NewKey(uint32(36)): "c"},
		r: map[Key]interface{}{
			NewKey(uint32(36)): "c",
			NewKey(uint32(37)): false,
		},
	}, {
		k: NewKey(uint32(37)),
		v: "foobar",
		m: map[Key]interface{}{},
		r: map[Key]interface{}{NewKey(uint32(37)): "foobar"},
	}, {
		k: NewKey(map[string]interface{}{"a": "b", "c": uint64(4)}),
		v: "foobar",
		m: map[Key]interface{}{
			NewKey(map[string]interface{}{"a": "b", "c": uint64(4)}): "foo",
		},
		r: map[Key]interface{}{
			NewKey(map[string]interface{}{"a": "b", "c": uint64(4)}): "foobar",
		},
	}, {
		k: NewKey(map[string]interface{}{"a": "b", "c": uint64(7)}),
		v: "foobar",
		m: map[Key]interface{}{
			NewKey(map[string]interface{}{"a": "b", "c": uint64(4)}): "foo",
		},
		r: map[Key]interface{}{
			NewKey(map[string]interface{}{"a": "b", "c": uint64(4)}): "foo",
			NewKey(map[string]interface{}{"a": "b", "c": uint64(7)}): "foobar",
		},
	}, {
		k: NewKey(map[string]interface{}{"a": "b", "d": uint64(6)}),
		v: "barfoo",
		m: map[Key]interface{}{
			NewKey(map[string]interface{}{"a": "b", "c": uint64(4)}): "foo",
		},
		r: map[Key]interface{}{
			NewKey(map[string]interface{}{"a": "b", "c": uint64(4)}): "foo",
			NewKey(map[string]interface{}{"a": "b", "d": uint64(6)}): "barfoo",
		},
	}}

	for i, tcase := range tests {
		tcase.k.SetToMap(tcase.m, tcase.v)
		if !test.DeepEqual(tcase.m, tcase.r) {
			t.Errorf("Wrong result for case %d:\nk: %#v\nm: %#v\nr: %#v",
				i,
				tcase.k,
				tcase.m,
				tcase.r)
		}
	}
}

func BenchmarkSetToMapWithStringKey(b *testing.B) {
	m := map[Key]interface{}{
		NewKey("a"):   true,
		NewKey("a1"):  true,
		NewKey("a2"):  true,
		NewKey("a3"):  true,
		NewKey("a4"):  true,
		NewKey("a5"):  true,
		NewKey("a6"):  true,
		NewKey("a7"):  true,
		NewKey("a8"):  true,
		NewKey("a9"):  true,
		NewKey("a10"): true,
		NewKey("a11"): true,
		NewKey("a12"): true,
		NewKey("a13"): true,
		NewKey("a14"): true,
		NewKey("a15"): true,
		NewKey("a16"): true,
		NewKey("a17"): true,
		NewKey("a18"): true,
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		NewKey(fmt.Sprintf("b%d", i)).SetToMap(m, true)
	}
}

func BenchmarkSetToMapWithUint64Key(b *testing.B) {
	m := map[Key]interface{}{
		NewKey(uint64(1)):  true,
		NewKey(uint64(2)):  true,
		NewKey(uint64(3)):  true,
		NewKey(uint64(4)):  true,
		NewKey(uint64(5)):  true,
		NewKey(uint64(6)):  true,
		NewKey(uint64(7)):  true,
		NewKey(uint64(8)):  true,
		NewKey(uint64(9)):  true,
		NewKey(uint64(10)): true,
		NewKey(uint64(11)): true,
		NewKey(uint64(12)): true,
		NewKey(uint64(13)): true,
		NewKey(uint64(14)): true,
		NewKey(uint64(15)): true,
		NewKey(uint64(16)): true,
		NewKey(uint64(17)): true,
		NewKey(uint64(18)): true,
		NewKey(uint64(19)): true,
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		NewKey(uint64(i)).SetToMap(m, true)
	}
}

func BenchmarkGetFromMapWithMapKey(b *testing.B) {
	m := map[Key]interface{}{
		NewKey(map[string]interface{}{"a0": true}):  true,
		NewKey(map[string]interface{}{"a1": true}):  true,
		NewKey(map[string]interface{}{"a2": true}):  true,
		NewKey(map[string]interface{}{"a3": true}):  true,
		NewKey(map[string]interface{}{"a4": true}):  true,
		NewKey(map[string]interface{}{"a5": true}):  true,
		NewKey(map[string]interface{}{"a6": true}):  true,
		NewKey(map[string]interface{}{"a7": true}):  true,
		NewKey(map[string]interface{}{"a8": true}):  true,
		NewKey(map[string]interface{}{"a9": true}):  true,
		NewKey(map[string]interface{}{"a10": true}): true,
		NewKey(map[string]interface{}{"a11": true}): true,
		NewKey(map[string]interface{}{"a12": true}): true,
		NewKey(map[string]interface{}{"a13": true}): true,
		NewKey(map[string]interface{}{"a14": true}): true,
		NewKey(map[string]interface{}{"a15": true}): true,
		NewKey(map[string]interface{}{"a16": true}): true,
		NewKey(map[string]interface{}{"a17": true}): true,
		NewKey(map[string]interface{}{"a18": true}): true,
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		key := NewKey(map[string]interface{}{fmt.Sprintf("a%d", i%19): true})
		_, found := key.GetFromMap(m)
		if !found {
			b.Fatalf("WTF: %#v", key)
		}
	}
}
