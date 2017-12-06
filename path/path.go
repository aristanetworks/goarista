// Copyright (c) 2017 Arista Networks, Inc.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the COPYING file.

// Package path provides functionality for dealing with absolute paths elementally.
package path

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/aristanetworks/goarista/key"
)

// Path is an absolute path broken down into elements where each element is a key.Key.
type Path []key.Key

func copyElements(path Path, elements ...interface{}) {
	for i, element := range elements {
		switch val := element.(type) {
		case string:
			path[i] = key.New(val)
		case key.Key:
			path[i] = val
		default:
			panic(fmt.Errorf("unsupported type: %T", element))
		}
	}
}

// New constructs a Path from a variable number of elements.
// Each element may either be a string or a key.Key.
func New(elements ...interface{}) Path {
	path := make(Path, len(elements))
	copyElements(path, elements...)
	return path
}

// FromString constructs a Path from the elements resulting
// from a split of the input string by "/". The string MUST
// begin with a '/' character unless it is the empty string
// in which case an empty Path is returned.
func FromString(str string) Path {
	if str == "" {
		return Path{}
	} else if str[0] != '/' {
		panic(fmt.Errorf("not an absolute path: %q", str))
	}
	elements := strings.Split(str, "/")[1:]
	path := make(Path, len(elements))
	for i, element := range elements {
		path[i] = key.New(element)
	}
	return path
}

// Append appends a variable number of elements to a Path.
// Each element may either be a string or a key.Key.
func Append(path Path, elements ...interface{}) Path {
	if len(elements) == 0 {
		return path
	}
	n := len(path)
	p := make(Path, n+len(elements))
	copy(p, path)
	copyElements(p[n:], elements...)
	return p
}

// String returns the Path as a string.
func (p Path) String() string {
	if len(p) == 0 {
		return ""
	}
	var buf bytes.Buffer
	for _, element := range p {
		buf.WriteByte('/')
		buf.WriteString(element.String())
	}
	return buf.String()
}

// Equal returns whether the Path contains the same elements as the other Path.
// This method implements key.Comparable.
func (p Path) Equal(other interface{}) bool {
	o, ok := other.(Path)
	if !ok {
		return false
	}
	if len(o) != len(p) {
		return false
	}
	return o.hasPrefix(p)
}

// HasPrefix returns whether the Path is prefixed by the other Path.
func (p Path) HasPrefix(prefix Path) bool {
	if len(prefix) > len(p) {
		return false
	}
	return p.hasPrefix(prefix)
}

func (p Path) hasPrefix(prefix Path) bool {
	for i := range prefix {
		if !prefix[i].Equal(p[i]) {
			return false
		}
	}
	return true
}
