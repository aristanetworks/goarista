// Copyright (c) 2020 Arista Networks, Inc.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the COPYING file.

package path

import (
	"errors"
	"math/rand"
	"strings"
	"testing"

	"github.com/aristanetworks/goarista/key"
	"github.com/aristanetworks/goarista/test"
)

func TestMap2IsEmpty(t *testing.T) {
	m := map2{}

	if !m.IsEmpty() {
		t.Errorf("Expected IsEmpty() to return true; Got false")
	}

	nonWildcardPath := key.Path{key.New("foo")}
	wildcardPath := key.Path{Wildcard, key.New("bar"), key.New("baz")}

	m.Set(nonWildcardPath, 0)
	if m.IsEmpty() {
		t.Errorf("Expected IsEmpty() to return false; Got true")
	}

	m.Set(wildcardPath, 2)
	if m.IsEmpty() {
		t.Errorf("Expected IsEmpty() to return false; Got true")
	}

	m.Delete(nonWildcardPath)
	if m.IsEmpty() {
		t.Errorf("Expected IsEmpty() to return false; Got true")
	}

	m.Delete(wildcardPath)
	if !m.IsEmpty() {
		t.Errorf("Expected IsEmpty() to return true; Got false")
	}

	m.Set(nil, nil)
	if m.IsEmpty() {
		t.Errorf("Expected IsEmpty() to return false; Got true")
	}

	m.Delete(nil)
	if !m.IsEmpty() {
		t.Errorf("Expected IsEmpty() to return true; Got false")
	}
}

func TestMap2Set(t *testing.T) {
	m := map2{}
	a := m.Set(key.Path{key.New("foo")}, 0)
	b := m.Set(key.Path{key.New("foo")}, 1)
	if !a || b {
		t.Fatal("Map.Set not working properly")
	}
}

func TestMap2Visit(t *testing.T) {
	m := map2{}
	m.Set(key.Path{key.New("foo"), key.New("bar"), key.New("baz")}, 1)
	m.Set(key.Path{Wildcard, key.New("bar"), key.New("baz")}, 2)
	m.Set(key.Path{Wildcard, Wildcard, key.New("baz")}, 3)
	m.Set(key.Path{Wildcard, Wildcard, Wildcard}, 4)
	m.Set(key.Path{key.New("foo"), Wildcard, Wildcard}, 5)
	m.Set(key.Path{key.New("foo"), key.New("bar"), Wildcard}, 6)
	m.Set(key.Path{key.New("foo"), Wildcard, key.New("baz")}, 7)
	m.Set(key.Path{Wildcard, key.New("bar"), Wildcard}, 8)

	m.Set(key.Path{}, 10)

	m.Set(key.Path{Wildcard}, 20)
	m.Set(key.Path{key.New("foo")}, 21)

	m.Set(key.Path{key.New("zap"), key.New("zip")}, 30)
	m.Set(key.Path{key.New("zap"), key.New("zip")}, 31)

	m.Set(key.Path{key.New("zip"), Wildcard}, 40)
	m.Set(key.Path{key.New("zip"), Wildcard}, 41)

	testCases := []struct {
		path     key.Path
		expected map[int]int
	}{{
		path:     key.Path{key.New("foo"), key.New("bar"), key.New("baz")},
		expected: map[int]int{1: 1, 2: 1, 3: 1, 4: 1, 5: 1, 6: 1, 7: 1, 8: 1},
	}, {
		path:     key.Path{key.New("qux"), key.New("bar"), key.New("baz")},
		expected: map[int]int{2: 1, 3: 1, 4: 1, 8: 1},
	}, {
		path:     key.Path{key.New("foo"), key.New("qux"), key.New("baz")},
		expected: map[int]int{3: 1, 4: 1, 5: 1, 7: 1},
	}, {
		path:     key.Path{key.New("foo"), key.New("bar"), key.New("qux")},
		expected: map[int]int{4: 1, 5: 1, 6: 1, 8: 1},
	}, {
		path:     key.Path{},
		expected: map[int]int{10: 1},
	}, {
		path:     key.Path{key.New("foo")},
		expected: map[int]int{20: 1, 21: 1},
	}, {
		path:     key.Path{key.New("foo"), key.New("bar")},
		expected: map[int]int{},
	}, {
		path:     key.Path{key.New("zap"), key.New("zip")},
		expected: map[int]int{31: 1},
	}, {
		path:     key.Path{key.New("zip"), key.New("zap")},
		expected: map[int]int{41: 1},
	}}

	for _, tc := range testCases {
		result := make(map[int]int, len(tc.expected))
		m.Visit(tc.path, accumulator(result))
		if diff := test.Diff(tc.expected, result); diff != "" {
			t.Errorf("Test case %v: %s", tc.path, diff)
			t.Logf("m:\n%s", &m)
		}
	}
}

func TestMap2VisitError(t *testing.T) {
	m := map2{}
	m.Set(key.Path{key.New("foo"), key.New("bar")}, 1)
	m.Set(key.Path{Wildcard, key.New("bar")}, 2)

	errTest := errors.New("Test")

	err := m.Visit(key.Path{key.New("foo"), key.New("bar")},
		func(v interface{}) error { return errTest })
	if err != errTest {
		t.Errorf("Unexpected error. Expected: %v, Got: %v", errTest, err)
	}
	err = m.VisitPrefixes(key.Path{key.New("foo"), key.New("bar"), key.New("baz")},
		func(v interface{}) error { return errTest })
	if err != errTest {
		t.Errorf("Unexpected error. Expected: %v, Got: %v", errTest, err)
	}
}

func TestMap2(t *testing.T) {
	m := map2{}
	m.Set(New(), 0)
	m.Set(New("foo", "bar", "baz"), 1)
	m.Set(New("foo", Wildcard), 2)
	m.Set(New(Wildcard, "bar"), 3)
	m.Set(New("zap", "zip"), 4)
	m.Set(New("baz", "qux", "zap"), 5) // suffix path
	m.Set(New("baz", "qux"), nil)
	m.Set(New("baz"), 6) // prefixpath

	testCases := []struct {
		path key.Path
		v    interface{}
		ok   bool
	}{{
		path: New(),
		v:    0,
		ok:   true,
	}, {
		path: New("foo", "bar", "baz"),
		v:    1,
		ok:   true,
	}, {
		path: New("foo", Wildcard),
		v:    2,
		ok:   true,
	}, {
		path: New(Wildcard, "bar"),
		v:    3,
		ok:   true,
	}, {
		path: New("baz", "qux"),
		v:    nil,
		ok:   true,
	}, {
		path: New("bar", "foo"),
	}, {
		path: New("zap", Wildcard),
	}, {
		path: New("baz", "qux", "zap"),
		v:    5,
		ok:   true,
	}, {
		path: New("baz"),
		v:    6,
		ok:   true,
	}}

	for _, tc := range testCases {
		t.Run(tc.path.String(), func(t *testing.T) {
			v, ok := m.Get(tc.path)
			if v != tc.v || ok != tc.ok {
				t.Errorf("Expected (v: %v, ok: %t), Got (v: %v, ok: %t)",
					tc.v, tc.ok, v, ok)
			}
			ok = m.Delete(tc.path)
			if tc.ok != ok {
				t.Errorf("Delete returned unexpected value. Expected %t, Got %t", tc.ok, ok)
			}
			v, ok = m.Get(tc.path)
			if ok {
				t.Errorf("Get returned unexpected value after deleted: %v", v)
			}
			if tc.ok {
				m.Set(tc.path, tc.v)
			}
		})
	}
}

func TestMap2String(t *testing.T) {
	m := map2{}
	m.Set(key.Path{}, 0)
	m.Set(key.Path{key.New("foo"), key.New("bar")}, 1)
	m.Set(key.Path{key.New("foo"), key.New("quux")}, 2)
	m.Set(key.Path{key.New("foo"), Wildcard}, 3)

	expected := strings.Join([]string{
		"/: 0",
		"/foo/*: 3",
		"/foo/bar: 1",
		"/foo/quux: 2",
	}, "\n")
	got := m.String()

	if expected != got {
		t.Errorf("Unexpected string. Expected:\n\n%s\n\nGot:\n\n%s", expected, got)
	}
}

func TestMap2VisitPrefixes(t *testing.T) {
	m := map2{}
	m.Set(key.Path{}, 0)
	m.Set(key.Path{key.New("foo")}, 1)
	m.Set(key.Path{key.New("foo"), key.New("bar")}, 2)
	m.Set(key.Path{key.New("foo"), key.New("bar"), key.New("baz")}, 3)
	m.Set(key.Path{key.New("foo"), key.New("bar"), key.New("baz"), key.New("quux")}, 4)
	m.Set(key.Path{key.New("quux"), key.New("bar")}, 5)
	m.Set(key.Path{key.New("foo"), key.New("quux")}, 6)
	m.Set(key.Path{Wildcard}, 7)
	m.Set(key.Path{key.New("foo"), Wildcard}, 8)
	m.Set(key.Path{Wildcard, key.New("bar")}, 9)
	m.Set(key.Path{Wildcard, key.New("quux")}, 10)
	m.Set(key.Path{key.New("quux"), key.New("quux"), key.New("quux"), key.New("quux")}, 11)

	testCases := []struct {
		path     key.Path
		expected map[int]int
	}{{
		path:     key.Path{key.New("foo"), key.New("bar"), key.New("baz")},
		expected: map[int]int{0: 1, 1: 1, 2: 1, 3: 1, 7: 1, 8: 1, 9: 1},
	}, {
		path:     key.Path{key.New("zip"), key.New("zap")},
		expected: map[int]int{0: 1, 7: 1},
	}, {
		path:     key.Path{key.New("foo"), key.New("zap")},
		expected: map[int]int{0: 1, 1: 1, 8: 1, 7: 1},
	}, {
		path:     key.Path{key.New("quux"), key.New("quux"), key.New("quux")},
		expected: map[int]int{0: 1, 7: 1, 10: 1},
	}}

	for _, tc := range testCases {
		result := make(map[int]int, len(tc.expected))
		m.VisitPrefixes(tc.path, accumulator(result))
		if diff := test.Diff(tc.expected, result); diff != "" {
			t.Errorf("Test case %v: %s", tc.path, diff)
		}
	}
}

func TestMap2VisitPrefixed(t *testing.T) {
	m := map2{}
	m.Set(key.Path{}, 0)
	m.Set(key.Path{key.New("qux")}, 1)
	m.Set(key.Path{key.New("foo")}, 2)
	m.Set(key.Path{key.New("foo"), key.New("qux")}, 3)
	m.Set(key.Path{key.New("foo"), key.New("bar")}, 4)
	m.Set(key.Path{Wildcard, key.New("bar")}, 5)
	m.Set(key.Path{key.New("foo"), Wildcard}, 6)
	m.Set(key.Path{key.New("qux"), key.New("foo"), key.New("bar")}, 7)

	testCases := []struct {
		in  key.Path
		out map[int]int
	}{{
		in:  key.Path{},
		out: map[int]int{0: 1, 1: 1, 2: 1, 3: 1, 4: 1, 5: 1, 6: 1, 7: 1},
	}, {
		in:  key.Path{key.New("qux")},
		out: map[int]int{1: 1, 5: 1, 7: 1},
	}, {
		in:  key.Path{key.New("foo")},
		out: map[int]int{2: 1, 3: 1, 4: 1, 5: 1, 6: 1},
	}, {
		in:  key.Path{key.New("foo"), key.New("qux")},
		out: map[int]int{3: 1, 6: 1},
	}, {
		in:  key.Path{key.New("foo"), key.New("bar")},
		out: map[int]int{4: 1, 5: 1, 6: 1},
	}, {
		in:  key.Path{key.New(int64(0))},
		out: map[int]int{5: 1},
	}, {
		in:  key.Path{Wildcard},
		out: map[int]int{5: 1},
	}, {
		in:  key.Path{Wildcard, Wildcard},
		out: map[int]int{},
	}}

	for _, tc := range testCases {
		out := make(map[int]int, len(tc.out))
		m.VisitPrefixed(tc.in, accumulator(out))
		if diff := test.Diff(tc.out, out); diff != "" {
			t.Errorf("Test case %v: %s", tc.out, diff)
		}
	}
}

func benchmarkPathMap2(pathLength, pathDepth int, b *testing.B) {
	// Push pathDepth paths, each of length pathLength
	path := genWords(pathLength, 10)
	words := genWords(pathDepth, 10)
	m := map2{}
	n := &m.n
	for _, element := range path {
		n.children = key.NewMap()
		for _, word := range words {
			n.children.Set(word, &node{p: key.Path{word}})
		}
		next, _ := n.children.Get(element)
		n = next.(*node)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		m.Visit(path, func(v interface{}) error { return nil })
	}
}

func BenchmarkPathMap2_1x25(b *testing.B)  { benchmarkPathMap2(1, 25, b) }
func BenchmarkPathMap2_10x50(b *testing.B) { benchmarkPathMap2(10, 25, b) }
func BenchmarkPathMap2_20x50(b *testing.B) { benchmarkPathMap2(20, 25, b) }

var paths []key.Path

func buildPaths() {
	if paths != nil {
		return
	}
	// 5 x 1 x 7 x 1 x 3 x 1 x 8 x 1
	paths = make([]key.Path, 0, 5*7*3*8)
	path := make(key.Path, 0, 8)
	for i := 0; i < 5; i++ {
		char := string(rune('a' + i))
		element := key.New(strings.Repeat(char, 6))
		path1 := append(path, element, element)
		for j := 0; j < 7; j++ {
			char := string(rune('a' + j))
			element := key.New(strings.Repeat(char, 6))
			path2 := append(path1, element, element)
			for k := 0; k < 3; k++ {
				char := string(rune('a' + k))
				element := key.New(strings.Repeat(char, 6))
				path3 := append(path2, element, element)
				for l := 0; l < 8; l++ {
					char := string(rune('a' + l))
					element := key.New(strings.Repeat(char, 6))
					path4 := append(path3, element, element)

					paths = append(paths, Clone(path4))
				}
			}
		}

	}

	r := rand.New(rand.NewSource(1234))
	r.Shuffle(len(paths), func(i, j int) {
		paths[i], paths[j] = paths[j], paths[i]
	})
}

func TestBuildPaths(t *testing.T) {
	buildPaths()
	var m map2
	for _, p := range paths {
		m.Set(p, nil)
	}

	t.Log(m.String())
}

func BenchmarkMapGrow(b *testing.B) {
	buildPaths()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var m Map
		for _, p := range paths {
			m.Set(p, nil)
		}
	}
}

func BenchmarkMap2Grow(b *testing.B) {
	buildPaths()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var m map2
		for _, p := range paths {
			m.Set(p, nil)
		}
	}
}

func BenchmarkMapGet(b *testing.B) {
	buildPaths()
	var m Map
	for _, p := range paths {
		m.Set(p, nil)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, p := range paths {
			_, ok := m.Get(p)
			if !ok {
				b.Fatalf("couldn't find %s", p)
			}
		}
	}
}

func BenchmarkMap2Get(b *testing.B) {
	buildPaths()
	var m map2
	for _, p := range paths {
		m.Set(p, nil)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, p := range paths {
			_, ok := m.Get(p)
			if !ok {
				b.Fatalf("couldn't find %s", p)
			}
		}
	}
}
