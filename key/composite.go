// Copyright (c) 2016 Arista Networks, Inc.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the COPYING file.

package key

func hashInterface(v interface{}) uintptr {
	if vv, ok := v.(Key); ok {
		v = vv.Key()
	}
	switch v := v.(type) {
	case map[string]interface{}:
		return hashMapString(v)
	case map[Key]interface{}:
		return hashMapKey(v)
	case []interface{}:
		return hashSlice(v)
	case Pointer:
		// This case applies to pointers used
		// as values in maps or slices (i.e.
		// not wrapped in a key).
		return hashSlice(pointerToSlice(v))
	case Path:
		// This case applies to paths used
		// as values in maps or slices (i.e
		// not wrapped in a kay).
		return hashSlice(pathToSlice(v))
	case Hashable:
		return uintptr(v.Hash())
	default:
		return _nilinterhash(v)
	}
}

// HashInterface computes the hash of a Key
func HashInterface(v interface{}) uintptr {
	return hashInterface(v)
}

func hashMapString(m map[string]interface{}) uintptr {
	h := uintptr(31 * (len(m) + 1))
	for k, v := range m {
		// Use addition so that the order of iteration doesn't matter.
		h += _strhash(k)
		h += hashInterface(v)
	}
	return h
}

func hashMapKey(m map[Key]interface{}) uintptr {
	h := uintptr(31 * (len(m) + 1))
	for k, v := range m {
		// Use addition so that the order of iteration doesn't matter.
		switch k := k.(type) {
		case interfaceKey:
			h += _nilinterhash(k.key)
		case compositeKey:
			h += hashMapString(k.m)
		}
		h += hashInterface(v)
	}
	return h
}

func hashSlice(s []interface{}) uintptr {
	h := uintptr(31 * (len(s) + 1))
	for _, v := range s {
		h += hashInterface(v)
	}
	return h
}
