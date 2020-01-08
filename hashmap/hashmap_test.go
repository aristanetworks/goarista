// Copyright (c) 2019 Arista Networks, Inc.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the COPYING file.

package hashmap

import (
	"fmt"
	"math/rand"
	"strings"
	"testing"

	"github.com/aristanetworks/goarista/key"
)

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

func TestMapSetGet(t *testing.T) {
	m := New[Hashable, any](0,
		func(h Hashable) uint64 { return h.Hash() },
		func(x, y Hashable) bool { return x.Equal(y) })
	tests := []struct {
		setkey interface{}
		getkey interface{}
		val    interface{}
		found  bool
	}{{
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
		setkey: key.New(map[string]interface{}{"a": int32(1)}),
		getkey: key.New(map[string]interface{}{"a": int32(1)}),
		val:    "foo",
		found:  true,
	}, {
		getkey: key.New(map[string]interface{}{"a": int32(2)}),
		val:    nil,
		found:  false,
	}, {
		setkey: key.New(map[string]interface{}{"a": int32(2)}),
		getkey: key.New(map[string]interface{}{"a": int32(2)}),
		val:    "bar",
		found:  true,
	}}
	for _, tcase := range tests {
		if tcase.setkey != nil {
			m.Set(tcase.setkey.(Hashable), tcase.val)
		}
		val, found := m.Get(tcase.getkey.(Hashable))
		if found != tcase.found {
			t.Errorf("found is %t, but expected found %t", found, tcase.found)
		}
		if val != tcase.val {
			t.Errorf("val is %v for key %v, but expected val %v", val, tcase.getkey, tcase.val)
		}
	}
	t.Log(m.debug())
}

func BenchmarkMapGrow(b *testing.B) {
	keys := make([]key.Key, 150)
	for j := 0; j < len(keys); j++ {
		keys[j] = key.New(map[string]interface{}{
			"foobar": 100,
			"baz":    j,
		})
	}
	b.Run("key.Map", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			m := key.NewMap()
			for j := 0; j < len(keys); j++ {
				m.Set(keys[j], "foobar")
			}
			if m.Len() != len(keys) {
				b.Fatal(m)
			}
		}
	})
	b.Run("Hashmap", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			m := New[Hashable, any](0,
				func(h Hashable) uint64 { return h.Hash() },
				func(x, y Hashable) bool { return x.Equal(y) })
			for j := 0; j < len(keys); j++ {
				m.Set(keys[j].(Hashable), "foobar")
			}
			if m.Len() != len(keys) {
				b.Fatal(m)
			}
		}
	})
	b.Run("Hashmap-presize", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			m := New[Hashable, any](150,
				func(h Hashable) uint64 { return h.Hash() },
				func(x, y Hashable) bool { return x.Equal(y) })
			for j := 0; j < len(keys); j++ {
				m.Set(keys[j].(Hashable), "foobar")
			}
			if m.Len() != len(keys) {
				b.Fatal(m)
			}
		}
	})
}

func BenchmarkMapGet(b *testing.B) {
	keys := make([]key.Key, 150)
	for j := 0; j < len(keys); j++ {
		keys[j] = key.New(map[string]interface{}{
			"foobar": 100,
			"baz":    j,
		})
	}
	keysRandomOrder := make([]key.Key, len(keys))
	copy(keysRandomOrder, keys)
	rand.Shuffle(len(keysRandomOrder), func(i, j int) {
		keysRandomOrder[i], keysRandomOrder[j] = keysRandomOrder[j], keysRandomOrder[i]
	})
	b.Run("key.Map", func(b *testing.B) {
		m := key.NewMap()
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
	b.Run("Hashmap", func(b *testing.B) {
		m := New[Hashable, any](0,
			func(h Hashable) uint64 { return h.Hash() },
			func(x, y Hashable) bool { return x.Equal(y) })
		for j := 0; j < len(keys); j++ {
			m.Set(keys[j].(Hashable), "foobar")
		}
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			for _, k := range keysRandomOrder {
				_, ok := m.Get(k.(Hashable))
				if !ok {
					b.Fatal("didn't find key")
				}
			}
		}
	})
}

func (m *Hashmap[K, V]) debug() string {
	var buf strings.Builder

	for i, ent := range m.entries {
		var (
			k        string
			distance int
		)
		if !ent.occupied {
			k = "<empty>"
		} else {
			if ent.tombstone {
				k = "<tombstone>"
			} else {
				k = fmt.Sprint(ent.key)
			}
			distance = i - m.position(ent.hash)
			if distance < 0 {
				distance += len(m.entries)
			}
		}
		fmt.Fprintf(&buf, "%d %d %s\n", i, distance, k)
	}

	return buf.String()
}
