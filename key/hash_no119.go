// Copyright (c) 2022 Arista Networks, Inc.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the COPYING file.

//go:build !go1.19

package key

import (
	"hash/maphash"
)

// hashBytes is a less efficient version of maphash.Bytes, which was introduced in go1.19
func hashBytes(seed maphash.Seed, v []byte) uint64 {
	var h maphash.Hash
	h.SetSeed(seed)
	h.Write(v)
	return h.Sum64()
}

// hashString is a less efficient version of maphash.String, which was introduced in go1.19
func hashString(seed maphash.Seed, v string) uint64 {
	var h maphash.Hash
	h.SetSeed(seed)
	h.WriteString(v)
	return h.Sum64()
}
