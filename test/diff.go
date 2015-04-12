// Copyright (c) 2015 Arista Networks, Inc.  All rights reserved.
// Arista Networks, Inc. Confidential and Proprietary.

package test

import (
	"fmt"
	"reflect"
	"sort"
	"strings"
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

	return diffImpl(a, b, nil)
}

func diffImpl(a, b interface{}, seen map[edge]struct{}) string {
	av := reflect.ValueOf(a)
	bv := reflect.ValueOf(b)
	// Check if nil
	if !av.IsValid() {
		if !bv.IsValid() {
			return "" // Both are "nil" with no type
		}
		return fmt.Sprintf("one value is nil and the other is of type: %T", b)
	} else if !bv.IsValid() {
		return fmt.Sprintf("one value is nil and the other is of type: %T", a)
	}
	if av.Type() != bv.Type() {
		return fmt.Sprintf("types are different: %T vs %T", a, b)
	}

	switch a := a.(type) {
	case bool,
		int8, int16, int32, int64,
		uint8, uint16, uint32, uint64,
		float32, float64:
		if a != b {
			typ := reflect.TypeOf(a).Name()
			return fmt.Sprintf("%s(%v) != %s(%v)", typ, a, typ, b)
		}
		return ""
	}

	if ac, ok := a.(diffable); ok {
		return ac.Diff(b.(diffable))
	}

	if ac, ok := a.(comparable); ok {
		if ac.Equal(b.(comparable)) {
			return ""
		}
		return fmt.Sprintf("Comparable types are different: %s vs %s",
			PrettyPrint(a), PrettyPrint(b))
	}

	switch av.Kind() {
	case reflect.Array, reflect.Slice:
		l := av.Len()
		if l != bv.Len() {
			return fmt.Sprintf("Arrays have different size: %d != %d",
				l, bv.Len())
		}
		for i := 0; i < l; i++ {
			diff := diffImpl(av.Index(i).Interface(), bv.Index(i).Interface(),
				seen)
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
			return fmt.Sprintf("Maps have different size: %d != %d (%s)",
				av.Len(), bv.Len(), diffMapKeys(av, bv))
		}
		for _, ka := range av.MapKeys() {
			ae := av.MapIndex(ka)
			if k := ka.Kind(); k == reflect.Ptr || k == reflect.Interface {
				return diffComplexKeyMap(av, bv, seen)
			}
			be := bv.MapIndex(ka)
			if !be.IsValid() {
				return fmt.Sprintf(
					"key %s in map is missing in the second map",
					prettyPrint(ka, ptrSet{}, prettyPrintDepth))
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
			if diff := diffImpl(ae.Interface(), be.Interface(), seen); len(diff) > 0 {
				return fmt.Sprintf(
					"for key %s in map, values are different: %s",
					prettyPrint(ka, ptrSet{}, prettyPrintDepth), diff)
			}
		}

	case reflect.Ptr, reflect.Interface:
		if c, d := isNilCheck(av, bv); c {
			return d
		}
		av = av.Elem()
		bv = bv.Elem()

		if av.CanAddr() && bv.CanAddr() {
			e := edge{from: av.UnsafeAddr(), to: bv.UnsafeAddr()}
			// Detect and prevent cycles.
			if seen == nil {
				seen = make(map[edge]struct{})
			} else if _, ok := seen[e]; ok {
				return ""
			}
			seen[e] = struct{}{}
		}
		return diffImpl(av.Interface(), bv.Interface(), seen)

	case reflect.String:
		if av.String() != bv.String() {
			return fmt.Sprintf("Strings different: %q vs %q", a, b)
		}

	case reflect.Struct:
		typ := av.Type()
		for i, n := 0, av.NumField(); i < n; i++ {
			if typ.Field(i).Tag.Get("deepequal") == "ignore" {
				continue
			}
			af := forceExport(av.Field(i))
			bf := forceExport(bv.Field(i))
			if diff := diffImpl(af.Interface(), bf.Interface(), seen); len(diff) > 0 {
				return fmt.Sprintf("attributes %q are different: %s",
					av.Type().Field(i).Name, diff)
			}
		}

	default:
		return fmt.Sprintf("Unknown or unsupported type: %T", a)
	}

	return ""
}

func diffComplexKeyMap(av, bv reflect.Value, seen map[edge]struct{}) string {
	ok, ka, be := complexKeyMapEqual(av, bv, seen)
	if ok {
		return ""
	} else if be.IsValid() {
		return fmt.Sprintf("for complex key %s in map, values are different: %s",
			prettyPrint(ka, ptrSet{}, prettyPrintDepth),
			diffImpl(av.MapIndex(ka).Interface(), be.Interface(), seen))
	}
	return fmt.Sprintf("complex key %s in map is missing in the second map",
		prettyPrint(ka, ptrSet{}, prettyPrintDepth))
}

func diffMapKeys(av, bv reflect.Value) string {
	var diffs []string
	// TODO: We produce extraneous diffs for composite keys.
	for _, ka := range av.MapKeys() {
		be := bv.MapIndex(ka)
		if !be.IsValid() {
			diffs = append(diffs, fmt.Sprintf("missing key: %s",
				PrettyPrint(ka.Interface())))
		}
	}
	for _, kb := range bv.MapKeys() {
		ae := av.MapIndex(kb)
		if !ae.IsValid() {
			diffs = append(diffs, fmt.Sprintf("extra key: %s",
				PrettyPrint(kb.Interface())))
		}
	}
	sort.Strings(diffs)
	return strings.Join(diffs, ", ")
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
