// Copyright (c) 2017 Arista Networks, Inc.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the COPYING file.

package path

import (
	"errors"
	"fmt"
	"testing"

	"github.com/aristanetworks/goarista/key"
	"github.com/aristanetworks/goarista/test"
)

func accumulator(counter map[int]int) VisitorFunc {
	return func(val interface{}) error {
		counter[val.(int)]++
		return nil
	}
}

func TestIsEmpty(t *testing.T) {
	m := Map{}

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

func TestMapSet(t *testing.T) {
	m := Map{}
	a := m.Set(key.Path{key.New("foo")}, 0)
	b := m.Set(key.Path{key.New("foo")}, 1)
	if !a || b {
		t.Fatal("Map.Set not working properly")
	}
}

func TestMapVisit(t *testing.T) {
	m := Map{}
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
		}
	}
}

func TestMapVisitError(t *testing.T) {
	m := Map{}
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

func TestMapGet(t *testing.T) {
	m := Map{}
	m.Set(key.Path{}, 0)
	m.Set(key.Path{key.New("foo"), key.New("bar")}, 1)
	m.Set(key.Path{key.New("foo"), Wildcard}, 2)
	m.Set(key.Path{Wildcard, key.New("bar")}, 3)
	m.Set(key.Path{key.New("zap"), key.New("zip")}, 4)
	m.Set(key.Path{key.New("baz"), key.New("qux")}, nil)

	testCases := []struct {
		path key.Path
		v    interface{}
		ok   bool
	}{{
		path: nil,
		v:    0,
		ok:   true,
	}, {
		path: key.Path{key.New("foo"), key.New("bar")},
		v:    1,
		ok:   true,
	}, {
		path: key.Path{key.New("foo"), Wildcard},
		v:    2,
		ok:   true,
	}, {
		path: key.Path{Wildcard, key.New("bar")},
		v:    3,
		ok:   true,
	}, {
		path: key.Path{key.New("baz"), key.New("qux")},
		v:    nil,
		ok:   true,
	}, {
		path: key.Path{key.New("bar"), key.New("foo")},
		v:    nil,
	}, {
		path: key.Path{key.New("zap"), Wildcard},
		v:    nil,
	}}

	for _, tc := range testCases {
		v, ok := m.Get(tc.path)
		if v != tc.v || ok != tc.ok {
			t.Errorf("Test case %v: Expected (v: %v, ok: %t), Got (v: %v, ok: %t)",
				tc.path, tc.v, tc.ok, v, ok)
		}
	}
}

func TestMapGetLongestPrefix(t *testing.T) {
	type testMap struct {
		pathMap Map
		expectedValues map[string]interface{}
	}
	makeMap := func(paths []string) (result testMap) {
		result.expectedValues = make(map[string]interface{})

		nextValue := uint32(1)
		for _, path := range paths {
			result.pathMap.Set(FromString(path), nextValue)
			result.expectedValues[path] = nextValue
			nextValue++
		}

		return
	}

	regularMap := makeMap([]string{
		"/",
		"/a",
		"/a/b",
		"/a/b/c/d",
		"/a/b/c/d/e",
		"/r/s",
		"/r/s/t",
		"/u/v",
	})

	noEntryAtRootMap := makeMap([]string{
		"/r/s",
		"/r/s/t",
		"/u/v",
	})

	rootOnlyMap := makeMap([]string{"/"})

	emptyMap := makeMap(nil)

	testCases := []struct {
		name        string
		mp          testMap
		path        string
		ok          bool
		longestPath string
	}{
		// The root path
		{
			name:        "exact match, descendents, root path",
			mp:          regularMap,
			path:        "/",
			ok:          true,
			longestPath: "/",
		},
		{
			name:        "no exact match, descendents, root path",
			mp:          noEntryAtRootMap,
			path:        "/",
			ok:          false,
			longestPath: "",
		},
		{
			name:        "exact match, no descendents, root path",
			mp:          rootOnlyMap,
			path:        "/",
			ok:          true,
			longestPath: "/",
		},
		{
			name:        "no exact match, no descendents, root path",
			mp:          emptyMap,
			path:        "/",
			ok:          false,
			longestPath: "",
		},

		// Non-root paths when the path map has entries associated with shorter
		// prefixes
		{
			name:        "exact match, descendents, ancestor",
			mp:          regularMap,
			path:        "/a/b/c/d",
			ok:          true,
			longestPath: "/a/b/c/d",
		},
		{
			name:        "no exact match, descendents, ancestor",
			mp:          regularMap,
			path:        "/a/b/c",
			ok:          true,
			longestPath: "/a/b",
		},
		{
			name:        "exact match, no descendents, ancestor",
			mp:          regularMap,
			path:        "/a/b/c/d/e",
			ok:          true,
			longestPath: "/a/b/c/d/e",
		},
		// When considering divergent paths (i.e. paths p where the path map has
		// neither an entry associated with p nor any entry associated with a
		// descendent path of p), they may diverge from the map nodes at a node
		// representing an entry or they may diverge from a node representing a
		// non-entry.
		{
			name:        "no exact match, no descendents, ancestor, stray from "+
			             "internal entry",
			mp:          regularMap,
			path:        "/a/b/f",
			ok:          true,
			longestPath: "/a/b",
		},
		{
			name:        "no exact match, no descendents, ancestor, stray two entries "+
			             "from internal entry",
			mp:          regularMap,
			path:        "/a/b/f/g",
			ok:          true,
			longestPath: "/a/b",
		},
		{
			name:        "no exact match, no descendents, ancestor, stray from leaf "+
			             "entry",
			mp:          regularMap,
			path:        "/a/b/c/d/e/f",
			ok:          true,
			longestPath: "/a/b/c/d/e",
		},
		{
			name:        "no exact match, no descendents, ancestor, stray two entries "+
			             "from leaf entry",
			mp:          regularMap,
			path:        "/a/b/c/d/e/f/g",
			ok:          true,
			longestPath: "/a/b/c/d/e",
		},
		{
			name:        "no exact match, no descendents, ancestor, stray from "+
			             "internal non-entry",
			mp:          regularMap,
			path:        "/a/b/c/f",
			ok:          true,
			longestPath: "/a/b",
		},
		{
			name:        "no exact match, no descendents, ancestor, stray two entries "+
			             "from internal non-entry",
			mp:          regularMap,
			path:        "/a/b/c/f",
			ok:          true,
			longestPath: "/a/b",
		},

		// Non-root paths when the path map has no entries associated with shorter
		// prefixes except for an entry associated with the root path
		{
			name:        "exact match, descendents, ancestor is root",
			mp:          regularMap,
			path:        "/r/s",
			ok:          true,
			longestPath: "/r/s",
		},
		{
			name:        "no exact match, descendents, ancestor is root",
			mp:          regularMap,
			path:        "/r",
			ok:          true,
			longestPath: "/",
		},
		{
			name:        "exact match, no descendents, ancestor is root",
			mp:          regularMap,
			path:        "/u/v",
			ok:          true,
			longestPath: "/u/v",
		},
		{
			name:        "no exact match, no descendents, ancestor is root",
			mp:          regularMap,
			path:        "/x/y/z",
			ok:          true,
			longestPath: "/",
		},

		// Non-root paths when the path map has no entries associated with shorter
		// prefixes
		{
			name:        "exact match, descendents, no ancestor",
			mp:          noEntryAtRootMap,
			path:        "/r/s",
			ok:          true,
			longestPath: "/r/s",
		},
		{
			name:        "no exact match, descendents, no ancestor",
			mp:          noEntryAtRootMap,
			path:        "/r",
			ok:          false,
			longestPath: "",
		},
		{
			name:        "exact match, no descendents, no ancestor",
			mp:          noEntryAtRootMap,
			path:        "/u/v",
			ok:          true,
			longestPath: "/u/v",
		},
		{
			name:        "no exact match, no descendents, no ancestor",
			mp:          noEntryAtRootMap,
			path:        "/x/y/z",
			ok:          false,
			longestPath: "",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Ensure single canonical representation of test case struct.
			if !tc.ok && tc.longestPath != "" {
				t.Fatalf(
					"Test case %q expects ok == false but has a configured "+
					"longestPath value of %q. Please clear this.",
					tc.name,
					tc.longestPath,
				)
			}

			// Dump details of test case for easy access on failure.
			defer func() {
				if t.Failed() {
					t.Logf("Failed with test case: %+v", tc)
				}
			}()

			inputPath := FromString(tc.path)

			t.Logf("Running GetLongestPrefix with %v", inputPath)
			longestPrefix, v, ok := tc.mp.pathMap.GetLongestPrefix(inputPath)

			if ok != tc.ok {
				t.Fatalf("Unexpected ok value; expected:%v actual:%v", tc.ok, ok)
			}

			if !ok {
				if !Equal(longestPrefix, nil) {
					// Note: path.Equal([]key.Key{}, nil) == true.
					t.Errorf("Unexpected non-empty longestPrefix: %v", longestPrefix)
				}
				if v != nil {
					t.Errorf(
						"Expected zero-value (nil); received unexpected value: %v",
						v,
					)
				}
			} else {
				expectedlongestPrefix := FromString(tc.longestPath)
				if !Equal(longestPrefix, expectedlongestPrefix) {
					t.Errorf(
						"Unexpected longestPrefix; expected:%v actual:%v",
						expectedlongestPrefix,
						longestPrefix,
					)
				}

				expectedValue := tc.mp.expectedValues[tc.longestPath]
				if v != expectedValue {
					t.Errorf(
						"Unexpected entry value; expected:%v actual:%v",
						expectedValue,
						v,
					)
				}
			}
		})
	}
}

func countNodes(m *Map) int {
	if m == nil {
		return 0
	}
	count := 1
	count += countNodes(m.wildcard)
	for it := m.children.Iter(); it.Next(); {
		count += countNodes(it.Elem())
	}
	return count
}

func TestMapDelete(t *testing.T) {
	m := Map{}
	m.Set(key.Path{}, 0)
	m.Set(key.Path{Wildcard}, 1)
	m.Set(key.Path{key.New("foo"), key.New("bar")}, 2)
	m.Set(key.Path{key.New("foo"), Wildcard}, 3)
	m.Set(key.Path{key.New("foo")}, 4)

	n := countNodes(&m)
	if n != 5 {
		t.Errorf("Initial count wrong. Expected: 5, Got: %d", n)
	}

	testCases := []struct {
		del      key.Path    // key.Path to delete
		expected bool        // expected return value of Delete
		visit    key.Path    // key.Path to Visit
		before   map[int]int // Expected to find items before deletion
		after    map[int]int // Expected to find items after deletion
		count    int         // Count of nodes
	}{{
		del:      key.Path{key.New("zap")}, // A no-op Delete
		expected: false,
		visit:    key.Path{key.New("foo"), key.New("bar")},
		before:   map[int]int{2: 1, 3: 1},
		after:    map[int]int{2: 1, 3: 1},
		count:    5,
	}, {
		del:      key.Path{key.New("foo"), key.New("bar")},
		expected: true,
		visit:    key.Path{key.New("foo"), key.New("bar")},
		before:   map[int]int{2: 1, 3: 1},
		after:    map[int]int{3: 1},
		count:    4,
	}, {
		del:      key.Path{key.New("foo")},
		expected: true,
		visit:    key.Path{key.New("foo")},
		before:   map[int]int{1: 1, 4: 1},
		after:    map[int]int{1: 1},
		count:    4,
	}, {
		del:      key.Path{key.New("foo")},
		expected: false,
		visit:    key.Path{key.New("foo")},
		before:   map[int]int{1: 1},
		after:    map[int]int{1: 1},
		count:    4,
	}, {
		del:      key.Path{Wildcard},
		expected: true,
		visit:    key.Path{key.New("foo")},
		before:   map[int]int{1: 1},
		after:    map[int]int{},
		count:    3,
	}, {
		del:      key.Path{Wildcard},
		expected: false,
		visit:    key.Path{key.New("foo")},
		before:   map[int]int{},
		after:    map[int]int{},
		count:    3,
	}, {
		del:      key.Path{key.New("foo"), Wildcard},
		expected: true,
		visit:    key.Path{key.New("foo"), key.New("bar")},
		before:   map[int]int{3: 1},
		after:    map[int]int{},
		count:    1, // Should have deleted "foo" and "bar" nodes
	}, {
		del:      key.Path{},
		expected: true,
		visit:    key.Path{},
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

func TestMapVisitPrefixes(t *testing.T) {
	m := Map{}
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

func TestMapVisitPrefixed(t *testing.T) {
	m := Map{}
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

func TestMapVisitChildren(t *testing.T) {
	m := Map{}
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
	m.Set(key.Path{Wildcard, key.New("bar"), key.New("quux")}, 10)
	m.Set(key.Path{key.New("quux"), key.New("quux"), key.New("quux"), key.New("quux")}, 11)
	m.Set(key.Path{key.New("a"), key.New("b"), key.New("c"), key.New("d")}, 12)
	m.Set(key.Path{key.New("a"), key.New("b")}, 13)

	testCases := []struct {
		path     key.Path
		expected map[int]int
	}{{
		path:     key.Path{key.New("foo"), key.New("bar"), key.New("baz")},
		expected: map[int]int{4: 1},
	}, {
		path:     key.Path{key.New("zip"), key.New("zap")},
		expected: map[int]int{},
	}, {
		path:     key.Path{key.New("foo"), key.New("bar")},
		expected: map[int]int{3: 1, 10: 1},
	}, {
		path:     key.Path{key.New("quux"), key.New("quux"), key.New("quux")},
		expected: map[int]int{11: 1},
	}, {
		path:     key.Path{key.New("a"), key.New("b")},
		expected: map[int]int{},
	}}

	for _, tc := range testCases {
		result := make(map[int]int, len(tc.expected))
		m.VisitChildren(tc.path, accumulator(result))
		if diff := test.Diff(tc.expected, result); diff != "" {
			t.Errorf("Test case %v: %s", tc.path, diff)
			t.Errorf("tc.expected: %#v, got %#v", tc.expected, result)
		}
	}
}

func TestMapString(t *testing.T) {
	m := Map{}
	m.Set(key.Path{}, 0)
	m.Set(key.Path{key.New("foo"), key.New("bar")}, 1)
	m.Set(key.Path{key.New("foo"), key.New("quux")}, 2)
	m.Set(key.Path{key.New("foo"), Wildcard}, 3)

	expected := `Val: 0
Child "foo":
  Child "*":
    Val: 3
  Child "bar":
    Val: 1
  Child "quux":
    Val: 2
`
	got := fmt.Sprint(&m)

	if expected != got {
		t.Errorf("Unexpected string. Expected:\n\n%s\n\nGot:\n\n%s", expected, got)
	}
}

func genWords(count, wordLength int) key.Path {
	chars := []byte("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789")
	if count+wordLength > len(chars) {
		panic("need more chars")
	}
	result := make(key.Path, count)
	for i := 0; i < count; i++ {
		result[i] = key.New(string(chars[i : i+wordLength]))
	}
	return result
}

func benchmarkPathMap(pathLength, pathDepth int, b *testing.B) {
	// Push pathDepth paths, each of length pathLength
	path := genWords(pathLength, 10)
	words := genWords(pathDepth, 10)
	root := &Map{}
	m := root
	for _, element := range path {
		m.children = newKeyMap[any]()
		for _, word := range words {
			m.children.Set(word, &Map{})
		}
		next, _ := m.children.Get(element)
		m = next
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		root.Visit(path, func(v interface{}) error { return nil })
	}
}

func BenchmarkPathMap1x25(b *testing.B)  { benchmarkPathMap(1, 25, b) }
func BenchmarkPathMap10x50(b *testing.B) { benchmarkPathMap(10, 25, b) }
func BenchmarkPathMap20x50(b *testing.B) { benchmarkPathMap(20, 25, b) }
