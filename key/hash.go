// Copyright (c) 2016 Arista Networks, Inc.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the COPYING file.

package key

import "hash/maphash"

// hashInterfaceSeed is the process-stable seed used by HashInterface and by
// the Hash methods on the unexported key types.
var hashInterfaceSeed = maphash.MakeSeed()

// Hash and HashInterface both dispatch on the input's concrete type and call
// into maphash. They look parallel but can't share their type switches:
// Hash switches on the unexported Key wrapper types (int64Key, strKey, ...)
// and returns a fresh uint64 via maphash.Comparable; writeHashAny — the
// helper used for slice/map elements — switches on raw types and writes into
// an existing maphash.Hash via maphash.WriteComparable. Funneling either
// through the other regresses scalar performance by 20-50% because
// maphash.Comparable[T=any] goes through the runtime's interface-type
// hasher, whereas a concrete T specializes to a direct call.

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

// HashInterface computes the hash of a value.
func HashInterface(v any) uintptr {
	if vv, ok := v.(Key); ok {
		v = vv.Key()
	}
	switch v := v.(type) {
	case nil:
		return uintptr(maphash.Comparable(hashInterfaceSeed, v))
	case string:
		return uintptr(maphash.String(hashInterfaceSeed, v))
	case []byte:
		return uintptr(maphash.Bytes(hashInterfaceSeed, v))
	case int:
		return uintptr(maphash.Comparable(hashInterfaceSeed, v))
	case int8:
		return uintptr(maphash.Comparable(hashInterfaceSeed, v))
	case int16:
		return uintptr(maphash.Comparable(hashInterfaceSeed, v))
	case int32:
		return uintptr(maphash.Comparable(hashInterfaceSeed, v))
	case int64:
		return uintptr(maphash.Comparable(hashInterfaceSeed, v))
	case uint:
		return uintptr(maphash.Comparable(hashInterfaceSeed, v))
	case uint16:
		return uintptr(maphash.Comparable(hashInterfaceSeed, v))
	case uint32:
		return uintptr(maphash.Comparable(hashInterfaceSeed, v))
	case uint64:
		return uintptr(maphash.Comparable(hashInterfaceSeed, v))
	case float32:
		return uintptr(maphash.Comparable(hashInterfaceSeed, v))
	case float64:
		return uintptr(maphash.Comparable(hashInterfaceSeed, v))
	case bool:
		return uintptr(maphash.Comparable(hashInterfaceSeed, v))
	case map[string]any:
		return uintptr(hashMapKey(hashInterfaceSeed, mapKey(v)))
	case []any:
		return uintptr(hashSliceAny(hashInterfaceSeed, v))
	case Pointer:
		return uintptr(hashPath(hashInterfaceSeed, v.Pointer()))
	case Path:
		return uintptr(hashPath(hashInterfaceSeed, v))
	case Hashable:
		return uintptr(maphash.Comparable(hashInterfaceSeed, v.Hash()))
	default:
		return uintptr(maphash.Comparable(hashInterfaceSeed, v))
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

// writeHashAny hashes a slice/map element into an existing maphash.Hash.
// The typed cases let maphash.WriteComparable instantiate at a concrete T
// so it specializes to a direct hash call; the any-typed default goes
// through the runtime's interface-type hasher and is noticeably slower
// (~2 ns per element).
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
	case string:
		h.WriteString(v)
	case int:
		maphash.WriteComparable(h, v)
	case int8:
		maphash.WriteComparable(h, v)
	case int16:
		maphash.WriteComparable(h, v)
	case int32:
		maphash.WriteComparable(h, v)
	case int64:
		maphash.WriteComparable(h, v)
	case uint:
		maphash.WriteComparable(h, v)
	case uint16:
		maphash.WriteComparable(h, v)
	case uint32:
		maphash.WriteComparable(h, v)
	case uint64:
		maphash.WriteComparable(h, v)
	case float32:
		maphash.WriteComparable(h, v)
	case float64:
		maphash.WriteComparable(h, v)
	case bool:
		maphash.WriteComparable(h, v)
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
