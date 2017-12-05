// Copyright (c) 2017 Arista Networks, Inc.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the COPYING file.

package path

import (
	"fmt"
	"testing"

	"github.com/aristanetworks/goarista/key"
	"github.com/aristanetworks/goarista/value"
)

func TestNewPath(t *testing.T) {
	tcases := []struct {
		in  []interface{}
		out Path
	}{
		{
			in:  nil,
			out: nil,
		}, {
			in:  []interface{}{},
			out: Path{},
		}, {
			in:  []interface{}{""},
			out: Path{key.New("")},
		}, {
			in:  []interface{}{key.New("")},
			out: Path{key.New("")},
		}, {
			in:  []interface{}{"foo"},
			out: Path{key.New("foo")},
		}, {
			in:  []interface{}{key.New("foo")},
			out: Path{key.New("foo")},
		}, {
			in:  []interface{}{"foo", key.New("bar")},
			out: Path{key.New("foo"), key.New("bar")},
		}, {
			in:  []interface{}{key.New("foo"), "bar", key.New("baz")},
			out: Path{key.New("foo"), key.New("bar"), key.New("baz")},
		},
	}
	for i, tcase := range tcases {
		if p := New(tcase.in...); !p.Equal(tcase.out) {
			t.Fatalf("Test %d failed: %#v != %#v", i, p, tcase.out)
		}
	}
}

func TestAppendPath(t *testing.T) {
	tcases := []struct {
		base     Path
		elements []interface{}
		expected Path
	}{
		{
			base:     Path{},
			elements: []interface{}{},
			expected: Path{},
		}, {
			base:     Path{},
			elements: []interface{}{""},
			expected: Path{key.New("")},
		}, {
			base:     Path{},
			elements: []interface{}{key.New("")},
			expected: Path{key.New("")},
		}, {
			base:     Path{},
			elements: []interface{}{"foo", key.New("bar")},
			expected: Path{key.New("foo"), key.New("bar")},
		}, {
			base:     Path{key.New("foo")},
			elements: []interface{}{key.New("bar"), "baz"},
			expected: Path{key.New("foo"), key.New("bar"), key.New("baz")},
		}, {
			base:     Path{key.New("foo"), key.New("bar")},
			elements: []interface{}{key.New("baz")},
			expected: Path{key.New("foo"), key.New("bar"), key.New("baz")},
		},
	}
	for i, tcase := range tcases {
		if p := Append(tcase.base, tcase.elements...); !p.Equal(tcase.expected) {
			t.Fatalf("Test %d failed: %#v != %#v", i, p, tcase.expected)
		}
	}
}

type customKey struct {
	i *int
}

func (c customKey) String() string {
	return fmt.Sprintf("customKey=%d", *c.i)
}

func (c customKey) MarshalJSON() ([]byte, error) {
	return nil, nil
}

func (c customKey) ToBuiltin() interface{} {
	return nil
}

func (c customKey) Equal(other interface{}) bool {
	o, ok := other.(customKey)
	return ok && *c.i == *o.i
}

var (
	_ value.Value    = customKey{}
	_ key.Comparable = customKey{}
	a                = 1
	b                = 1
)

func TestPathEquality(t *testing.T) {
	tcases := []struct {
		base     Path
		other    Path
		expected bool
	}{
		{
			base:     Path{},
			other:    Path{},
			expected: true,
		}, {
			base:     Path{},
			other:    Path{key.New("")},
			expected: false,
		}, {
			base:     Path{key.New("foo")},
			other:    Path{key.New("foo")},
			expected: true,
		}, {
			base:     Path{key.New("foo")},
			other:    Path{key.New("bar")},
			expected: false,
		}, {
			base:     Path{key.New("foo"), key.New("bar")},
			other:    Path{key.New("foo")},
			expected: false,
		}, {
			base:     Path{key.New("foo"), key.New("bar")},
			other:    Path{key.New("bar"), key.New("foo")},
			expected: false,
		}, {
			base:     Path{key.New("foo"), key.New("bar"), key.New("baz")},
			other:    Path{key.New("foo"), key.New("bar"), key.New("baz")},
			expected: true,
		},
		// Ensure that we check deep equality.
		{
			base:     Path{key.New(map[string]interface{}{})},
			other:    Path{key.New(map[string]interface{}{})},
			expected: true,
		}, {
			base:     Path{key.New(customKey{i: &a})},
			other:    Path{key.New(customKey{i: &b})},
			expected: true,
		},
	}
	for i, tcase := range tcases {
		if result := tcase.base.Equal(tcase.other); result != tcase.expected {
			t.Fatalf("Test %d failed: base: %#v; other: %#v, expected: %t",
				i, tcase.base, tcase.other, tcase.expected)
		}
	}
}

func TestPathHasPrefix(t *testing.T) {
	tcases := []struct {
		base     Path
		prefix   Path
		expected bool
	}{
		{
			base:     Path{},
			prefix:   Path{},
			expected: true,
		}, {
			base:     Path{key.New("foo")},
			prefix:   Path{},
			expected: true,
		}, {
			base:     Path{key.New("foo"), key.New("bar")},
			prefix:   Path{key.New("foo")},
			expected: true,
		}, {
			base:     Path{key.New("foo"), key.New("bar")},
			prefix:   Path{key.New("bar")},
			expected: false,
		}, {
			base:     Path{key.New("foo"), key.New("bar")},
			prefix:   Path{key.New("bar"), key.New("foo")},
			expected: false,
		}, {
			base:     Path{key.New("foo"), key.New("bar")},
			prefix:   Path{key.New("foo"), key.New("bar")},
			expected: true,
		}, {
			base:     Path{key.New("foo"), key.New("bar")},
			prefix:   Path{key.New("foo"), key.New("bar"), key.New("baz")},
			expected: false,
		},
	}
	for i, tcase := range tcases {
		if result := tcase.base.HasPrefix(tcase.prefix); result != tcase.expected {
			t.Fatalf("Test %d failed: base: %#v; prefix: %#v, expected: %t",
				i, tcase.base, tcase.prefix, tcase.expected)
		}
	}
}

func TestPathFromString(t *testing.T) {
	tcases := []struct {
		in  string
		out Path
	}{
		{
			in:  "",
			out: Path{},
		}, {
			in:  "/",
			out: Path{key.New("")},
		}, {
			in:  "//",
			out: Path{key.New(""), key.New("")},
		}, {
			in:  "/foo",
			out: Path{key.New("foo")},
		}, {
			in:  "/foo/bar",
			out: Path{key.New("foo"), key.New("bar")},
		}, {
			in:  "/foo/bar/baz",
			out: Path{key.New("foo"), key.New("bar"), key.New("baz")},
		}, {
			in:  "/0/123/456/789",
			out: Path{key.New("0"), key.New("123"), key.New("456"), key.New("789")},
		}, {
			in:  "/`~!@#$%^&*()_+{}\\/|[];':\"<>?,./",
			out: Path{key.New("`~!@#$%^&*()_+{}\\"), key.New("|[];':\"<>?,."), key.New("")},
		},
	}
	for i, tcase := range tcases {
		if p := FromString(tcase.in); !p.Equal(tcase.out) {
			t.Fatalf("Test %d failed: %#v != %#v", i, p, tcase.out)
		}
	}
}

func TestPathToString(t *testing.T) {
	tcases := []struct {
		in  Path
		out string
	}{
		{
			in:  Path{},
			out: "",
		}, {
			in:  Path{key.New("")},
			out: "/",
		}, {
			in:  Path{key.New("foo")},
			out: "/foo",
		}, {
			in:  Path{key.New("foo"), key.New("bar")},
			out: "/foo/bar",
		}, {
			in:  Path{key.New("/foo"), key.New("bar")},
			out: "//foo/bar",
		}, {
			in:  Path{key.New("foo"), key.New("bar/")},
			out: "/foo/bar/",
		}, {
			in:  Path{key.New(""), key.New("foo"), key.New("bar")},
			out: "//foo/bar",
		}, {
			in:  Path{key.New("foo"), key.New("bar"), key.New("")},
			out: "/foo/bar/",
		}, {
			in:  Path{key.New("/"), key.New("foo"), key.New("bar")},
			out: "///foo/bar",
		}, {
			in:  Path{key.New("foo"), key.New("bar"), key.New("/")},
			out: "/foo/bar//",
		},
	}
	for i, tcase := range tcases {
		if s := tcase.in.String(); s != tcase.out {
			t.Fatalf("Test %d failed: %s != %s", i, s, tcase.out)
		}
	}
}
