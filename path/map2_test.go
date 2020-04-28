// Copyright (c) 2020 Arista Networks, Inc.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the COPYING file.

package path

import (
	"strings"
	"testing"

	"github.com/aristanetworks/goarista/key"
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
