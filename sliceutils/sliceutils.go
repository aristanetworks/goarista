// Copyright (c) 2023 Arista Networks, Inc.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the COPYING file.

package sliceutils

import (
	"golang.org/x/exp/slices"
)

// ToAnySlice takes a []T, and converts it into a []any.
// This is a common conversion when a function expects a []any but the calling code has a []T, with
// T not being any.
func ToAnySlice[T any](in []T) []any {
	l := len(in)
	out := make([]any, l)
	for i := 0; i < l; i++ {
		out[i] = any(in[i])
	}
	return out
}

// SortedStringKeys returns the keys of a string to anything map
// in a sorted slice.
func SortedStringKeys[T any](m map[string]T) []string {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	slices.Sort(keys)
	return keys
}
