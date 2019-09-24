// Copyright (c) 2019 Arista Networks, Inc.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the COPYING file.

package key

import (
	"reflect"
)

// An entry represents an entry in a map whose key is not normally hashable,
// and is therefore of type Hashable
// (that is, a Hash method has been defined for this entry's key, and we can index it)
type entry struct {
	k    Hashable
	v    interface{}
	next *entry
}

// Map represents a map of arbitrary keys and values.
type Map struct {
	normal map[interface{}]interface{}
	custom map[uint64]entry
	length int // length of the Map
}

// String will output the string representation of the map
func (m *Map) String() string {
	// TODO
	return ""
}

// Equal will eventually do a better job of comparing two Maps
func (m *Map) Equal(other interface{}) bool {
	o, ok := other.(Map)
	if !ok {
		return false
	}
	if m.length != o.length {
		return false
	}
	// TODO: revise
	if !reflect.DeepEqual(m.normal, o.normal) {
		return false
	}
	if !reflect.DeepEqual(m.custom, o.custom) {
		return false
	}
	return true
}

// Hashable represents the key for an entry in a Map that cannot natively be hashed
type Hashable interface {
	Hash() uint64
	Equal(other interface{}) bool
}

// Set allows the indexing of entries with arbitrary key types, so long as the keys are
// either hashable natively or are of type Hashable
func (m *Map) Set(k, v interface{}) {
	if k == nil {
		return
	}
	if hkey, ok := k.(Hashable); ok {
		if m.custom == nil {
			m.custom = make(map[uint64]entry)
		}
		// get hash, add to custom if not present
		// if present, append to next of root entry
		h := hkey.Hash()
		rootentry, ok := m.custom[h]
		if !ok {
			rootentry = entry{k: hkey, v: v}
		} else {
			var prev *entry
			curr := &rootentry
			for curr != nil {
				if curr.k.Equal(hkey) {
					curr.v = v
					m.custom[h] = rootentry
					return
				}
				prev = curr
				curr = prev.next
			}
			prev.next = &entry{k: hkey, v: v}
		}
		// write the stack back
		m.custom[h] = rootentry
	} else {
		if m.normal == nil {
			m.normal = make(map[interface{}]interface{})
		}
		l := len(m.normal)
		m.normal[k] = v
		if l == len(m.normal) { // len hasn't changed
			return
		}
	}
	m.length++
}

// Get retrieves the value stored with key k from the Map
func (m *Map) Get(k interface{}) (interface{}, bool) {
	if hkey, ok := k.(Hashable); ok {
		h := hkey.Hash()
		hentry, ok := m.custom[h]
		if !ok {
			return nil, false
		}
		curr := &hentry
		for curr != nil {
			if curr.k.Equal(hkey) {
				return curr.v, true
			}
			curr = curr.next
		}
		return nil, false
	}
	v, ok := m.normal[k]
	return v, ok
}
