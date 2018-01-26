// Copyright (c) 2017 Arista Networks, Inc.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the COPYING file.

package path

import (
	"bytes"
	"fmt"
	"sort"

	"github.com/aristanetworks/goarista/key"
	"github.com/aristanetworks/goarista/pathmap"
)

// Map associates paths to values. It allows wildcards. A Map
// is primarily used to register handlers with paths that can
// be easily looked up each time a path is updated.
type Map interface {
	// Visit calls a function fn for every value in the Map
	// that is registered with a match of a path p. In the
	// general case, time complexity is linear with respect
	// to the length of p but it can be as bad as O(2^len(p))
	// if there are a lot of paths with wildcards registered.
	//
	// Example:
	//
	// a := path.New("foo", "bar", "baz")
	// b := path.New("foo", path.Wildcard, "baz")
	// c := path.New(path.Wildcard, "bar", "baz")
	// d := path.New("foo", "bar", path.Wildcard)
	// e := path.New(path.Wildcard, path.Wildcard, "baz")
	// f := path.New(path.Wildcard, "bar", path.Wildcard)
	// g := path.New("foo", path.Wildcard, path.Wildcard)
	// h := path.New(path.Wildcard, path.Wildcard, path.Wildcard)
	//
	// m.Set(a, 1)
	// m.Set(b, 2)
	// m.Set(c, 3)
	// m.Set(d, 4)
	// m.Set(e, 5)
	// m.Set(f, 6)
	// m.Set(g, 7)
	// m.Set(h, 8)
	//
	// p := path.New("foo", "bar", "baz")
	//
	// m.Visit(p, fn)
	//
	// Result: fn(1), fn(2), fn(3), fn(4), fn(5), fn(6), fn(7) and fn(8)
	Visit(p Path, fn pathmap.VisitorFunc) error

	// VisitPrefix calls a function fn for every value in the
	// Map that is registered with a prefix of a path p.
	//
	// Example:
	//
	// a := path.New()
	// b := path.New("foo")
	// c := path.New("foo", "bar")
	// d := path.New("foo", "baz")
	// e := path.New(path.Wildcard, "bar")
	//
	// m.Set(a, 1)
	// m.Set(b, 2)
	// m.Set(c, 3)
	// m.Set(d, 4)
	// m.Set(e, 5)
	//
	// p := path.New("foo", "bar", "baz")
	//
	// m.VisitPrefix(p, fn)
	//
	// Result: fn(1), fn(2), fn(3), fn(5)
	VisitPrefix(p Path, fn pathmap.VisitorFunc) error

	// Get returns the value registered with an exact match of a
	// path p. If there is no exact match for p, Get returns nil.
	//
	// Example:
	//
	// m.Set(path.New("foo", "bar"), 1)
	//
	// a := m.Get(path.New("foo", "bar"))
	// b := m.Get(path.New("foo", path.Wildcard))
	//
	// Result: a == 1 and b == nil
	Get(p Path) interface{}

	// Set registers a path p with a value. Any previous value that
	// was registered with p is overwritten.
	//
	// Example:
	//
	// p := path.New("foo", "bar")
	//
	// m.Set(p, 0)
	// m.Set(p, 1)
	//
	// v := m.Get(p)
	//
	// Result: v == 1
	Set(p Path, v interface{})

	// Delete unregisters the value registered with a path. It
	// returns true if a value was deleted and false otherwise.
	//
	// Example:
	//
	// p := path.New("foo", "bar")
	//
	// m.Set(p, 0)
	//
	// a := m.Delete(p)
	// b := m.Delete(p)
	//
	// Result: a == true and b == false
	Delete(p Path) bool
}

// Wildcard is a special key representing any possible path.
var Wildcard = wildcard{}

type wildcard struct{}

func (w wildcard) Key() interface{} {
	return struct{}{}
}

func (w wildcard) String() string {
	return "*"
}

func (w wildcard) Equal(other interface{}) bool {
	_, ok := other.(wildcard)
	return ok
}

type node struct {
	val      interface{}
	wildcard *node
	children map[key.Key]*node
}

// NewMap creates a new Map
func NewMap() Map {
	return &node{}
}

// Visit calls f for every matching registration in the Map
func (n *node) Visit(p Path, f pathmap.VisitorFunc) error {
	for i, element := range p {
		if n.wildcard != nil {
			if err := n.wildcard.Visit(p[i+1:], f); err != nil {
				return err
			}
		}
		next, ok := n.children[element]
		if !ok {
			return nil
		}
		n = next
	}
	if n.val == nil {
		return nil
	}
	return f(n.val)
}

// VisitPrefix calls f for every registered path that is a prefix of
// the path
func (n *node) VisitPrefix(p Path, f pathmap.VisitorFunc) error {
	for i, element := range p {
		if n.val != nil {
			if err := f(n.val); err != nil {
				return err
			}
		}
		if n.wildcard != nil {
			if err := n.wildcard.VisitPrefix(p[i+1:], f); err != nil {
				return err
			}
		}
		next, ok := n.children[element]
		if !ok {
			return nil
		}
		n = next
	}
	if n.val == nil {
		return nil
	}
	return f(n.val)
}

// Get returns the mapping for path
func (n *node) Get(p Path) interface{} {
	for _, element := range p {
		if element.Equal(Wildcard) {
			if n.wildcard == nil {
				return nil
			}
			n = n.wildcard
			continue
		}
		next, ok := n.children[element]
		if !ok {
			return nil
		}
		n = next
	}
	return n.val
}

// Set a mapping of path to value. Path may contain wildcards. Set
// replaces what was there before.
func (n *node) Set(p Path, v interface{}) {
	for _, element := range p {
		if element.Equal(Wildcard) {
			if n.wildcard == nil {
				n.wildcard = &node{}
			}
			n = n.wildcard
			continue
		}
		if n.children == nil {
			n.children = map[key.Key]*node{}
		}
		next, ok := n.children[element]
		if !ok {
			next = &node{}
			n.children[element] = next
		}
		n = next
	}
	n.val = v
}

// Delete removes the mapping for path
func (n *node) Delete(p Path) bool {
	nodes := make([]*node, len(p)+1)
	for i, element := range p {
		nodes[i] = n
		if element.Equal(Wildcard) {
			if n.wildcard == nil {
				return false
			}
			n = n.wildcard
			continue
		}
		next, ok := n.children[element]
		if !ok {
			return false
		}
		n = next
	}
	n.val = nil
	nodes[len(p)] = n

	// See if we can delete any node objects.
	for i := len(p); i > 0; i-- {
		n = nodes[i]
		if n.val != nil || n.wildcard != nil || len(n.children) > 0 {
			break
		}
		parent := nodes[i-1]
		element := p[i-1]
		if element.Equal(Wildcard) {
			parent.wildcard = nil
		} else {
			delete(parent.children, element)
		}
	}
	return true
}

func (n *node) String() string {
	var b bytes.Buffer
	n.write(&b, "")
	return b.String()
}

func (n *node) write(b *bytes.Buffer, indent string) {
	if n.val != nil {
		b.WriteString(indent)
		fmt.Fprintf(b, "Val: %v", n.val)
		b.WriteString("\n")
	}
	if n.wildcard != nil {
		b.WriteString(indent)
		fmt.Fprintf(b, "Child %q:\n", Wildcard)
		n.wildcard.write(b, indent+"  ")
	}
	children := make([]key.Key, 0, len(n.children))
	for key := range n.children {
		children = append(children, key)
	}
	sort.Slice(children, func(i, j int) bool {
		return children[i].String() < children[j].String()
	})

	for _, key := range children {
		child := n.children[key]
		b.WriteString(indent)
		fmt.Fprintf(b, "Child %q:\n", key.String())
		child.write(b, indent+"  ")
	}
}
