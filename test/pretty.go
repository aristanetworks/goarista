// Copyright (c) 2015 Arista Networks, Inc.  All rights reserved.
// Arista Networks, Inc. Confidential and Proprietary.

package test

import (
	"fmt"
	"reflect"
	"sort"
)

// PrettyPrint tries to display a human readable version of an interface
func PrettyPrint(v interface{}) string {
	return prettyPrint(reflect.ValueOf(v))
}

func prettyPrint(v reflect.Value) string {
	switch v.Kind() {
	case reflect.Invalid:
		return "nil"
	case reflect.Bool:
		return fmt.Sprintf("%t", v.Bool())
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return fmt.Sprintf("%s(%d)", v.Type(), v.Int())
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return fmt.Sprintf("%s(%d)", v.Type(), v.Uint())
	case reflect.Float32, reflect.Float64:
		return fmt.Sprintf("%s(%g)", v.Type(), v.Float())
	case reflect.Complex64, reflect.Complex128:
		return fmt.Sprintf("%s(%g)", v.Type(), v.Complex())
	case reflect.String:
		return fmt.Sprintf("%q", v.String())
	case reflect.Ptr:
		return "*" + prettyPrint(v.Elem())
	case reflect.Interface:
		return prettyPrint(v.Elem())
	case reflect.Map:
		var r []byte
		r = append(r, []byte(v.Type().String())...)
		r = append(r, '{')
		var elems mapEntries
		for _, k := range v.MapKeys() {
			elem := &mapEntry{
				k: prettyPrint(k),
				v: prettyPrint(v.MapIndex(k)),
			}
			elems.entries = append(elems.entries, elem)
		}
		sort.Sort(&elems)
		for i, e := range elems.entries {
			if i > 0 {
				r = append(r, []byte(", ")...)
			}
			r = append(r, []byte(e.k)...)
			r = append(r, ':')
			r = append(r, []byte(e.v)...)
		}
		r = append(r, '}')
		return string(r)
	case reflect.Struct:
		var r []byte
		r = append(r, []byte(v.Type().String())...)
		r = append(r, '{')
		for i := 0; i < v.NumField(); i++ {
			if i > 0 {
				r = append(r, []byte(", ")...)
			}
			sf := v.Type().Field(i)
			r = append(r, sf.Name...)
			r = append(r, ':')
			r = append(r, prettyPrint(v.Field(i))...)
		}
		r = append(r, '}')
		return string(r)
	default:
		return fmt.Sprintf("%#v", v.Interface())
	}
}

// IsNilCheck checks if the two values can compare to nil
// if checked == false, deep equal should continue
func IsNilCheck(a, b interface{}) (bool /*checked*/, string) {
	return isNilCheck(reflect.ValueOf(a), reflect.ValueOf(b))
}
