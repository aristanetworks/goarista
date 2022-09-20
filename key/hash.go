// Copyright (c) 2016 Arista Networks, Inc.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the COPYING file.

package key

import (
	"encoding/binary"
	"hash/maphash"
	"math"
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

func _nilinterhash(v interface{}) uintptr {
	return nilinterhash(unsafe.Pointer(&v), 0)
}

// Hash will hash a key. Can be used with
// [github.com/aristanetworks/gomap.Map].
func Hash(seed maphash.Seed, k Key) uint64 {
	var buf [8]byte
	switch v := k.(type) {
	case mapKey:
		return hashMapKey(seed, v)
	case interfaceKey:
		s := v.Hash()
		// Mix up the hash to ensure it covers 64-bits
		binary.LittleEndian.PutUint64(buf[:8], uint64(s))
		return hashBytes(seed, buf[:8])
	case strKey:
		return hashString(seed, string(v))
	case bytesKey:
		return hashBytes(seed, []byte(v))
	case int8Key:
		buf[0] = byte(v)
		return hashBytes(seed, buf[:1])
	case int16Key:
		binary.LittleEndian.PutUint16(buf[:2], uint16(v))
		return hashBytes(seed, buf[:2])
	case int32Key:
		binary.LittleEndian.PutUint32(buf[:4], uint32(v))
		return hashBytes(seed, buf[:4])
	case int64Key:
		binary.LittleEndian.PutUint64(buf[:8], uint64(v))
		return hashBytes(seed, buf[:8])
	case uint8Key:
		buf[0] = byte(v)
		return hashBytes(seed, buf[:1])
	case uint16Key:
		binary.LittleEndian.PutUint16(buf[:2], uint16(v))
		return hashBytes(seed, buf[:2])
	case uint32Key:
		binary.LittleEndian.PutUint32(buf[:4], uint32(v))
		return hashBytes(seed, buf[:4])
	case uint64Key:
		binary.LittleEndian.PutUint64(buf[:8], uint64(v))
		return hashBytes(seed, buf[:8])
	case float32Key:
		binary.LittleEndian.PutUint32(buf[:4], math.Float32bits(float32(v)))
		return hashBytes(seed, buf[:4])
	case float64Key:
		binary.LittleEndian.PutUint64(buf[:8], math.Float64bits(float64(v)))
		return hashBytes(seed, buf[:8])
	case boolKey:
		if v {
			buf[0] = 1
		}
		return hashBytes(seed, buf[:1])
	case sliceKey:
		return hashSliceKey(seed, v)
	case pointerKey:
		return hashSliceKey(seed, v.sliceKey)
	case pathKey:
		return hashSliceKey(seed, v.sliceKey)
	case nilKey:
		return hashBytes(seed, nil)
	case Hashable:
		// Mix up the hash to ensure it covers 64-bits
		binary.LittleEndian.PutUint64(buf[:8], v.Hash())
		return hashBytes(seed, buf[:8])
	default:
		s := _nilinterhash(v.Key())
		binary.LittleEndian.PutUint64(buf[:8], uint64(s))
		return hashBytes(seed, buf[:8])
	}
}

func hashMapKey(seed maphash.Seed, m mapKey) uint64 {
	var buf [8]byte
	var h maphash.Hash
	h.SetSeed(seed)
	var s uint64
	for k, v := range m {
		h.WriteString(k)
		binary.BigEndian.PutUint64(buf[:8], uint64(HashInterface(v)))
		h.Write(buf[:8])
		// combine hashes with addition so that order doesn't
		// matter
		s += h.Sum64()
		h.Reset()
	}
	return s

}

func hashSliceKey(seed maphash.Seed, s sliceKey) uint64 {
	var buf [8]byte
	var h maphash.Hash
	h.SetSeed(seed)
	for _, v := range s {
		binary.BigEndian.PutUint64(buf[:8], uint64(HashInterface(v)))
		h.Write(buf[:8])
	}
	return h.Sum64()
}
