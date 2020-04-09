// Copyright (c) 2019 Arista Networks, Inc.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the COPYING file.

package key

import (
	"errors"
	"sort"
	"strings"
)

// Map allows the indexing of entries with arbitrary key types, so long as the keys are
// either hashable natively or implement Hashable
type Map struct {
	normal map[interface{}]interface{}
	custom map[uint64]entry
	length int // length of the Map
}

// NewMap creates a new Map from a list of key-value pairs, so long as the list is of even length.
func NewMap(keysAndVals ...interface{}) *Map {
	length := len(keysAndVals)
	if length%2 != 0 {
		panic("Odd number of arguments passed to NewMap. Arguments should be of form: " +
			"key1, value1, key2, value2, ...")
	}
	m := Map{}
	for i := 0; i < length; i += 2 {
		m.Set(keysAndVals[i], keysAndVals[i+1])
	}
	return &m
}

// String outputs the string representation of the map
func (m *Map) String() string {
	if m == nil {
		return "key.Map(nil)"
	}
	stringify := func(v interface{}) string {
		return stringifyCollectionHelper(v)
	}
	type kv struct {
		k string
		v string
	}
	var length int
	kvs := make([]kv, 0, m.Len())
	_ = m.Iter(func(k, v interface{}) error {
		element := kv{
			k: stringify(k),
			v: stringify(v),
		}
		kvs = append(kvs, element)
		length += len(element.k) + len(element.v)
		return nil
	})
	sort.Slice(kvs, func(i, j int) bool { return kvs[i].k < kvs[j].k })
	var buf strings.Builder
	buf.Grow(length + len("key.Map[]") + 2*len(kvs) /* room for seperators: ", :" */)
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
	if m == nil {
		return 0
	}
	return m.length
}

// An entry represents an entry in a map whose key is not normally hashable,
// and is therefore of type Hashable
// (that is, a Hash method has been defined for this entry's key, and we can index it)
//
// Because hash collisions are possible (though unlikely), an entry is
// actually a linked list. To save space, the valOrNext field serves
// double-duty. It is either just the value associated with the
// entry's key, indicating the end of the list, or it contains a
// *chainedEntry. A chainedEntry holds the key's value and the next
// entry (which may also contain a *chainedEntry).
type entry struct {
	k Hashable
	// contains a value or *chainedEntry
	valOrNext interface{}
}

type chainedEntry struct {
	val interface{}
	entry
}

// entrySearch searches entry for matching keys. It returns the
// containing entry if found, and the last entry in the chain if not
// found.
func entrySearch(ent *entry, k Hashable) (containing *entry, found bool) {
	for {
		if k.Equal(ent.k) {
			return ent, true
		}
		chEnt, ok := ent.valOrNext.(*chainedEntry)
		if !ok {
			return ent, false
		}
		ent = &chEnt.entry
	}
}

func entryGetValue(ent *entry) interface{} {
	if chEnt, ok := ent.valOrNext.(*chainedEntry); ok {
		return chEnt.val
	}
	return ent.valOrNext
}

func entrySetValue(ent *entry, v interface{}) {
	if chEnt, ok := ent.valOrNext.(*chainedEntry); ok {
		chEnt.val = v
		return
	}
	ent.valOrNext = v
}

// entryAppend appends a new entry to the end of ent.
func entryAppend(ent *entry, k Hashable, v interface{}) {
	if _, ok := ent.valOrNext.(*chainedEntry); ok {
		panic("chained entry passed to entryAppend ")
	}
	ent.valOrNext = &chainedEntry{
		val: ent.valOrNext,
		entry: entry{
			k:         k,
			valOrNext: v,
		},
	}
}

// entryRemove removes an entry that has key k. The new head entry is
// returned along with true if k is found and removed, and false if
// not found.
func entryRemove(head *entry, k Hashable) (*entry, bool) {
	if k.Equal(head.k) {
		// head of list matches
		if chEnt, ok := head.valOrNext.(*chainedEntry); ok {
			return &chEnt.entry, true
		}
		return nil, true
	}
	prev := head
	for {
		next, ok := prev.valOrNext.(*chainedEntry)
		if !ok {
			// reached end of chain, not found
			return head, false
		}
		if k.Equal(next.entry.k) {
			if nextNext, ok := next.entry.valOrNext.(*chainedEntry); ok {
				// Remove next from entry list, but next contains
				// prev's val, so move that into nextNext.
				nextNext.val = next.val
				prev.valOrNext = nextNext
			} else {
				// Remove end of the list
				prev.valOrNext = next.val
			}
			return head, true
		}
		prev = &next.entry
	}
}

func entryIter(ent entry, f func(k, v interface{}) error) error {
	for {
		if chEnt, ok := ent.valOrNext.(*chainedEntry); ok {
			if err := f(ent.k, chEnt.val); err != nil {
				return err
			}
			ent = chEnt.entry
			continue
		}
		return f(ent.k, ent.valOrNext)
	}
}

// Hashable represents the key for an entry in a Map that cannot natively be hashed
type Hashable interface {
	Hash() uint64
	Equal(other interface{}) bool
}

// Equal compares two Maps
func (m *Map) Equal(other interface{}) bool {
	if (m == nil) != (other == nil) {
		return false
	}
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
	if m == nil {
		return 0
	}
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
			m.custom[h] = entry{k: hkey, valOrNext: v}
			m.length++
			return
		}
		ent, found := entrySearch(&rootentry, hkey)
		if found {
			entrySetValue(ent, v)
			m.custom[h] = rootentry
			return
		}
		entryAppend(ent, hkey, v)
		m.custom[h] = rootentry
		m.length++
	} else {
		if m.normal == nil {
			m.normal = make(map[interface{}]interface{})
		}
		l := len(m.normal)
		m.normal[k] = v
		if l != len(m.normal) { // len has changed
			m.length++
		}
	}
}

// Get retrieves the value stored with key k from the Map
func (m *Map) Get(k interface{}) (interface{}, bool) {
	if m == nil {
		return nil, false
	}
	if hkey, ok := k.(Hashable); ok {
		h := hkey.Hash()
		hentry, ok := m.custom[h]
		if !ok {
			return nil, false
		}
		ent, found := entrySearch(&hentry, hkey)
		if !found {
			return nil, false
		}
		return entryGetValue(ent), true
	}
	v, ok := m.normal[k]
	return v, ok
}

// Del removes an entry with key k from the Map
func (m *Map) Del(k interface{}) {
	if m == nil {
		return
	}
	if hkey, ok := k.(Hashable); ok {
		if m.custom == nil {
			return
		}
		h := hkey.Hash()
		hentry, ok := m.custom[h]
		if !ok {
			return
		}
		newEnt, found := entryRemove(&hentry, hkey)
		if !found {
			return
		}
		m.length--
		if newEnt == nil {
			delete(m.custom, h)
		} else {
			m.custom[h] = *newEnt
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
	if m == nil {
		return nil
	}
	for k, v := range m.normal {
		if err := f(k, v); err != nil {
			return err
		}
	}
	for _, ent := range m.custom {
		if err := entryIter(ent, f); err != nil {
			return err
		}
	}
	return nil
}
