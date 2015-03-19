// Copyright (c) 2015 Arista Networks, Inc.  All rights reserved.
// Arista Networks, Inc. Confidential and Proprietary.

package types

// Key represents the Key in the updates and deletes of the Notification objects
type Key interface {
	Key() interface{}
	String() string
	Equal(other Key) bool
}

type keyImpl struct {
	key interface{}
}

// NewKey returns a Key object implementing the Key interface
// to wrapped the key passed in
func NewKey(intf interface{}) Key {
	return keyImpl{key: intf}
}

func (k keyImpl) Key() interface{} {
	return k.key
}

func (k keyImpl) String() string {
	s, err := StringifyInterface(k.key)
	if err != nil {
		panic("Unable to stringify a key: " + err.Error())
	}
	return s
}

func (k keyImpl) Equal(other Key) bool {
	return keyEqual(k.key, other.Key())
}

func keyEqual(a, b interface{}) bool {
	if a, ok := a.(*map[string]interface{}); ok {
		b, ok := b.(*map[string]interface{})
		if !ok || len(*a) != len(*b) {
			return false
		}
		for k, av := range *a {
			if bv, ok := (*b)[k]; !ok || !keyEqual(av, bv) {
				return false
			}
		}
		return true
	}

	return a == b
}
