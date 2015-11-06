// Copyright (C) 2015  Arista Networks, Inc.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the COPYING file.

package key

import (
	"encoding/json"
	"fmt"
)

// Key represents the Key in the updates and deletes of the Notification
// objects.  The only reason this exists is that Go won't let us define
// our own hash function for non-hashable types, and unfortunately we
// need to be able to index maps by map[string]interface{} objects.
type Key interface {
	Key() interface{}
	String() string
	Equal(other interface{}) bool

	// IsHashable is true if this key is hashable and can be accessed in O(1) in a Go map.
	// If false, then a O(N) lookup is required to find this key in a Go map.
	// The only kind of unhashable key currently supported is map[string]interface{}.
	IsHashable() bool

	// Helper methods to manipulate maps keyed by `Key'.

	// GetFromMap returns the value for the entry of this Key.
	GetFromMap(map[Key]interface{}) (interface{}, bool)
	// DeleteFromMap deletes the entry in the map for this Key.
	// This is a no-op if the key does not exist in the map.
	DeleteFromMap(map[Key]interface{})
	// SetToMap updates or inserts an entry in the map for this Key.
	SetToMap(m map[Key]interface{}, value interface{})
}

// Keyable is an interface that should be applied to hashable values
// that can be wrapped as Key's.
type Keyable interface {
	// KeyString returns the string representation of this key.
	KeyString() string
}

type keyImpl struct {
	key interface{}
}

// New wraps the given value in a Key.
// This function panics if the value passed in isn't allowed in a Key or
// doesn't implement Keyable.
func New(intf interface{}) Key {
	switch t := intf.(type) {
	case map[string]interface{}:
		intf = &t
	case int8, int16, int32, int64,
		uint8, uint16, uint32, uint64,
		float32, float64, string, bool,
		Keyable:
	default:
		panic(fmt.Sprintf("Invalid type for key: %T", intf))
	}
	return keyImpl{key: intf}
}

func (k keyImpl) Key() interface{} {
	if m, ok := k.key.(*map[string]interface{}); ok {
		return *m
	}
	return k.key
}

func (k keyImpl) String() string {
	return stringify(k.key)
}

func isHashableMap(m map[Key]interface{}) bool {
	for k := range m {
		return k.IsHashable()
	}
	return true
}

func (k keyImpl) IsHashable() bool {
	_, ok := k.key.(*map[string]interface{})
	return !ok
}

func (k keyImpl) GetFromMap(m map[Key]interface{}) (interface{}, bool) {
	if len(m) == 0 {
		return nil, false
	}
	if isHashableMap(m) {
		v, ok := m[k]
		return v, ok
	}
	for key, value := range m {
		if k.Equal(key) {
			return value, true
		}
	}
	return nil, false
}
func (k keyImpl) DeleteFromMap(m map[Key]interface{}) {
	if len(m) == 0 {
		return
	}
	if isHashableMap(m) {
		delete(m, k)
		return
	}
	for key := range m {
		if k.Equal(key) {
			delete(m, key)
			return
		}
	}
}

func (k keyImpl) SetToMap(m map[Key]interface{}, value interface{}) {
	if isHashableMap(m) {
		m[k] = value
		return
	}
	for key := range m {
		if k.Equal(key) {
			m[key] = value
			return
		}
	}
	m[k] = value
}

func (k keyImpl) GoString() string {
	return fmt.Sprintf("key.New(%#v)", k.Key())
}

func (k keyImpl) MarshalJSON() ([]byte, error) {
	return json.Marshal(k.Key())
}

func (k keyImpl) Equal(other interface{}) bool {
	o, ok := other.(Key)
	if !ok {
		return false
	}
	if m, ok := k.key.(*map[string]interface{}); ok {
		m2, ok := o.Key().(map[string]interface{})
		return ok && keyEqual(*m, m2)
	}
	return keyEqual(k.key, o.Key())
}

// Comparable types have an equality-testing method.
type Comparable interface {
	// Equal returns true if this object is equal to the other one.
	Equal(other interface{}) bool
}

func keyEqual(a, b interface{}) bool {
	switch a := a.(type) {
	case map[string]interface{}:
		b, ok := b.(map[string]interface{})
		if !ok || len(a) != len(b) {
			return false
		}
		for k, av := range a {
			if bv, ok := b[k]; !ok || !keyEqual(av, bv) {
				return false
			}
		}
		return true
	case map[Key]interface{}:
		b, ok := b.(map[Key]interface{})
		if !ok || len(a) != len(b) {
			return false
		}
		for k, av := range a {
			if bv, ok := k.GetFromMap(b); !ok || !keyEqual(av, bv) {
				return false
			}
		}
		return true
	case Comparable:
		return a.Equal(b)
	}

	return a == b
}
