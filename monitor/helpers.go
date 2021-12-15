// Copyright (c) 2021 Arista Networks, Inc.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the COPYING file.

package monitor

import (
	"expvar"
	"fmt"
	"strings"
)

// VarsToString gives a string with all exported variables
// the returned string is in a pretty format.
func VarsToString() string {
	sb := strings.Builder{}
	sb.WriteString("{\n")
	first := true
	expvar.Do(func(kv expvar.KeyValue) {
		if !first {
			sb.WriteString(",\n")
		}
		first = false
		sb.WriteString(fmt.Sprintf("\t%q: %s", kv.Key, kv.Value))
	})
	sb.WriteString("\n}")
	return sb.String()
}
