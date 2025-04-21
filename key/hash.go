// Copyright (c) 2016 Arista Networks, Inc.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the COPYING file.

package key

import (
	"hash/maphash"
	"unsafe"
)

//go:noescape
//go:linkname strhash runtime.strhash
func strhash(a unsafe.Pointer, h uintptr) uintptr

func _strhash(s string) uintptr {
	return strhash(unsafe.Pointer(&s), 0)
}

//go:noescape
//go:linkname nilinterhash runtime.nilinterhash
func nilinterhash(a unsafe.Pointer, h uintptr) uintptr

func _nilinterhash(v any) uintptr {
	return nilinterhash(unsafe.Pointer(&v), 0)
}

// Hash will hash a key. Can be used with
// [github.com/aristanetworks/gomap.Map].
func Hash(seed maphash.Seed, k Key) uint64 {
	switch v := k.(type) {
	case mapKey:
		return hashMapKey(seed, v)
	case interfaceKey:
		if h, ok := v.key.(Hashable); ok {
			// Mix up the hash to ensure it covers 64-bits
			return maphash.Comparable(seed, h.Hash())
		}
		return maphash.Comparable(seed, v.key)
	case strKey:
		return maphash.String(seed, string(v))
	case bytesKey:
		return maphash.Bytes(seed, []byte(v))
	case int8Key:
		return maphash.Comparable(seed, v)
	case int16Key:
		return maphash.Comparable(seed, v)
	case int32Key:
		return maphash.Comparable(seed, v)
	case int64Key:
		return maphash.Comparable(seed, v)
	case uint8Key:
		return maphash.Comparable(seed, v)
	case uint16Key:
		return maphash.Comparable(seed, v)
	case uint32Key:
		return maphash.Comparable(seed, v)
	case uint64Key:
		return maphash.Comparable(seed, v)
	case float32Key:
		return maphash.Comparable(seed, v)
	case float64Key:
		return maphash.Comparable(seed, v)
	case boolKey:
		return maphash.Comparable(seed, v)
	case sliceKey:
		return hashSliceAny(seed, v)
	case pointerKey:
		return hashSliceAny(seed, v.sliceKey)
	case pathKey:
		return hashSliceAny(seed, v.sliceKey)
	case nilKey:
		return maphash.Bytes(seed, nil)
	case Hashable:
		return maphash.Comparable(seed, v.Hash())
	default:
		return maphash.Comparable(seed, v.Key())
	}
}

func hashMapKey(seed maphash.Seed, m mapKey) uint64 {
	var h maphash.Hash
	h.SetSeed(seed)
	var s uint64
	for k, v := range m {
		maphash.WriteComparable(&h, k)
		writeHashAny(&h, v)
		// combine hashes with addition so that order doesn't
		// matter
		s += h.Sum64()
		h.Reset()
	}
	return s
}

func hashSliceAny(seed maphash.Seed, s []any) uint64 {
	var h maphash.Hash
	h.SetSeed(seed)
	for _, v := range s {
		writeHashAny(&h, v)
	}
	return h.Sum64()
}

func writeHashAny(h *maphash.Hash, v any) {
	if vv, ok := v.(Key); ok {
		v = vv.Key()
	}
	switch v := v.(type) {
	case map[string]any:
		maphash.WriteComparable(h, hashMapKey(h.Seed(), v))
	case []any:
		maphash.WriteComparable(h, hashSliceAny(h.Seed(), v))
	case []byte:
		h.Write(v)
	case Pointer:
		// This case applies to pointers used
		// as values in maps or slices (i.e.
		// not wrapped in a key).
		maphash.WriteComparable(h, hashPath(h.Seed(), v.Pointer()))
	case Path:
		// This case applies to paths used
		// as values in maps or slices (i.e
		// not wrapped in a key).
		maphash.WriteComparable(h, hashPath(h.Seed(), v))
	case Hashable:
		maphash.WriteComparable(h, v.Hash())
	default:
		maphash.WriteComparable(h, v)
	}
}

func hashPath(seed maphash.Seed, p Path) uint64 {
	var h maphash.Hash
	h.SetSeed(seed)
	for _, v := range p {
		maphash.WriteComparable(&h, Hash(h.Seed(), v))
	}
	return h.Sum64()
}
