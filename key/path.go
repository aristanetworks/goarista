// Copyright (c) 2018 Arista Networks, Inc.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the COPYING file.

package key

import "bytes"

// Path represents a path decomposed into elements where each
// element is a Key. A Path can be interpreted as either
// absolute or relative depending on how it is used.
type Path []Key

// String returns the Path as an absolute path string.
func (p Path) String() string {
	if len(p) == 0 {
		return "/"
	}
	var buf bytes.Buffer
	for _, element := range p {
		buf.WriteByte('/')
		buf.WriteString(element.String())
	}
	return buf.String()
}
