// Copyright (c) 2025 Arista Networks, Inc.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the COPYING file.

package key_test

import (
	"fmt"
	"hash/maphash"
	"testing"

	. "github.com/aristanetworks/goarista/key"
)

// hashableKey is a Key that also implements Hashable, exercising
// the `case Hashable` branch of Hash (which is reached only after
// the concrete-type cases miss).
type hashableKey struct {
	v uint64
}

func (h hashableKey) Key() any       { return h }
func (h hashableKey) String() string { return fmt.Sprintf("hashableKey{%d}", h.v) }
func (h hashableKey) Equal(other any) bool {
	o, ok := other.(hashableKey)
	return ok && o.v == h.v
}
func (h hashableKey) Hash() uint64 { return h.v }

func TestHashAllBranches(t *testing.T) {
	seed := maphash.MakeSeed()
	cases := []struct {
		name string
		a, b Key // equal pair
	}{
		{"nil", New(nil), New(nil)},
		{"string", New("hello"), New("hello")},
		{"bytes", New([]byte{1, 2, 3}), New([]byte{1, 2, 3})},
		{"int8", New(int8(-5)), New(int8(-5))},
		{"int16", New(int16(-500)), New(int16(-500))},
		{"int32", New(int32(-50000)), New(int32(-50000))},
		{"int64", New(int64(-5000000000)), New(int64(-5000000000))},
		{"uint8", New(uint8(5)), New(uint8(5))},
		{"uint16", New(uint16(500)), New(uint16(500))},
		{"uint32", New(uint32(50000)), New(uint32(50000))},
		{"uint64", New(uint64(5000000000)), New(uint64(5000000000))},
		{"float32", New(float32(1.5)), New(float32(1.5))},
		{"float64", New(float64(1.5)), New(float64(1.5))},
		{"bool", New(true), New(true)},
		{"map", New(map[string]any{"a": 1, "b": "x"}),
			New(map[string]any{"a": 1, "b": "x"})},
		{"slice", New([]any{int64(1), "x", true}),
			New([]any{int64(1), "x", true})},
		{"path", New(Path{New("a"), New(int64(1))}),
			New(Path{New("a"), New(int64(1))})},
		{"pointer", New(NewPointer(Path{New("a"), New("b")})),
			New(NewPointer(Path{New("a"), New("b")}))},
		{"hashable", hashableKey{42}, hashableKey{42}},
		{"value", New(customKey{i: 7}), New(customKey{i: 7})},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ha := Hash(seed, tc.a)
			hb := Hash(seed, tc.b)
			if ha != hb {
				t.Errorf("equal keys hashed differently: %x vs %x", ha, hb)
			}
		})
	}
}

func TestHashMapOrderIndependent(t *testing.T) {
	seed := maphash.MakeSeed()
	// Construct logically-equal maps and verify they hash the same. Go's
	// map iteration is already randomized, so running this with many
	// entries exercises the additive combine in hashMapKey.
	m1 := map[string]any{}
	m2 := map[string]any{}
	for i := range 50 {
		k := fmt.Sprintf("k%d", i)
		m1[k] = int64(i)
		m2[k] = int64(i)
	}
	h1 := Hash(seed, New(m1))
	h2 := Hash(seed, New(m2))
	if h1 != h2 {
		t.Fatalf("equal maps hashed differently: %x vs %x", h1, h2)
	}
}

func TestHashMapContentSensitive(t *testing.T) {
	seed := maphash.MakeSeed()
	cases := []struct {
		name string
		a, b map[string]any
	}{
		{"diff-int-val", map[string]any{"k": int64(1)},
			map[string]any{"k": int64(2)}},
		{"diff-key", map[string]any{"a": int64(1)},
			map[string]any{"b": int64(1)}},
		{"diff-key-and-val", map[string]any{"a": "x"},
			map[string]any{"b": "y"}},
		{"diff-str-val", map[string]any{"k": "hello"},
			map[string]any{"k": "world"}},
		{"diff-nested-val", map[string]any{"k": []any{int64(1)}},
			map[string]any{"k": []any{int64(2)}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ha := Hash(seed, New(tc.a))
			hb := Hash(seed, New(tc.b))
			if ha == hb {
				t.Fatalf("different maps share hash %x: %v vs %v", ha, tc.a, tc.b)
			}
		})
	}
}

func TestHashSliceOrderDependent(t *testing.T) {
	seed := maphash.MakeSeed()
	a := New([]any{int64(1), int64(2), int64(3)})
	b := New([]any{int64(3), int64(2), int64(1)})
	if Hash(seed, a) == Hash(seed, b) {
		// Could occur but is very unlikely.
		t.Fatalf("reversed slices share a hash (rare collision)")
	}
}

func TestHashAnyNestedTypes(t *testing.T) {
	seed := maphash.MakeSeed()
	// hashAny is exercised indirectly via slice/map values. Verify each
	// branch produces a stable, equal-keys-equal-hashes result.
	nested := []struct {
		name string
		a, b any
	}{
		{"nested-map", map[string]any{"x": int64(1)},
			map[string]any{"x": int64(1)}},
		{"nested-slice", []any{int64(1), int64(2)},
			[]any{int64(1), int64(2)}},
		{"nested-bytes", []byte{9, 8, 7}, []byte{9, 8, 7}},
		{"nested-pointer", NewPointer(Path{New("p"), New("q")}),
			NewPointer(Path{New("p"), New("q")})},
		{"nested-path", Path{New("p"), New("q")},
			Path{New("p"), New("q")}},
		{"nested-hashable", hashableKey{99}, hashableKey{99}},
		{"nested-primitive", "hi", "hi"},
	}
	for _, tc := range nested {
		t.Run("slice/"+tc.name, func(t *testing.T) {
			ha := Hash(seed, New([]any{tc.a}))
			hb := Hash(seed, New([]any{tc.b}))
			if ha != hb {
				t.Errorf("equal nested values hashed differently: %x vs %x", ha, hb)
			}
		})
		t.Run("map/"+tc.name, func(t *testing.T) {
			ha := Hash(seed, New(map[string]any{"k": tc.a}))
			hb := Hash(seed, New(map[string]any{"k": tc.b}))
			if ha != hb {
				t.Errorf("equal nested values hashed differently: %x vs %x", ha, hb)
			}
		})
	}
}

func TestHashDifferentSeeds(t *testing.T) {
	s1 := maphash.MakeSeed()
	s2 := maphash.MakeSeed()
	k := New("seed-sensitive")
	if Hash(s1, k) == Hash(s2, k) {
		// Possible but vanishingly unlikely.
		t.Fatalf("two random seeds produced the same hash for %v", k)
	}
}

func TestHashStable(t *testing.T) {
	// A given (seed, key) pair must always produce the same hash within
	// a process.
	seed := maphash.MakeSeed()
	k := New(map[string]any{
		"a": int64(1),
		"b": []any{int64(2), "three"},
		"c": []byte{0xde, 0xad, 0xbe, 0xef},
	})
	want := Hash(seed, k)
	for range 10 {
		if got := Hash(seed, k); got != want {
			t.Fatalf("Hash not stable: got %x want %x", got, want)
		}
	}
}

var benchSink uint64

func BenchmarkHash(b *testing.B) {
	seed := maphash.MakeSeed()
	bigSlice := make([]any, 32)
	for i := range bigSlice {
		bigSlice[i] = int64(i)
	}
	bigMap := make(map[string]any, 32)
	for i := range 32 {
		bigMap[fmt.Sprintf("k%d", i)] = int64(i)
	}
	cases := []struct {
		name string
		k    Key
	}{
		{"nil", New(nil)},
		{"string", New("hello world")},
		{"bytes", New([]byte("hello world"))},
		{"int64", New(int64(123456789))},
		{"uint64", New(uint64(123456789))},
		{"float64", New(float64(3.14159))},
		{"bool", New(true)},
		{"hashable", hashableKey{0xdeadbeef}},
		{"value", New(customKey{i: 42})},
		{"slice-small", New([]any{int64(1), "x", true})},
		{"slice-big", New(bigSlice)},
		{"map-small", New(map[string]any{"a": int64(1), "b": "x"})},
		{"map-big", New(bigMap)},
		{"path", New(Path{New("a"), New("b"), New("c")})},
		{"pointer", New(NewPointer(Path{New("a"), New("b"), New("c")}))},
		{"nested", New([]any{
			map[string]any{"x": int64(1)},
			[]byte{1, 2, 3},
			NewPointer(Path{New("p")}),
		})},
	}
	for _, tc := range cases {
		b.Run(tc.name, func(b *testing.B) {
			var s uint64
			for b.Loop() {
				s ^= Hash(seed, tc.k)
			}
			benchSink = s
		})
	}
}
