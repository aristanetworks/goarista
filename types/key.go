// Copyright (c) 2015 Arista Networks, Inc.  All rights reserved.
// Arista Networks, Inc. Confidential and Proprietary.

package types

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

	// Helper methods to manipulate maps keyed by `Key'.

	// GetFromMap returns the value for the entry of this Key.
	GetFromMap(map[Key]interface{}) (interface{}, bool)
	// DeleteFromMap deletes the entry in the map for this Key.
	// This is a no-op if the key does not exist in the map.
	DeleteFromMap(map[Key]interface{})
	// SetToMap updates or inserts an entry in the map for this Key.
	SetToMap(m map[Key]interface{}, value interface{})
}

type keyImpl struct {
	key interface{}
}

// NewKey returns a Key object implementing the Key interface
// to wrapped the key passed in
func NewKey(intf interface{}) Key {
	switch t := intf.(type) {
	case map[string]interface{}:
		intf = &t
	case int8, int16, int32, int64,
		uint8, uint16, uint32, uint64,
		float32, float64, string, bool:
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
	str, err := StringifyInterface(k.key)
	if err != nil {
		panic("Unable to stringify Key: " + err.Error())
	}
	return str
}

func isHashableMap(m map[Key]interface{}) bool {
	for k := range m {
		return k.(keyImpl).isHashable()
	}
	return false
}
func (k keyImpl) isHashable() bool {
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
	return fmt.Sprintf("types.NewKey(%#v)", k.Key())
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

func keyEqual(a, b interface{}) bool {
	if a, ok := a.(map[string]interface{}); ok {
		b, ok := b.(map[string]interface{})
		if !ok || len(a) != len(b) {
			return false
		}
		for k, av := range a {
			if bv, ok := (b)[k]; !ok || !keyEqual(av, bv) {
				return false
			}
		}
		return true
	}

	return a == b
}
