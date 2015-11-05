// Copyright (C) 2015  Arista Networks, Inc.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the COPYING file.

package key

import (
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
)

// Stringify transforms an arbitrary interface into its string
// representation.  We need to do this because some entities use the string
// representation of their keys as their names.
func Stringify(key interface{}) (string, error) {
	if key == nil {
		return "", errors.New("Unable to stringify nil")
	}
	var str string
	switch key := key.(type) {
	case bool:
		str = strconv.FormatBool(key)
	case uint8:
		str = strconv.FormatUint(uint64(key), 10)
	case uint16:
		str = strconv.FormatUint(uint64(key), 10)
	case uint32:
		str = strconv.FormatUint(uint64(key), 10)
	case uint64:
		str = strconv.FormatUint(key, 10)
	case int8:
		str = strconv.FormatInt(int64(key), 10)
	case int16:
		str = strconv.FormatInt(int64(key), 10)
	case int32:
		str = strconv.FormatInt(int64(key), 10)
	case int64:
		str = strconv.FormatInt(key, 10)
	case float32:
		str = "f" + strconv.FormatInt(int64(math.Float32bits(key)), 10)
	case float64:
		str = "f" + strconv.FormatInt(int64(math.Float64bits(key)), 10)
	case string:
		str = key
	case map[string]interface{}:
		keys := SortedKeys(key)
		var err error
		for i, k := range keys {
			v := key[k]
			keys[i], err = Stringify(v)
			if err != nil {
				return str, err
			}
		}
		str = strings.Join(keys, "_")
	case *map[string]interface{}:
		return Stringify(*key)
	case map[Key]interface{}:
		m := make(map[string]interface{}, len(key))
		for k, v := range key {
			m[k.String()] = v
		}
		keys := SortedKeys(m)
		for i, k := range keys {
			v := m[k]
			sk, err := Stringify(k)
			if err != nil {
				return str, err
			}
			sv, err := Stringify(v)
			if err != nil {
				return str, err
			}
			keys[i] = sk + "=" + sv
		}
		str = strings.Join(keys, "_")

	case Keyable:
		return key.KeyString(), nil

	default:
		return "", fmt.Errorf("Unable to stringify type %T", key)
	}

	return str, nil
}
