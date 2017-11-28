// Copyright (c) 2017 Arista Networks, Inc.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the COPYING file.

package path

import (
	"errors"
	"fmt"
	"testing"

	"github.com/aristanetworks/goarista/key"
	"github.com/aristanetworks/goarista/pathmap"
	"github.com/aristanetworks/goarista/test"
)

func accumulator(counter map[int]int) pathmap.VisitorFunc {
	return func(val interface{}) error {
		counter[val.(int)]++
		return nil
	}
}

func TestVisit(t *testing.T) {
	m := NewMap()
	m.Set(Path{key.New("foo"), key.New("bar"), key.New("baz")}, 1)
	m.Set(Path{key.New("*"), key.New("bar"), key.New("baz")}, 2)
	m.Set(Path{key.New("*"), key.New("*"), key.New("baz")}, 3)
	m.Set(Path{key.New("*"), key.New("*"), key.New("*")}, 4)
	m.Set(Path{key.New("foo"), key.New("*"), key.New("*")}, 5)
	m.Set(Path{key.New("foo"), key.New("bar"), key.New("*")}, 6)
	m.Set(Path{key.New("foo"), key.New("*"), key.New("baz")}, 7)
	m.Set(Path{key.New("*"), key.New("bar"), key.New("*")}, 8)

	m.Set(Path{}, 10)

	m.Set(Path{key.New("*")}, 20)
	m.Set(Path{key.New("foo")}, 21)

	m.Set(Path{key.New("zap"), key.New("zip")}, 30)
	m.Set(Path{key.New("zap"), key.New("zip")}, 31)

	m.Set(Path{key.New("zip"), key.New("*")}, 40)
	m.Set(Path{key.New("zip"), key.New("*")}, 41)

	testCases := []struct {
		path     Path
		expected map[int]int
	}{{
		path:     Path{key.New("foo"), key.New("bar"), key.New("baz")},
		expected: map[int]int{1: 1, 2: 1, 3: 1, 4: 1, 5: 1, 6: 1, 7: 1, 8: 1},
	}, {
		path:     Path{key.New("qux"), key.New("bar"), key.New("baz")},
		expected: map[int]int{2: 1, 3: 1, 4: 1, 8: 1},
	}, {
		path:     Path{key.New("foo"), key.New("qux"), key.New("baz")},
		expected: map[int]int{3: 1, 4: 1, 5: 1, 7: 1},
	}, {
		path:     Path{key.New("foo"), key.New("bar"), key.New("qux")},
		expected: map[int]int{4: 1, 5: 1, 6: 1, 8: 1},
	}, {
		path:     Path{},
		expected: map[int]int{10: 1},
	}, {
		path:     Path{key.New("foo")},
		expected: map[int]int{20: 1, 21: 1},
	}, {
		path:     Path{key.New("foo"), key.New("bar")},
		expected: map[int]int{},
	}, {
		path:     Path{key.New("zap"), key.New("zip")},
		expected: map[int]int{31: 1},
	}, {
		path:     Path{key.New("zip"), key.New("zap")},
		expected: map[int]int{41: 1},
	}}

	for _, tc := range testCases {
		result := make(map[int]int, len(tc.expected))
		m.Visit(tc.path, accumulator(result))
		if diff := test.Diff(tc.expected, result); diff != "" {
			t.Errorf("Test case %v: %s", tc.path, diff)
		}
	}
}

func TestVisitError(t *testing.T) {
	m := NewMap()
	m.Set(Path{key.New("foo"), key.New("bar")}, 1)
	m.Set(Path{key.New("*"), key.New("bar")}, 2)

	errTest := errors.New("Test")

	err := m.Visit(Path{key.New("foo"), key.New("bar")},
		func(v interface{}) error { return errTest })
	if err != errTest {
		t.Errorf("Unexpected error. Expected: %v, Got: %v", errTest, err)
	}
	err = m.VisitPrefix(Path{key.New("foo"), key.New("bar"), key.New("baz")},
		func(v interface{}) error { return errTest })
	if err != errTest {
		t.Errorf("Unexpected error. Expected: %v, Got: %v", errTest, err)
	}
}

func TestGet(t *testing.T) {
	m := NewMap()
	m.Set(Path{}, 0)
	m.Set(Path{key.New("foo"), key.New("bar")}, 1)
	m.Set(Path{key.New("foo"), key.New("*")}, 2)
	m.Set(Path{key.New("*"), key.New("bar")}, 3)
	m.Set(Path{key.New("zap"), key.New("zip")}, 4)

	testCases := []struct {
		path     Path
		expected interface{}
	}{{
		path:     Path{},
		expected: 0,
	}, {
		path:     Path{key.New("foo"), key.New("bar")},
		expected: 1,
	}, {
		path:     Path{key.New("foo"), key.New("*")},
		expected: 2,
	}, {
		path:     Path{key.New("*"), key.New("bar")},
		expected: 3,
	}, {
		path:     Path{key.New("bar"), key.New("foo")},
		expected: nil,
	}, {
		path:     Path{key.New("zap"), key.New("*")},
		expected: nil,
	}}

	for _, tc := range testCases {
		got := m.Get(tc.path)
		if got != tc.expected {
			t.Errorf("Test case %v: Expected %v, Got %v",
				tc.path, tc.expected, got)
		}
	}
}

func countNodes(n *node) int {
	if n == nil {
		return 0
	}
	count := 1
	count += countNodes(n.wildcard)
	for _, child := range n.children {
		count += countNodes(child)
	}
	return count
}

func TestDelete(t *testing.T) {
	m := NewMap()
	m.Set(Path{}, 0)
	m.Set(Path{key.New("*")}, 1)
	m.Set(Path{key.New("foo"), key.New("bar")}, 2)
	m.Set(Path{key.New("foo"), key.New("*")}, 3)

	n := countNodes(m.(*node))
	if n != 5 {
		t.Errorf("Initial count wrong. Expected: 5, Got: %d", n)
	}

	testCases := []struct {
		del      Path        // Path to delete
		expected bool        // expected return value of Delete
		visit    Path        // Path to Visit
		before   map[int]int // Expected to find items before deletion
		after    map[int]int // Expected to find items after deletion
		count    int         // Count of nodes
	}{{
		del:      Path{key.New("zap")}, // A no-op Delete
		expected: false,
		visit:    Path{key.New("foo"), key.New("bar")},
		before:   map[int]int{2: 1, 3: 1},
		after:    map[int]int{2: 1, 3: 1},
		count:    5,
	}, {
		del:      Path{key.New("foo"), key.New("bar")},
		expected: true,
		visit:    Path{key.New("foo"), key.New("bar")},
		before:   map[int]int{2: 1, 3: 1},
		after:    map[int]int{3: 1},
		count:    4,
	}, {
		del:      Path{key.New("*")},
		expected: true,
		visit:    Path{key.New("foo")},
		before:   map[int]int{1: 1},
		after:    map[int]int{},
		count:    3,
	}, {
		del:      Path{key.New("*")},
		expected: false,
		visit:    Path{key.New("foo")},
		before:   map[int]int{},
		after:    map[int]int{},
		count:    3,
	}, {
		del:      Path{key.New("foo"), key.New("*")},
		expected: true,
		visit:    Path{key.New("foo"), key.New("bar")},
		before:   map[int]int{3: 1},
		after:    map[int]int{},
		count:    1, // Should have deleted "foo" and "bar" nodes
	}, {
		del:      Path{},
		expected: true,
		visit:    Path{},
		before:   map[int]int{0: 1},
		after:    map[int]int{},
		count:    1, // Root node can't be deleted
	}}

	for i, tc := range testCases {
		beforeResult := make(map[int]int, len(tc.before))
		m.Visit(tc.visit, accumulator(beforeResult))
		if diff := test.Diff(tc.before, beforeResult); diff != "" {
			t.Errorf("Test case %d (%v): %s", i, tc.del, diff)
		}

		if got := m.Delete(tc.del); got != tc.expected {
			t.Errorf("Test case %d (%v): Unexpected return. Expected %t, Got: %t",
				i, tc.del, tc.expected, got)
		}

		afterResult := make(map[int]int, len(tc.after))
		m.Visit(tc.visit, accumulator(afterResult))
		if diff := test.Diff(tc.after, afterResult); diff != "" {
			t.Errorf("Test case %d (%v): %s", i, tc.del, diff)
		}
	}
}

func TestVisitPrefix(t *testing.T) {
	m := NewMap()
	m.Set(Path{}, 0)
	m.Set(Path{key.New("foo")}, 1)
	m.Set(Path{key.New("foo"), key.New("bar")}, 2)
	m.Set(Path{key.New("foo"), key.New("bar"), key.New("baz")}, 3)
	m.Set(Path{key.New("foo"), key.New("bar"), key.New("baz"), key.New("quux")}, 4)
	m.Set(Path{key.New("quux"), key.New("bar")}, 5)
	m.Set(Path{key.New("foo"), key.New("quux")}, 6)
	m.Set(Path{key.New("*")}, 7)
	m.Set(Path{key.New("foo"), key.New("*")}, 8)
	m.Set(Path{key.New("*"), key.New("bar")}, 9)
	m.Set(Path{key.New("*"), key.New("quux")}, 10)
	m.Set(Path{key.New("quux"), key.New("quux"), key.New("quux"), key.New("quux")}, 11)

	testCases := []struct {
		path     Path
		expected map[int]int
	}{{
		path:     Path{key.New("foo"), key.New("bar"), key.New("baz")},
		expected: map[int]int{0: 1, 1: 1, 2: 1, 3: 1, 7: 1, 8: 1, 9: 1},
	}, {
		path:     Path{key.New("zip"), key.New("zap")},
		expected: map[int]int{0: 1, 7: 1},
	}, {
		path:     Path{key.New("foo"), key.New("zap")},
		expected: map[int]int{0: 1, 1: 1, 8: 1, 7: 1},
	}, {
		path:     Path{key.New("quux"), key.New("quux"), key.New("quux")},
		expected: map[int]int{0: 1, 7: 1, 10: 1},
	}}

	for _, tc := range testCases {
		result := make(map[int]int, len(tc.expected))
		m.VisitPrefix(tc.path, accumulator(result))
		if diff := test.Diff(tc.expected, result); diff != "" {
			t.Errorf("Test case %v: %s", tc.path, diff)
		}
	}
}

func TestString(t *testing.T) {
	m := NewMap()
	m.Set(Path{}, 0)
	m.Set(Path{key.New("foo"), key.New("bar")}, 1)
	m.Set(Path{key.New("foo"), key.New("quux")}, 2)
	m.Set(Path{key.New("foo"), key.New("*")}, 3)

	expected := `Val: 0
Child "foo":
  Child "*":
    Val: 3
  Child "bar":
    Val: 1
  Child "quux":
    Val: 2
`
	got := fmt.Sprint(m)

	if expected != got {
		t.Errorf("Unexpected string. Expected:\n\n%s\n\nGot:\n\n%s", expected, got)
	}
}

func genWords(count, wordLength int) Path {
	chars := []byte("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789")
	if count+wordLength > len(chars) {
		panic("need more chars")
	}
	result := make(Path, count)
	for i := 0; i < count; i++ {
		result[i] = key.New(string(chars[i : i+wordLength]))
	}
	return result
}

func benchmarkPathMap(pathLength, pathDepth int, b *testing.B) {
	m := NewMap()

	// Push pathDepth paths, each of length pathLength
	path := genWords(pathLength, 10)
	words := genWords(pathDepth, 10)
	n := m.(*node)
	for _, element := range path {
		n.children = map[key.Key]*node{}
		for _, word := range words {
			n.children[word] = &node{}
		}
		n = n.children[element]
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		m.Visit(path, func(v interface{}) error { return nil })
	}
}

func BenchmarkPathMap1x25(b *testing.B)  { benchmarkPathMap(1, 25, b) }
func BenchmarkPathMap10x50(b *testing.B) { benchmarkPathMap(10, 25, b) }
func BenchmarkPathMap20x50(b *testing.B) { benchmarkPathMap(20, 25, b) }
