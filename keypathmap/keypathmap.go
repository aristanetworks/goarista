// Copyright (c) 2017 Arista Networks, Inc.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the COPYING file.

package keypathmap

import (
	"bytes"
	"fmt"
	"sort"

	"github.com/aristanetworks/goarista/key"
	"github.com/aristanetworks/goarista/pathmap"
)

// KeyPathMap associates Paths to a values. It allows wildcards. The
// primary use of PathMap is to be able to register handlers to paths
// that can be efficiently looked up every time a path is updated.
//
// For example:
//
// m.Set({key.New("interfaces"), key.New("*"), key.New("adminStatus")}, AdminStatusHandler)
// m.Set({key.New("interface"), key.New("Management1"), key.New("adminStatus")},
//     Management1AdminStatusHandler)
//
// m.Visit({key.New("interfaces"), key.New("Ethernet3/32/1"), key.New("adminStatus")},
//     HandlerExecutor)
// >> AdminStatusHandler gets passed to HandlerExecutor
// m.Visit({key.New("interfaces"), key.New("Management1"), key.New("adminStatus")},
//    HandlerExecutor)
// >> AdminStatusHandler and Management1AdminStatusHandler gets passed to HandlerExecutor
//
// Note, Visit performance is typically linearly with the length of
// the path. But, it can be as bad as O(2^len(Path)) when TreeMap
// nodes have children and a wildcard associated with it. For example,
// if these paths were registered:
//
// m.Set([]string{key.New("foo"), key.New("bar"), key.New("baz")}, 1)
// m.Set([]string{key.New("*"), key.New("bar"), key.New("baz")}, 2)
// m.Set([]string{key.New("*"), key.New("*"), key.New("baz")}, 3)
// m.Set([]string{key.New("*"), key.New("*"), key.New("*")}, 4)
// m.Set([]string{key.New("foo"), key.New("*"), key.New("*")}, 5)
// m.Set([]string{key.New("foo"), key.New("bar"), key.New("*")}, 6)
// m.Set([]string{key.New("foo"), key.New("*"), key.New("baz")}, 7)
// m.Set([]string{key.New("*"), key.New("bar"), key.New("*")}, 8)
//
// m.Visit([]{key.New("foo"),key.New("bar"),key.New("baz")}, Foo) // 2^3 nodes traversed
//
// This shouldn't be a concern with our paths because it is likely
// that a TreeMap node will either have a wildcard or children, not
// both. A TreeMap node that corresponds to a collection will often be a
// wildcard, otherwise it will have specific children.
type KeyPathMap interface {
	// Visit calls f for every registration in the KeyPathMap that
	// matches path. For example,
	//
	// m.Set({key.New("foo"), key.New("bar")}, 1)
	// m.Set({key.New("*"), key.New("bar")}, 2)
	//
	// m.Visit({key.New("foo"), key.New("bar")}, Printer)
	// >> Calls Printer(1) and Printer(2)
	Visit(path []key.Key, f pathmap.VisitorFunc) error

	// VisitPrefix calls f for every registration in the KeyPathMap that
	// is a prefix of path. For example,
	//
	// m.Set({}, 0)
	// m.Set({key.New("foo")}, 1)
	// m.Set({key.New("foo"), key.New("bar")}, 2)
	// m.Set({key.New("foo"), key.New("quux")}, 3)
	// m.Set({key.New("*"), key.New("bar")}, 4)
	//
	// m.VisitPrefix({key.New("foo"), key.New("bar"), key.New("baz")}, Printer)
	// >> Calls Printer on values 0, 1, 2, and 4
	VisitPrefix(path []key.Key, f pathmap.VisitorFunc) error

	// Get returns the mapping for path. This returns the exact
	// mapping for path. For example, if you register two paths
	//
	// m.Set({key.New("foo"), key.New("bar")}, 1)
	// m.Set({key.New("*"), key.New("bar")}, 2)
	//
	// m.Get({key.New("foo"), key.New("bar")}) => 1
	// m.Get({key.New("*"), key.New("bar")}) => 2
	Get(path []key.Key) interface{}

	// Set a mapping of path to value. Path may contain wildcards. Set
	// replaces what was there before.
	Set(path []key.Key, v interface{})

	// Delete removes the mapping for path
	Delete(path []key.Key) bool
}

// Wildcard is a special key representing any possible path
var Wildcard key.Key = key.New("*")

type node struct {
	val      interface{}
	wildcard *node
	children map[key.Key]*node
}

// New creates a new KeyPathMap
func New() KeyPathMap {
	return &node{}
}

// Visit calls f for every matching registration in the KeyPathMap
func (n *node) Visit(path []key.Key, f pathmap.VisitorFunc) error {
	for i, element := range path {
		if n.wildcard != nil {
			if err := n.wildcard.Visit(path[i+1:], f); err != nil {
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
func (n *node) VisitPrefix(path []key.Key, f pathmap.VisitorFunc) error {
	for i, element := range path {
		// Call f on each node we visit
		if n.val != nil {
			if err := f(n.val); err != nil {
				return err
			}
		}
		if n.wildcard != nil {
			if err := n.wildcard.VisitPrefix(path[i+1:], f); err != nil {
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
	// Call f on the final node
	return f(n.val)
}

// Get returns the mapping for path
func (n *node) Get(path []key.Key) interface{} {
	for _, element := range path {
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
func (n *node) Set(path []key.Key, v interface{}) {
	for _, element := range path {
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
func (n *node) Delete(path []key.Key) bool {
	nodes := make([]*node, len(path)+1)
	for i, element := range path {
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
	nodes[len(path)] = n

	// See if we can delete any node objects
	for i := len(path); i > 0; i-- {
		n = nodes[i]
		if n.val != nil || n.wildcard != nil || len(n.children) > 0 {
			break
		}
		parent := nodes[i-1]
		element := path[i-1]
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
