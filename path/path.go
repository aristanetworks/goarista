// Copyright (c) 2017 Arista Networks, Inc.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the COPYING file.

// Package path contains methods for dealing with elemental paths.
package path

import (
	"bytes"
	"strings"

	"github.com/aristanetworks/goarista/key"
)

// Path represents a path decomposed into elements where each
// element is a key.Key. A Path can be interpreted as either
// absolute or relative depending on how it is used.
type Path []key.Key

// New constructs a Path from a variable number of elements.
// Each element may either be a key.Key or a value that can
// be wrapped by a key.Key.
func New(elements ...interface{}) Path {
	result := make(Path, len(elements))
	copyElements(result, elements...)
	return result
}

// Append appends a variable number of elements to a Path.
// Each element may either be a key.Key or a value that can
// be wrapped by a key.Key. Note that calling Append on a
// single Path returns that same Path, whereas in all other
// cases a new Path is returned.
func Append(path Path, elements ...interface{}) Path {
	if len(elements) == 0 {
		return path
	}
	n := len(path)
	result := make(Path, n+len(elements))
	copy(result, path)
	copyElements(result[n:], elements...)
	return result
}

// Join joins a variable number of Paths together. Each path
// in the joining is treated as a subpath of its predecessor.
// Calling Join with no or only empty paths returns nil.
func Join(paths ...Path) Path {
	n := 0
	for _, path := range paths {
		n += len(path)
	}
	if n == 0 {
		return nil
	}
	result, i := make(Path, n), 0
	for _, path := range paths {
		i += copy(result[i:], path)
	}
	return result
}

// Parent returns all but the last element of the Path. If
// the Path is empty, Parent returns nil.
func Parent(path Path) Path {
	if len(path) > 0 {
		return path[:len(path)-1]
	}
	return nil
}

// Base returns the last element of the Path. If the Path is
// empty, Base returns nil.
func Base(path Path) key.Key {
	if len(path) > 0 {
		return path[len(path)-1]
	}
	return nil
}

// Clone returns a new Path with the same elements as in the
// provided Path.
func Clone(path Path) Path {
	result := make(Path, len(path))
	copy(result, path)
	return result
}

// Equal returns whether Path a and Path b are the same
// length and whether each element in b corresponds to the
// same element in a.
func Equal(a, b Path) bool {
	return len(a) == len(b) && hasPrefix(a, b)
}

// HasElement returns whether element b exists in Path a.
func HasElement(a Path, b key.Key) bool {
	for _, element := range a {
		if element.Equal(b) {
			return true
		}
	}
	return false
}

// HasPrefix returns whether Path b is at most the length
// of Path a and whether each element in b corresponds to
// the same element in a from the first element.
func HasPrefix(a, b Path) bool {
	return len(a) >= len(b) && hasPrefix(a, b)
}

// Match returns whether Path a and Path b are the same
// length and whether each element in b corresponds to the
// same element or a wildcard in a.
func Match(a, b Path) bool {
	return len(a) == len(b) && matchPrefix(a, b)
}

// MatchPrefix returns whether Path b is at most the length
// of Path a and whether each element in b corresponds to
// the same element or a wildcard in a from the first
// element.
func MatchPrefix(a, b Path) bool {
	return len(a) >= len(b) && matchPrefix(a, b)
}

// FromString constructs a Path from the elements resulting
// from a split of the input string by "/". Strings that do
// not lead with a '/' are accepted but not reconstructable
// with Path.String.
func FromString(str string) Path {
	if str == "" {
		return Path{}
	} else if str[0] == '/' {
		str = str[1:]
	}
	elements := strings.Split(str, "/")
	result := make(Path, len(elements))
	for i, element := range elements {
		result[i] = key.New(element)
	}
	return result
}

// String returns the Path as an absolute path string.
func (p Path) String() string {
	if len(p) == 0 {
		return "/"
	}
	var buf bytes.Buffer
	for _, element := range p {
		buf.WriteByte('/')
		buf.WriteString(element.String())
	}
	return buf.String()
}

func copyElements(dest Path, elements ...interface{}) {
	for i, element := range elements {
		switch val := element.(type) {
		case key.Key:
			dest[i] = val
		default:
			dest[i] = key.New(val)
		}
	}
}

func hasPrefix(a, b Path) bool {
	for i := range b {
		if !b[i].Equal(a[i]) {
			return false
		}
	}
	return true
}

func matchPrefix(a, b Path) bool {
	for i := range b {
		if !a[i].Equal(Wildcard) && !b[i].Equal(a[i]) {
			return false
		}
	}
	return true
}
