// Copyright (c) 2017 Arista Networks, Inc.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the COPYING file.

package path

import (
	"fmt"
	"sort"
	"strings"

	"github.com/aristanetworks/goarista/key"
	"github.com/aristanetworks/gomap"
)

// Map associates paths to values. It allows wildcards. A Map
// is primarily used to register handlers with paths that can
// be easily looked up each time a path is updated.
type Map = MapOf[any]

// VisitorFunc is a function that handles the value associated
// with a path in a Map. Note that only the value is passed in
// as an argument since the path can be stored inside the value
// if needed.
type VisitorFunc func(v any) error

type visitType int

const (
	match visitType = iota
	prefix
	suffix
	children
)

// MapOf associates paths to values of type T. It allows wildcards. A
// Map is primarily used to register handlers with paths that can be
// easily looked up each time a path is updated.
type MapOf[T any] struct {
	val      T
	ok       bool
	wildcard *MapOf[T]
	children *gomap.Map[key.Key, *MapOf[T]]
}

// Visit calls a function fn for every value in the Map
// that is registered with a match of a path p. In the
// general case, time complexity is linear with respect
// to the length of p but it can be as bad as O(2^len(p))
// if there are a lot of paths with wildcards registered.
func (m *MapOf[T]) Visit(p key.Path, fn func(v T) error) error {
	return m.visit(match, p, fn)
}

// VisitPrefixes calls a function fn for every value in the
// Map that is registered with a prefix of a path p.
func (m *MapOf[T]) VisitPrefixes(p key.Path, fn func(v T) error) error {
	return m.visit(prefix, p, fn)
}

// VisitPrefixed calls fn for every value in the map that is
// registerd with a path that is prefixed by p. This method
// can be used to visit every registered path if p is the
// empty path (or root path) which prefixes all paths.
func (m *MapOf[T]) VisitPrefixed(p key.Path, fn func(v T) error) error {
	return m.visit(suffix, p, fn)
}

// VisitChildren calls fn for every child for every node that
// matches the provided path.
func (m *MapOf[T]) VisitChildren(p key.Path, fn func(v T) error) error {
	return m.visit(children, p, fn)
}

func (m *MapOf[T]) visit(typ visitType, p key.Path, fn func(v T) error) error {
	for i, element := range p {
		if m.ok && typ == prefix {
			if err := fn(m.val); err != nil {
				return err
			}
		}
		if m.wildcard != nil {
			if err := m.wildcard.visit(typ, p[i+1:], fn); err != nil {
				return err
			}
		}
		next, ok := m.children.Get(element)
		if !ok {
			return nil
		}
		m = next
	}
	if typ == children {
		for it := m.children.Iter(); it.Next(); {
			if it.Elem().ok {
				if err := fn(it.Elem().val); err != nil {
					return err
				}
			}
		}
		return nil
	}
	if typ == suffix {
		return m.visitSubtree(fn)
	}
	if !m.ok {
		return nil
	}
	return fn(m.val)
}

func (m *MapOf[T]) visitSubtree(fn func(v T) error) error {
	if m.ok {
		if err := fn(m.val); err != nil {
			return err
		}
	}
	if m.wildcard != nil {
		if err := m.wildcard.visitSubtree(fn); err != nil {
			return err
		}
	}
	for it := m.children.Iter(); it.Next(); {
		if err := it.Elem().visitSubtree(fn); err != nil {
			return err
		}
	}
	return nil
}

// IsEmpty returns true if no paths have been registered, false otherwise.
func (m *MapOf[T]) IsEmpty() bool {
	return m.wildcard == nil && m.children.Len() == 0 && !m.ok
}

// Get returns the value registered with an exact match of a path p.
// If there is no exact match for p, Get returns the zero value and false.
// If p has an exact match and it is set to true, Get
// returns its value and true.
func (m *MapOf[T]) Get(p key.Path) (T, bool) {
	var zeroT T
	for _, element := range p {
		if element.Equal(Wildcard) {
			if m.wildcard == nil {
				return zeroT, false
			}
			m = m.wildcard
			continue
		}
		next, ok := m.children.Get(element)
		if !ok {
			return zeroT, false
		}
		m = next
	}
	return m.val, m.ok
}

// GetLongestPrefix determines the longest prefix of p for which an entry exists
// within the path map. If such a prefix exists, this function returns the prefix
// path, its associated value, and true. Otherwise, this functions returns the empty
// path, the zero value of T, and false.
func (m *MapOf[T]) GetLongestPrefix(p key.Path) (key.Path, T, bool) {
	foundPrefixEntry := m.ok
	prefixEntryPathLen := 0
	prefixEntryNode := m

	for i, element := range p {
		next, existsNode := m.children.Get(element)
		if !existsNode {
			// Next path element from p does not have an associated map node; return
			// values corresponding to the longest prefix (with an entry in the map)
			// visited thus far.
			break
		}

		if next.ok {
			// Found a new entry with a longer prefix; record the details for returning
			// after the loop.
			foundPrefixEntry = true
			prefixEntryPathLen = i + 1
			prefixEntryNode = next
		}

		m = next
	}

	return p[:prefixEntryPathLen], prefixEntryNode.val, foundPrefixEntry
}

func newKeyMap[T any]() *gomap.Map[key.Key, *MapOf[T]] {
	return gomap.New[key.Key, *MapOf[T]](func(a, b key.Key) bool { return a.Equal(b) }, key.Hash)
}

// Set registers a path p with a value. If the path was already
// registered with a value it returns false and true otherwise.
func (m *MapOf[T]) Set(p key.Path, v T) bool {
	for _, element := range p {
		if element.Equal(Wildcard) {
			if m.wildcard == nil {
				m.wildcard = &MapOf[T]{}
			}
			m = m.wildcard
			continue
		}
		if m.children == nil {
			m.children = newKeyMap[T]()
		}
		next, ok := m.children.Get(element)
		if !ok {
			next = &MapOf[T]{}
			m.children.Set(element, next)
		}
		m = next
	}
	set := !m.ok
	m.val, m.ok = v, true
	return set
}

// Delete unregisters the value registered with a path. It
// returns true if a value was deleted and false otherwise.
func (m *MapOf[T]) Delete(p key.Path) bool {
	maps := make([]*MapOf[T], len(p)+1)
	for i, element := range p {
		maps[i] = m
		if element.Equal(Wildcard) {
			if m.wildcard == nil {
				return false
			}
			m = m.wildcard
			continue
		}
		next, ok := m.children.Get(element)
		if !ok {
			return false
		}
		m = next
	}
	deleted := m.ok
	var zeroT T
	m.val, m.ok = zeroT, false
	maps[len(p)] = m

	// Remove any empty maps.
	for i := len(p); i > 0; i-- {
		m = maps[i]
		if m.ok || m.wildcard != nil || m.children.Len() > 0 {
			break
		}
		parent := maps[i-1]
		element := p[i-1]
		if element.Equal(Wildcard) {
			parent.wildcard = nil
		} else {
			parent.children.Delete(element)
		}
	}
	return deleted
}

func (m *MapOf[T]) String() string {
	var b strings.Builder
	m.write(&b, "")
	return b.String()
}

func (m *MapOf[T]) write(b *strings.Builder, indent string) {
	if m.ok {
		b.WriteString(indent)
		fmt.Fprintf(b, "Val: %v", m.val)
		b.WriteString("\n")
	}
	if m.wildcard != nil {
		b.WriteString(indent)
		fmt.Fprintf(b, "Child %q:\n", Wildcard)
		m.wildcard.write(b, indent+"  ")
	}
	children := make([]key.Key, 0, m.children.Len())
	for it := m.children.Iter(); it.Next(); {
		children = append(children, it.Key())
	}
	sort.Slice(children, func(i, j int) bool {
		return children[i].String() < children[j].String()
	})

	for _, key := range children {
		child, _ := m.children.Get(key)
		b.WriteString(indent)
		fmt.Fprintf(b, "Child %q:\n", key.String())
		child.write(b, indent+"  ")
	}
}
