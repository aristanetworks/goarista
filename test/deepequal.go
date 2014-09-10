// Copyright (c) 2014 Arista Networks, Inc.  All rights reserved.
// Arista Networks, Inc. Confidential and Proprietary.

package test

// comparable types have an equality-testing method.
type comparable interface {
	// Equal returns true if this object is equal to the other one.
	Equal(other interface{}) bool
}

// DeepEqual is a simpler implementation of reflect.DeepEqual that:
//   - Doesn't handle all the types (only the basic types found in our
//     system are supported).
//   - Doesn't handle cycles in references.
//   - Gives data types the ability to define their own comparison method by
//     implementing the comparable interface.
//   - Supports keys in maps that are pointers.
func DeepEqual(a, b interface{}) bool {
	if a == nil || b == nil {
		return a == b
	}
	switch a := a.(type) {
	case map[string]interface{}:
		v, ok := b.(map[string]interface{})
		if !ok {
			return false
		}
		for key, value := range a {
			if other, ok := v[key]; !ok || !DeepEqual(value, other) {
				return false
			}
		}
		// Check for keys in b not in a.
		for key := range v {
			if _, ok := a[key]; !ok {
				return false
			}
		}
		return true
	case map[uint32]interface{}:
		v, ok := b.(map[uint32]interface{})
		if !ok {
			return false
		}
		for key, value := range a {
			if other, ok := v[key]; !ok || !DeepEqual(value, other) {
				return false
			}
		}
		// Check for keys in b not in a.
		for key := range v {
			if _, ok := a[key]; !ok {
				return false
			}
		}
		return true
	case map[uint64]interface{}:
		v, ok := b.(map[uint64]interface{})
		if !ok {
			return false
		}
		for key, value := range a {
			if other, ok := v[key]; !ok || !DeepEqual(value, other) {
				return false
			}
		}
		// Check for keys in b not in a.
		for key := range v {
			if _, ok := a[key]; !ok {
				return false
			}
		}
		return true
	case map[interface{}]interface{}:
		v, ok := b.(map[interface{}]interface{})
		if !ok {
			return false
		}
		// We compare in both directions to catch keys that are in b but not
		// in a.  It sucks to have to do another O(N^2) for this, but oh well.
		return mapEqual(a, v) && mapEqual(v, a)
	case *map[string]interface{}:
		v, ok := b.(*map[string]interface{})
		if !ok || a == nil || v == nil {
			return ok && a == v
		}
		return DeepEqual(*a, *v)
	case comparable:
		return a.Equal(b)
	case []string:
		v, ok := b.([]string)
		if !ok || len(a) != len(v) {
			return false
		}
		for i, s := range a {
			if s != v[i] {
				return false
			}
		}
		return true
	case []uint32:
		v, ok := b.([]uint32)
		if !ok || len(a) != len(v) {
			return false
		}
		for i, s := range a {
			if s != v[i] {
				return false
			}
		}
		return true
	case []uint64:
		v, ok := b.([]uint64)
		if !ok || len(a) != len(v) {
			return false
		}
		for i, s := range a {
			if s != v[i] {
				return false
			}
		}
		return true
	case []interface{}:
		v, ok := b.([]interface{})
		if !ok || len(a) != len(v) {
			return false
		}
		for i, s := range a {
			if !DeepEqual(s, v[i]) {
				return false
			}
		}
		return true
	case *[]string:
		v, ok := b.(*[]string)
		if !ok || a == nil || v == nil {
			return ok && a == v
		}
		return DeepEqual(*a, *v)
	case *[]interface{}:
		v, ok := b.(*[]interface{})
		if !ok || a == nil || v == nil {
			return ok && a == v
		}
		return DeepEqual(*a, *v)
	default:
		// All the basic types and structs that do not implement comparable.
		return a == b
	}
}

// mapEqual does O(N^2) comparisons to check that all the keys present in the
// first map are also present in the second map and have identical values.
// Note that this function only compares a to b, but not b to a.
func mapEqual(a, b map[interface{}]interface{}) bool {
	for akey, avalue := range a {
		found := false
		for bkey, bvalue := range b {
			if DeepEqual(akey, bkey) && DeepEqual(avalue, bvalue) {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}
