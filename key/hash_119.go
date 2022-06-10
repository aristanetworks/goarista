// Copyright (c) 2022 Arista Networks, Inc.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the COPYING file.

//go:build go1.19

package key

import (
	"hash/maphash"
)

func hashBytes(seed maphash.Seed, v []byte) uint64 {
	return maphash.Bytes(seed, v)
}

func hashString(seed maphash.Seed, v string) uint64 {
	return maphash.String(seed, v)
}
