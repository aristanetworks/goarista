// Copyright (c) 2019 Arista Networks, Inc.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the COPYING file.

package key

import (
	"errors"
	"fmt"
	"sort"
	"strings"
)

// An entry represents an entry in a map whose key is not normally hashable,
// and is therefore of type Hashable
// (that is, a Hash method has been defined for this entry's key, and we can index it)
type entry struct {
	k    Hashable
	v    interface{}
	next *entry
}

// Map allows the indexing of entries with arbitrary key types, so long as the keys are
// either hashable natively or implement Hashable
type Map struct {
	normal map[interface{}]interface{}
	custom map[uint64]entry
	length int // length of the Map
}

// NewMap creates a new Map from a list of key-value pairs, so long as the list is of even length.
func NewMap(keysAndVals ...interface{}) *Map {
	len := len(keysAndVals)
	if len%2 != 0 {
		panic("Odd number of arguments passed to NewMap. Arguments should be of form: " +
			"key1, value1, key2, value2, ...")
	}
	m := Map{}
	for i := 0; i < len; i += 2 {
		m.Set(keysAndVals[i], keysAndVals[i+1])
	}
	return &m
}

// String outputs the string representation of the map
func (m *Map) String() string {
	stringify := func(v interface{}) string {
		if s, ok := v.(fmt.Stringer); ok {
			return s.String()
		}
		return fmt.Sprint(v)
	}
	type kv struct {
		k string
		v string
	}
	kvs := make([]kv, 0, m.Len())
	_ = m.Iter(func(k, v interface{}) error {
		kvs = append(kvs, kv{
			k: stringify(k),
			v: stringify(v),
		})
		return nil
	})
	sort.Slice(kvs, func(i, j int) bool { return kvs[i].k < kvs[j].k })
	var buf strings.Builder
	buf.WriteString("key.Map[")
	for i, kv := range kvs {
		if i != 0 {
			buf.WriteByte(' ')
		}
		buf.WriteString(kv.k + ":" + kv.v)
	}
	buf.WriteString("]")
	return buf.String()
}

// Len returns the length of the Map
func (m *Map) Len() int {
	return m.length
}

// Hashable represents the key for an entry in a Map that cannot natively be hashed
type Hashable interface {
	Hash() uint64
	Equal(other interface{}) bool
}

// Equal compares two Maps
func (m *Map) Equal(other interface{}) bool {
	o, ok := other.(*Map)
	if !ok {
		return false
	}
	if m.length != o.length {
		return false
	}
	err := m.Iter(func(k, v interface{}) error {
		otherV, ok := o.Get(k)
		if !ok {
			return errors.New("notequal")
		}
		if !keyEqual(v, otherV) {
			return errors.New("notequal")
		}
		return nil
	})
	return err == nil
}

// Hash returns the hash value of this Map
func (m *Map) Hash() uint64 {
	var h uintptr
	m.Iter(func(k, v interface{}) error {
		h += hashInterface(k) + hashInterface(v)
		return nil
	})
	return uint64(h)
}

// Set adds a key-value pair to the Map
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

// Del removes an entry with key k from the Map
func (m *Map) Del(k interface{}) {
	if hkey, ok := k.(Hashable); ok {
		if m.custom == nil {
			return
		}
		h := hkey.Hash()
		hentry, ok := m.custom[h]
		if !ok {
			return
		}
		var prev *entry
		curr := &hentry
		for curr != nil {
			if curr.k.Equal(hkey) { // del
				if prev == nil { // delete the head
					if curr.next == nil { // no more entries at this hash, remove it
						delete(m.custom, h)
					} else {
						m.custom[h] = *curr.next
					}
				} else {
					// delete a mid/tail entry node
					prev.next = curr.next
					m.custom[h] = hentry
				}
				m.length--
				return
			}
			prev = curr
			curr = curr.next
		}
		return
	}
	// not Hashable, check normal
	if m.normal == nil {
		return
	}
	l := len(m.normal)
	delete(m.normal, k)
	if l != len(m.normal) {
		m.length--
	}
}

// Iter applies func f to every key-value pair in the Map
func (m *Map) Iter(f func(k, v interface{}) error) error {
	for k, v := range m.normal {
		if err := f(k, v); err != nil {
			return err
		}
	}
	for _, e := range m.custom {
		curr := &e
		for curr != nil {
			if err := f(curr.k, curr.v); err != nil {
				return err
			}
			curr = curr.next
		}
	}
	return nil
}
