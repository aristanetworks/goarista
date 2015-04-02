// Copyright (c) 2015 Arista Networks, Inc.  All rights reserved.
// Arista Networks, Inc. Confidential and Proprietary.

package test

import (
	"fmt"
	"os"
	"reflect"
	"sort"
	"strconv"
)

// PrettyPrint tries to display a human readable version of an interface
func PrettyPrint(v interface{}) string {
	return PrettyPrintWithDepth(v, prettyPrintDepth)
}

// PrettyPrintWithDepth tries to display a human readable version of an interface
// and allows to define the depth of the print
func PrettyPrintWithDepth(v interface{}, depth int) string {
	return prettyPrint(reflect.ValueOf(v), ptrSet{}, depth)
}

var prettyPrintDepth = 3

func init() {
	d := os.Getenv("PPDEPTH")
	if d, ok := strconv.Atoi(d); ok == nil && d >= 0 {
		prettyPrintDepth = d
	}
}

type ptrSet map[uintptr]struct{}

func prettyPrint(v reflect.Value, done ptrSet, depth int) string {
	return prettyPrintWithType(v, done, depth, true)
}

func prettyPrintWithType(v reflect.Value, done ptrSet, depth int, showType bool) string {
	if depth < 0 {
		return "<max_depth>"
	}
	switch v.Kind() {
	case reflect.Invalid:
		return "nil"
	case reflect.Bool:
		return fmt.Sprintf("%t", v.Bool())
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		if showType {
			return fmt.Sprintf("%s(%d)", v.Type(), v.Int())
		}
		return fmt.Sprintf("%d", v.Int())
	case reflect.Uint, reflect.Uintptr,
		reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		if showType {
			return fmt.Sprintf("%s(%d)", v.Type(), v.Uint())
		}
		return fmt.Sprintf("%d", v.Uint())
	case reflect.Float32, reflect.Float64:
		if showType {
			return fmt.Sprintf("%s(%g)", v.Type(), v.Float())
		}
		return fmt.Sprintf("%g", v.Float())
	case reflect.Complex64, reflect.Complex128:
		if showType {
			return fmt.Sprintf("%s(%g)", v.Type(), v.Complex())
		}
		return fmt.Sprintf("%g", v.Complex())
	case reflect.String:
		return fmt.Sprintf("%q", v.String())
	case reflect.Ptr:
		return "*" + prettyPrintWithType(v.Elem(), done, depth-1, showType)
	case reflect.Interface:
		return prettyPrintWithType(v.Elem(), done, depth-1, showType)
	case reflect.Map:
		var r []byte
		r = append(r, []byte(v.Type().String())...)
		r = append(r, '{')
		var elems mapEntries
		for _, k := range v.MapKeys() {
			elem := &mapEntry{
				k: prettyPrint(k, done, depth-1),
				v: prettyPrint(v.MapIndex(k), done, depth-1),
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
		// Circular dependency?
		if v.CanAddr() {
			ptr := v.UnsafeAddr()
			if _, ok := done[ptr]; ok {
				return fmt.Sprintf("%s{<circular dependency>}", v.Type().String())
			}
			done[ptr] = struct{}{}
		}
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
			r = append(r, prettyPrint(v.Field(i), done, depth-1)...)
		}
		r = append(r, '}')
		return string(r)
	case reflect.Chan:
		var ptr, bufsize string
		if v.Pointer() == 0 {
			ptr = "nil"
		} else {
			ptr = fmt.Sprintf("0x%x", v.Pointer())
		}
		if v.Cap() > 0 {
			bufsize = fmt.Sprintf("[%d]", v.Cap())
		}
		return fmt.Sprintf("(%s)(%s)%s", v.Type().String(), ptr, bufsize)
	case reflect.Func:
		return "func(...)"
	case reflect.Array, reflect.Slice:
		l := v.Len()
		var r []byte
		r = append(r, []byte(v.Type().String())...)
		r = append(r, '{')
		for i := 0; i < l; i++ {
			if i > 0 {
				r = append(r, []byte(", ")...)
			}
			r = append(r, prettyPrintWithType(v.Index(i), done, depth-1, false)...)
		}
		r = append(r, '}')
		return string(r)
	case reflect.UnsafePointer:
		var ptr string
		if v.Pointer() == 0 {
			ptr = "nil"
		} else {
			ptr = fmt.Sprintf("0x%x", v.Pointer())
		}
		if showType {
			return fmt.Sprintf("(unsafe.Pointer)(%s)", ptr)
		}
		return fmt.Sprintf("%s", ptr)
	default:
		return fmt.Sprintf("%#v", v.Interface())
	}
}

// IsNilCheck checks if the two values can compare to nil
// if checked == false, deep equal should continue
func IsNilCheck(a, b interface{}) (bool /*checked*/, string) {
	return isNilCheck(reflect.ValueOf(a), reflect.ValueOf(b))
}
