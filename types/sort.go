// Copyright (c) 2015 Arista Networks, Inc.  All rights reserved.
// Arista Networks, Inc. Confidential and Proprietary.

package types

import (
	"sort"
)

// SortedKeys returns the keys of the given map, in a sorted order.
func SortedKeys(m map[string]interface{}) []string {
	res := make([]string, len(m))
	var i int
	for k := range m {
		res[i] = k
		i++
	}
	sort.Strings(res)
	return res
}
