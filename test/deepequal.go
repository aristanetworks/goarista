// Copyright (c) 2014 Arista Networks, Inc.  All rights reserved.
// Arista Networks, Inc. Confidential and Proprietary.

package test

import (
	"bytes"
	"math"
	"reflect"
	"unsafe"
)

// comparable types have an equality-testing method.
type comparable interface {
	// Equal returns true if this object is equal to the other one.
	Equal(other interface{}) bool
}

var comparableType = reflect.TypeOf((*comparable)(nil)).Elem()

// DeepEqual is a faster implementation of reflect.DeepEqual that:
//   - Has a reflection-free fast-path for all the common types we use.
//   - Gives data types the ability to exclude some of their fields from the
//     consideration of DeepEqual by tagging them with `deepequal:"ignore"`.
//   - Gives data types the ability to define their own comparison method by
//     implementing the comparable interface.
//   - Supports "composite" (or "complex") keys in maps that are pointers.
func DeepEqual(a, b interface{}) bool {
	if a == nil || b == nil {
		return a == b
	}
	switch a := a.(type) {
	// Short circuit fast-path for common built-in types.
	case uint8, uint16, uint32, uint64,
		string, bool,
		int8, int16, int32, int64:
		return a == b

	case map[string]interface{}:
		v, ok := b.(map[string]interface{})
		if !ok || len(a) != len(v) {
			return false
		}
		for key, value := range a {
			if other, ok := v[key]; !ok || !DeepEqual(value, other) {
				return false
			}
		}
		return true
	case map[uint32]interface{}:
		v, ok := b.(map[uint32]interface{})
		if !ok || len(a) != len(v) {
			return false
		}
		for key, value := range a {
			if other, ok := v[key]; !ok || !DeepEqual(value, other) {
				return false
			}
		}
		return true
	case map[uint64]interface{}:
		v, ok := b.(map[uint64]interface{})
		if !ok || len(a) != len(v) {
			return false
		}
		for key, value := range a {
			if other, ok := v[key]; !ok || !DeepEqual(value, other) {
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
	case *map[interface{}]interface{}:
		v, ok := b.(*map[interface{}]interface{})
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
	case []byte:
		v, ok := b.([]byte)
		return ok && bytes.Equal(a, v)
	case []uint16:
		v, ok := b.([]uint16)
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
	case float32:
		v, ok := b.(float32)
		return ok && (a == b || (math.IsNaN(float64(a)) && math.IsNaN(float64(v))))
	case float64:
		v, ok := b.(float64)
		return ok && (a == b || (math.IsNaN(a) && math.IsNaN(v)))

	default:
		// Handle other kinds of non-comparable objects.
		return genericDeepEqual(a, b, make(map[edge]struct{}))
	}
}

type edge struct {
	from uintptr
	to   uintptr
}

func genericDeepEqual(a, b interface{}, seen map[edge]struct{}) bool {
	av := reflect.ValueOf(a)
	bv := reflect.ValueOf(b)
	if avalid, bvalid := av.IsValid(), bv.IsValid(); !avalid || !bvalid {
		return avalid == bvalid
	}
	if bv.Type() != av.Type() {
		return false
	}

	switch av.Kind() {
	case reflect.Interface:
		if av.Type().Implements(comparableType) {
			return av.Interface().(comparable).Equal(bv.Interface())
		}
		fallthrough
	case reflect.Ptr:
		if av.IsNil() || bv.IsNil() {
			return a == b
		}

		av = av.Elem()
		bv = bv.Elem()
		if av.CanAddr() && bv.CanAddr() {
			e := edge{from: av.UnsafeAddr(), to: bv.UnsafeAddr()}
			// Detect and prevent cycles.
			if _, ok := seen[e]; ok {
				return true
			}
			seen[e] = struct{}{}
		}

		return genericDeepEqual(av.Interface(), bv.Interface(), seen)
	case reflect.Slice, reflect.Array:
		l := av.Len()
		if l != bv.Len() {
			return false
		}
		for i := 0; i < l; i++ {
			if !genericDeepEqual(av.Index(i).Interface(),
				bv.Index(i).Interface(), seen) {
				return false
			}
		}
		return true
	case reflect.Map:
		if av.IsNil() != bv.IsNil() {
			return false
		}
		if av.Len() != bv.Len() {
			return false
		}
		if av.Pointer() == bv.Pointer() {
			return true
		}
		for _, k := range av.MapKeys() {
			// Upon finding the first key that's a pointer, we bail out and do
			// a O(N^2) comparison.
			if kk := k.Kind(); kk == reflect.Ptr || kk == reflect.Interface {
				ok, _, _ := complexKeyMapEqual(av, bv, seen)
				return ok
			}
			ea := av.MapIndex(k)
			eb := bv.MapIndex(k)
			if !eb.IsValid() {
				return false
			}
			if !genericDeepEqual(ea.Interface(), eb.Interface(), seen) {
				return false
			}
		}
		return true
	case reflect.Struct:
		typ := av.Type()
		if typ.Implements(comparableType) {
			return av.Interface().(comparable).Equal(bv.Interface())
		}
		for i, n := 0, av.NumField(); i < n; i++ {
			if typ.Field(i).Tag.Get("deepequal") == "ignore" {
				continue
			}
			af := forceExport(av.Field(i))
			bf := forceExport(bv.Field(i))
			if !genericDeepEqual(af.Interface(), bf.Interface(), seen) {
				return false
			}
		}
		return true
	default:
		// Other the basic types.
		return a == b
	}
}

// The `reflect' package intentionally makes it impossible to access the value
// of an unexported attribute.  The implementation of reflect.DeepEqual() cheats
// as it bypasses this check.  Unfortunately, we can't use the same cheat, which
// prevents us from re-implementing DeepEqual properly.  So this is our cheat on
// top of theirs.  It makes the given reflect.Value appear as if it was exported.
func forceExport(v reflect.Value) reflect.Value {
	const flagRO uintptr = 1 << 5 // from reflect/value.go
	ptr := unsafe.Pointer(&v)
	rv := (*struct {
		typ  unsafe.Pointer // a *reflect.rtype (reflect.Type)
		ptr  unsafe.Pointer // The value wrapped by this reflect.Value
		flag uintptr
	})(ptr)
	rv.flag &= ^flagRO // Unset the flag so this value appears to be exported.
	return v
}

// Compares two maps with complex keys (that are pointers).  This assumes the
// maps have already been checked to have the same sizes.  The cost of this
// function is O(N^2) in the size of the input maps.
//
// The return is to be interpreted this way:
//    true, _, _            =>   av == bv
//    false, key, invalid   =>   the given key wasn't found in bv
//    false, key, value     =>   the given key had the given value in bv,
//                               which is different in av
func complexKeyMapEqual(av, bv reflect.Value,
	seen map[edge]struct{}) (bool, reflect.Value, reflect.Value) {
	for _, ka := range av.MapKeys() {
		var eb reflect.Value // The entry in bv with a key equal to ka
		for _, kb := range bv.MapKeys() {
			if genericDeepEqual(ka.Elem().Interface(), kb.Elem().Interface(), seen) {
				// Found the corresponding entry in bv.
				eb = bv.MapIndex(kb)
				break
			}
		}
		if !eb.IsValid() { // We didn't find a key equal to `ka' in 'bv'.
			return false, ka, reflect.Value{}
		}
		ea := av.MapIndex(ka)
		if !genericDeepEqual(ea.Interface(), eb.Interface(), seen) {
			return false, ka, eb
		}
	}
	return true, reflect.Value{}, reflect.Value{}
}

// mapEqual does O(N^2) comparisons to check that all the keys present in the
// first map are also present in the second map and have identical values.
func mapEqual(a, b map[interface{}]interface{}) bool {
	if len(a) != len(b) {
		return false
	}
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
