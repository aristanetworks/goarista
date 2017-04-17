package value

import (
	"fmt"
	"math"
	"strconv"

	"arista/types"
)

// StringifyInterface transforms an arbitrary interface into its string
// representation.
func StringifyInterface(value interface{}) (string, error) {
	var str string
	switch v := value.(type) {
	default:
		return "", fmt.Errorf("unknown type: %T", v)
	case nil:
		return "", fmt.Errorf("invalid value: nil")
	case bool:
		str = strconv.FormatBool(v)
	case uint8:
		str = strconv.FormatUint(uint64(v), 10)
	case uint16:
		str = strconv.FormatUint(uint64(v), 10)
	case uint32:
		str = strconv.FormatUint(uint64(v), 10)
	case uint64:
		str = strconv.FormatUint(v, 10)
	case int8:
		str = strconv.FormatInt(int64(v), 10)
	case int16:
		str = strconv.FormatInt(int64(v), 10)
	case int32:
		str = strconv.FormatInt(int64(v), 10)
	case int64:
		str = strconv.FormatInt(v, 10)
	case float32:
		str = "f" + strconv.FormatInt(int64(math.Float32bits(v)), 10)
	case float64:
		str = "f" + strconv.FormatInt(int64(math.Float64bits(v)), 10)
	case string:
		str = v
	case types.Pointer:
		str = fmt.Sprintf("{%s}", v.Pointer)
	case types.Enum:
		str = v.Name
	case map[string]interface{}:
		keys := types.SortedKeys(v)
		for _, k := range keys {
			val := v[k]
			if len(str) > 0 {
				str += "_"
			}
			s, err := StringifyInterface(val)
			if err != nil {
				return str, err
			}
			str += s
		}
	case *map[string]interface{}:
		return StringifyInterface(*v)
	case map[uint64]interface{}:
		remap := map[string]interface{}{}
		for k, val := range v {
			remap[strconv.FormatUint(k, 10)] = val
		}
		return StringifyInterface(remap)
	case *map[uint64]interface{}:
		remap := map[string]interface{}{}
		for k, val := range *v {
			remap[strconv.FormatUint(k, 10)] = val
		}
		return StringifyInterface(remap)
	}
	return str, nil
}
