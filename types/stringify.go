// Copyright (c) 2015 Arista Networks, Inc.  All rights reserved.
// Arista Networks, Inc. Confidential and Proprietary.

package types

import (
	"errors"
	"fmt"
	"sort"
	"strconv"
)

// StringifyInterface transforms an arbitrary interface into its string
// representation.  We need to do this because some entities use the string
// representation of their keys as their names.
func StringifyInterface(key interface{}) (string, error) {
	var str string
	if key == nil {
		return "", errors.New("Unable to decode nil key")
	}
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
		str = strconv.FormatFloat(float64(key), 'b', -1, 32)
	case float64:
		str = strconv.FormatFloat(key, 'b', -1, 64)
	case string:
		str = key
	case map[string]interface{}:
		keys := make([]string, 0, len(key))
		for k := range key {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			v := key[k]
			if len(str) > 0 {
				str += "_"
			}
			s, err := StringifyInterface(v)
			if err != nil {
				return str, err
			}
			str += s
		}
	case *map[string]interface{}:
		return StringifyInterface(*key)

	default:
		return "", fmt.Errorf("don't know how to serialize key with type %T", key)
	}

	return str, nil
}
