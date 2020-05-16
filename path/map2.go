// Copyright (c) 2020 Arista Networks, Inc.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the COPYING file.

package path

import (
	"fmt"
	"sort"
	"strings"

	"github.com/aristanetworks/goarista/key"
)

// map2 is a more efficient implementation of Map with the same
// API. When it is complete map2 will replace Map.
type map2 struct {
	n node
}

type node struct {
	p key.Path

	wildcard *node
	children *key.Map

	val interface{}
	ok  bool
}

// Get returns the value registered with an exact match of a
// path p. If there is no exact match for p, Get returns nil
// and false. If p has an exact match and it is set to true,
// Get returns nil and true.
func (m *map2) Get(p key.Path) (interface{}, bool) {
	n, remaining, index := m.n.find(p)
	if !isMatch(n, remaining, index) {
		return nil, false
	}
	return n.val, n.ok
}

// Set registers a path p with a value. If the path was already
// registered with a value it returns false and true otherwise.
func (m *map2) Set(p key.Path, v interface{}) bool {
	n, remaining, index := m.n.find(p)
	if isMatch(n, remaining, index) {
		return n.set(v)
	}
	n.insert(remaining, index, v)
	return true
}

// Delete unregisters the value registered with a path. It
// returns true if a value was deleted and false otherwise.
func (m *map2) Delete(p key.Path) bool {
	return m.n.remove(p)
}

// IsEmpty returns true if no paths have been registered, false otherwise.
func (m *map2) IsEmpty() bool {
	return m.n.isEmpty()
}

func commonPrefix(a, b key.Path) int {
	if len(a) > len(b) {
		a, b = b, a
	}
	for i, ai := range a {
		if !ai.Equal(b[i]) {
			return i
		}
	}
	return len(a)
}

func commonPrefixFuzzy(pattern, p key.Path) int {
	var i int
	for ; i < len(pattern) && i < len(p) &&
		(pattern[i].Equal(Wildcard) || pattern[i].Equal(p[i])); i++ {
	}
	return i
}

func (n *node) set(val interface{}) bool {
	set := !n.ok
	n.val, n.ok = val, true
	return set
}

func isMatch(n *node, remaining key.Path, index int) bool {
	return len(remaining) == 0 && index == len(n.p)
}

// find returns the node matching this path, or the closest parent,
// along with the unmatched portion of the path and the index of n.p
// where it differs. Call isMatch on the returned values to discover
// if a match is found.
func (n *node) find(p key.Path) (*node, key.Path, int) {
	var c int
	if len(n.p) > 0 {
		// We know that the first element matches, because find
		// recursively calls find on nodes where the first element is
		// a match. This is only not true for find called on the root
		// node, which has an empty path.
		c = commonPrefix(n.p[1:], p[1:]) + 1
	}

	// Consume matched portion of p
	p = p[c:]
	if c < len(n.p) || len(p) == 0 {
		// An element was not equal or all of p has been consumed
		return n, p, c
	}

	next := p[0]
	if next.Equal(Wildcard) {
		if n.wildcard == nil {
			return n, p, c
		}
		return n.wildcard.find(p)
	}

	child, ok := n.children.Get(next)
	if !ok {
		return n, p, c
	}
	return child.(*node).find(p)
}

// insert takes in a path and val to be inserted on n at index.
func (n *node) insert(p key.Path, index int, val interface{}) {
	if index < len(n.p) {
		n.split(index)
	}

	if index != len(n.p) {
		panic("invariant violated")
	}
	if len(p) == 0 {
		// Just inserting a value
		n.set(val)
		return
	}

	newNode := &node{
		p:   Clone(p), // p came from a user, so it should be cloned.
		val: val,
		ok:  true,
	}

	next := p[0]
	if next.Equal(Wildcard) {
		n.wildcard = newNode
		return
	}
	if n.children == nil {
		n.children = key.NewMap()
	}
	n.children.Set(next, newNode)
}

func (n *node) split(index int) {
	child := &node{
		// I'm 90% sure I don't need to Clone here. An issue could
		// arise due to the append that happens during join, which may
		// overwrite a child's path. But, I don't think that can
		// happen because we only join when there is only one child
		// left, so either we are joining with the node that
		// previously caused a split, or that node has already been
		// deleted so we don't care about its path.
		p:        n.p[index:],
		wildcard: n.wildcard,
		children: n.children,
		val:      n.val,
		ok:       n.ok,
	}

	// Clear this node
	n.p = n.p[:index]
	n.wildcard = nil
	n.children = nil
	n.val = nil
	n.ok = false

	// insert this new child node
	next := child.p[0]
	if next.Equal(Wildcard) {
		n.wildcard = child
	} else {
		n.children = key.NewMap(next, child)
	}
}

func (n *node) join(child *node) {
	n.p = append(n.p, child.p...)
	n.wildcard = child.wildcard
	n.children = child.children
	n.val = child.val
	n.ok = child.ok
}

func (n *node) remove(p key.Path) bool {
	var c int
	if len(n.p) > 0 {
		// We know that the first element matches, because find
		// recursively calls find on nodes where the first element is
		// a match. This is only not true for find called on the root
		// node, which has an empty path.
		c = commonPrefix(n.p[1:], p[1:]) + 1
	}

	if c < len(n.p) {
		// not found
		return false
	}
	if c == len(p) {
		// found node
		deleted := n.ok
		if n.wildcard != nil && n.children.Len() == 0 {
			n.join(n.wildcard)
		} else if n.wildcard == nil && n.children.Len() == 1 {
			var child *node
			_ = n.children.Iter(func(_, v interface{}) error {
				child = v.(*node)
				return nil
			})
			n.join(child)
		} else {
			// n has multiple children or no children. Unset n. If n
			// becomes empty the recursive caller will remove this
			// node.
			n.val = nil
			n.ok = false
		}
		return deleted
	}

	// Consume matched portion of p
	p = p[c:]

	next := p[0]
	if next.Equal(Wildcard) {
		if n.wildcard == nil {
			return false
		}

		deleted := n.wildcard.remove(p)
		if n.wildcard.isEmpty() {
			n.wildcard = nil
		}
		return deleted
	}

	childV, ok := n.children.Get(next)
	if !ok {
		return false
	}
	child := childV.(*node)

	deleted := child.remove(p)
	if child.isEmpty() {
		n.children.Del(next)
	}
	return deleted
}

func (n *node) isEmpty() bool {
	return !n.ok && n.wildcard == nil && n.children.Len() == 0
}

// Visit calls a function fn for every value in the Map
// that is registered with a match of a path p. In the
// general case, time complexity is linear with respect
// to the length of p but it can be as bad as O(2^len(p))
// if there are a lot of paths with wildcards registered.
func (m *map2) Visit(p key.Path, fn VisitorFunc) error {
	return m.n.visit(match, p, fn)
}

// VisitPrefixes calls a function fn for every value in the
// Map that is registered with a prefix of a path p.
func (m *map2) VisitPrefixes(p key.Path, fn VisitorFunc) error {
	return m.n.visit(prefix, p, fn)
}

// VisitPrefixed calls fn for every value in the map that is
// registerd with a path that is prefixed by p. This method
// can be used to visit every registered path if p is the
// empty path (or root path) which prefixes all paths.
func (m *map2) VisitPrefixed(p key.Path, fn VisitorFunc) error {
	return m.n.visit(suffix, p, fn)
}

func (n *node) visit(typ visitType, p key.Path, fn VisitorFunc) error {
	var c int
	if len(n.p) > 0 {
		// We know that the first element matches, because find
		// recursively calls find on nodes where the first element is
		// a match. This is only not true for find called on the root
		// node, which has an empty path.
		c = commonPrefixFuzzy(n.p[1:], p[1:]) + 1
	}

	// Consume matched portion of p
	p = p[c:]
	if len(p) == 0 {
		if typ == suffix {
			return n.visitAll(fn)
		}

		if c == len(n.p) && n.ok {
			return fn(n.val)
		}
		// Didn't match all of n or n doesn't hold a value
		return nil
	}
	if c < len(n.p) {
		return nil
	}

	// Matched all of n
	if typ == prefix && n.ok {
		if err := fn(n.val); err != nil {
			return err
		}
	}

	if n.wildcard != nil {
		if err := n.wildcard.visit(typ, p, fn); err != nil {
			return err
		}
	}
	child, ok := n.children.Get(p[0])
	if !ok {
		return nil
	}
	return child.(*node).visit(typ, p, fn)
}

func (n *node) visitAll(fn VisitorFunc) error {
	if n.ok {
		if err := fn(n.val); err != nil {
			return err
		}
	}
	if n.wildcard != nil {
		if err := n.wildcard.visitAll(fn); err != nil {
			return err
		}
	}

	return n.children.Iter(func(_, child interface{}) error {
		return child.(*node).visitAll(fn)
	})
}

func (m *map2) String() string {
	paths := m.n.collectPaths(nil, nil)
	sort.Strings(paths)
	return strings.Join(paths, "\n")
}

func (n *node) collectPaths(cur key.Path, paths []string) []string {
	cur = append(cur, n.p...)
	if n.ok {
		paths = append(paths, fmt.Sprintf("%s: %v", cur, n.val))
	}
	if n.wildcard != nil {
		paths = n.wildcard.collectPaths(cur, paths)
	}

	_ = n.children.Iter(func(_, child interface{}) error {
		paths = child.(*node).collectPaths(cur, paths)
		return nil
	})
	return paths
}
