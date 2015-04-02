// Copyright (c) 2015 Arista Networks, Inc.  All rights reserved.
// Arista Networks, Inc. Confidential and Proprietary.

package test

import (
	"fmt"
	"reflect"
)

// diffable types have a method that returns the diff
// of two objects
type diffable interface {
	// Diff returns a human readable string of the diff of the two objects
	// an empty string means that the two objects are equal
	Diff(other interface{}) string
}

// Diff returns the difference of two objects in a human readable format
// Empty string is returned when there is no difference
func Diff(a, b interface{}) string {
	if DeepEqual(a, b) {
		return ""
	}

	return diffImpl(a, b)
}

func diffImpl(a, b interface{}) string {

	av := reflect.ValueOf(a)
	bv := reflect.ValueOf(b)
	// Check if nil
	if av.Kind() == reflect.Invalid {
		if bv.Kind() == reflect.Invalid {
			return "" // Both are "nil" with no type
		}
		return fmt.Sprintf(
			"one value is nil and the other is of type: %T",
			b)
	} else if bv.Kind() == reflect.Invalid {
		return fmt.Sprintf("one value is nil and the other is of type: %T",
			a)
	}
	if av.Type() != bv.Type() {
		return fmt.Sprintf("types are different: %T vs %T", a, b)
	}

	if ac, ok := a.(diffable); ok {
		return ac.Diff(b.(diffable))
	}

	if ac, ok := a.(comparable); ok {
		r := ac.Equal(b.(comparable))
		if r {
			return ""
		}
		return fmt.Sprintf("Comparable types are different: %s vs %s",
			PrettyPrint(a), PrettyPrint(b))
	}

	switch av.Kind() {
	case reflect.Bool:
		if av.Bool() != bv.Bool() {
			return fmt.Sprintf("Booleans different: %v, %v", a, b)
		}

	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		if av.Int() != bv.Int() {
			return fmt.Sprintf("Ints different: %v, %v", a, b)
		}

	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		if av.Uint() != bv.Uint() {
			return fmt.Sprintf("Uints different: %v, %v", a, b)
		}

	case reflect.Float32, reflect.Float64:
		if av.Float() != bv.Float() {
			return fmt.Sprintf("Floats different: %v, %v", a, b)
		}

	case reflect.Complex64, reflect.Complex128:
		if av.Complex() != bv.Complex() {
			return fmt.Sprintf("Complexes different: %v, %v", a, b)
		}

	case reflect.Array, reflect.Slice:
		l := av.Len()
		if l != bv.Len() {
			return fmt.Sprintf("Arrays have different size: %d != %d",
				av.Len(), bv.Len())
		}
		for i := 0; i < l; i++ {
			diff := diffImpl(av.Index(i).Interface(), bv.Index(i).Interface())
			if len(diff) > 0 {
				diff = fmt.Sprintf(
					"In arrays, values are different at index %d: %s",
					i, diff)
				return diff
			}
		}

	case reflect.Map:
		if c, d := isNilCheck(av, bv); c {
			return d
		}
		if av.Len() != bv.Len() {
			return fmt.Sprintf("Maps have different size: %d != %d",
				av.Len(), bv.Len())
		}
		for _, ka := range av.MapKeys() {
			ae := av.MapIndex(ka)
			var be reflect.Value
			// Find the corresponding entry in b
			ok := false
			for _, kb := range bv.MapKeys() {
				if ka.Kind() == reflect.Ptr {
					if diff := diffImpl(ka.Elem(), kb.Elem()); len(diff) == 0 {
						be = bv.MapIndex(kb)
						ok = true
						break
					}
				} else if diff := diffImpl(
					ka.Interface(), kb.Interface()); len(diff) == 0 {
					be = bv.MapIndex(kb)
					ok = true
					break
				}
			}
			if !ok {
				return fmt.Sprintf(
					"key %s in map is missing in the second map",
					prettyPrint(ka, ptrSet{}, prettyPrintDepth))
			}
			if !be.IsValid() {
				return fmt.Sprintf(
					"for key %s in map, values are different: %s vs %s "+
						"(the \"nil\" entry might be a missing entry)",
					prettyPrint(ka, ptrSet{}, prettyPrintDepth),
					prettyPrint(ae, ptrSet{}, prettyPrintDepth),
					prettyPrint(be, ptrSet{}, prettyPrintDepth))
			}
			if !ae.CanInterface() {
				return fmt.Sprintf(
					"for key %s in map, value can't become an interface: %s",
					prettyPrint(ka, ptrSet{}, prettyPrintDepth),
					prettyPrint(ae, ptrSet{}, prettyPrintDepth))
			}
			if !be.CanInterface() {
				return fmt.Sprintf(
					"for key %s in map, value can't become an interface: %s",
					prettyPrint(ka, ptrSet{}, prettyPrintDepth),
					prettyPrint(be, ptrSet{}, prettyPrintDepth))
			}
			if diff := diffImpl(ae.Interface(), be.Interface()); len(diff) > 0 {
				return fmt.Sprintf(
					"for key %s in map, values are different: %s",
					prettyPrint(ka, ptrSet{}, prettyPrintDepth), diff)
			}
		}

	case reflect.Ptr, reflect.Interface:
		if c, d := isNilCheck(av, bv); c {
			return d
		}
		return diffImpl(av.Elem().Interface(), bv.Elem().Interface())

	case reflect.String:
		if av.String() != bv.String() {
			return fmt.Sprintf("Strings different: %q vs %q", a, b)
		}

	case reflect.Struct:
		if a == b {
			return ""
		}
		return fmt.Sprintf("Structs types are different: %s vs %s",
			PrettyPrint(a), PrettyPrint(b))

	default:
		return fmt.Sprintf("Unknown or unsupported type: %T", a)
	}

	return ""
}

func isNilCheck(a, b reflect.Value) (bool /*checked*/, string) {
	if a.IsNil() {
		if b.IsNil() {
			return true, ""
		}
		return true, fmt.Sprintf("one value is nil and the other is not nil: %s",
			prettyPrint(b, ptrSet{}, prettyPrintDepth))
	} else if b.IsNil() {
		return true, fmt.Sprintf("one value is nil and the other is not nil: %s",
			prettyPrint(a, ptrSet{}, prettyPrintDepth))
	}
	return false, ""
}

type mapEntry struct {
	k, v string
}

type mapEntries struct {
	entries []*mapEntry
}

func (t *mapEntries) Len() int {
	return len(t.entries)
}
func (t *mapEntries) Less(i, j int) bool {
	if t.entries[i].k > t.entries[j].k {
		return false
	} else if t.entries[i].k < t.entries[j].k {
		return true
	}
	return t.entries[i].v <= t.entries[j].v
}
func (t *mapEntries) Swap(i, j int) {
	t.entries[i], t.entries[j] = t.entries[j], t.entries[i]
}
