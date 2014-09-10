// Copyright (c) 2014 Arista Networks, Inc.  All rights reserved.
// Arista Networks, Inc. Confidential and Proprietary.

package test_test // yes!

import (
	"testing"

	. "arista/test"
)

type comparable struct {
	a uint32
	t *testing.T
}

func (c comparable) Equal(v interface{}) bool {
	other, ok := v.(comparable)
	// Deliberately ignore t.
	return ok && c.a == other.a
}

type builtinCompare struct {
	a uint32
	b string
}

func TestDeepEqual(t *testing.T) {
	var emptyMapString map[string]interface{}
	testcases := []struct {
		a, b  interface{}
		equal bool
	}{{
		nil,
		nil,
		true,
	}, {
		uint8(5),
		uint8(5),
		true,
	}, {
		nil,
		uint8(5),
		false,
	}, {
		uint8(5),
		nil,
		false,
	}, {
		uint16(1),
		uint16(2),
		false,
	}, {
		int8(1),
		int16(1),
		false,
	}, {
		true,
		true,
		true,
	}, {
		float32(3.1415),
		float32(3.1415),
		true,
	}, {
		float32(3.1415),
		float32(3.1416),
		false,
	}, {
		float64(3.14159265),
		float64(3.14159265),
		true,
	}, {
		float64(3.14159265),
		float64(3.14159266),
		false,
	}, {
		emptyMapString,
		emptyMapString,
		true,
	}, {
		&emptyMapString,
		&emptyMapString,
		true,
	}, {
		emptyMapString,
		&emptyMapString,
		false,
	}, {
		&emptyMapString,
		emptyMapString,
		false,
	}, {
		map[string]interface{}{"a": uint32(42)},
		map[string]interface{}{"a": uint32(42)},
		true,
	}, {
		map[string]interface{}{"a": int32(42)},
		map[string]interface{}{"a": int32(51)},
		false,
	}, {
		map[string]interface{}{"a": uint32(42)},
		map[string]interface{}{},
		false,
	}, {
		map[string]interface{}{},
		map[string]interface{}{"a": uint32(42)},
		false,
	}, {
		map[string]interface{}{"a": uint64(42), "b": "extra"},
		map[string]interface{}{"a": uint64(42)},
		false,
	}, {
		map[string]interface{}{"a": uint64(42)},
		map[string]interface{}{"a": uint64(42), "b": "extra"},
		false,
	}, {
		map[uint32]interface{}{uint32(42): "foo"},
		map[uint32]interface{}{uint32(42): "foo"},
		true,
	}, {
		map[uint32]interface{}{uint32(42): "foo"},
		map[uint32]interface{}{uint32(51): "foo"},
		false,
	}, {
		map[uint32]interface{}{uint32(42): "foo"},
		map[uint32]interface{}{uint32(42): "foo", uint32(51): "bar"},
		false,
	}, {
		map[uint32]interface{}{uint32(42): "foo"},
		map[uint64]interface{}{uint64(42): "foo"},
		false,
	}, {
		map[uint64]interface{}{uint64(42): "foo"},
		map[uint64]interface{}{uint64(42): "foo"},
		true,
	}, {
		map[uint64]interface{}{uint64(42): "foo"},
		map[uint64]interface{}{uint64(51): "foo"},
		false,
	}, {
		map[uint64]interface{}{uint64(42): "foo"},
		map[uint64]interface{}{uint64(42): "foo", uint64(51): "bar"},
		false,
	}, {
		map[uint64]interface{}{uint64(42): "foo"},
		map[interface{}]interface{}{uint32(42): "foo"},
		false,
	}, {
		map[interface{}]interface{}{"a": uint32(42)},
		map[string]interface{}{"a": uint32(42)},
		false,
	}, {
		map[interface{}]interface{}{&map[string]interface{}{"a": "foo", "b": int16(8)}: "foo"},
		map[interface{}]interface{}{&map[string]interface{}{"a": "foo", "b": int16(8)}: "foo"},
		true,
	}, {
		map[interface{}]interface{}{&map[string]interface{}{"a": "foo", "b": uint32(8)}: "foo"},
		map[interface{}]interface{}{&map[string]interface{}{"a": "foo", "b": uint32(8)}: "fox"},
		false,
	}, {
		map[interface{}]interface{}{&map[string]interface{}{"a": "foo", "b": uint32(8)}: "foo"},
		map[interface{}]interface{}{&map[string]interface{}{"a": "foo", "b": uint32(5)}: "foo"},
		false,
	}, {
		map[interface{}]interface{}{&map[string]interface{}{"a": "foo", "b": uint32(8)}: "foo"},
		map[interface{}]interface{}{&map[string]interface{}{"a": "foo"}: "foo"},
		false,
	}, {
		map[interface{}]interface{}{
			&map[string]interface{}{"a": "foo", "b": int16(8)}: "foo",
			&map[string]interface{}{"a": "foo", "b": int8(8)}:  "foo",
		},
		map[interface{}]interface{}{
			&map[string]interface{}{"a": "foo", "b": int16(8)}: "foo",
			&map[string]interface{}{"a": "foo", "b": int8(8)}:  "foo",
		},
		true,
	}, {
		map[interface{}]interface{}{
			&map[string]interface{}{"a": "foo", "b": int16(8)}: "foo",
			&map[string]interface{}{"a": "foo", "b": int8(8)}:  "foo",
		},
		map[interface{}]interface{}{
			&map[string]interface{}{"a": "foo", "b": int16(8)}: "foo",
			&map[string]interface{}{"a": "foo", "b": int8(5)}:  "foo",
		},
		false,
	}, {
		map[interface{}]interface{}{
			&map[string]interface{}{"a": "foo", "b": int16(8)}: "foo",
			&map[string]interface{}{"a": "foo", "b": int8(8)}:  "foo",
		},
		map[interface{}]interface{}{
			&map[string]interface{}{"a": "foo", "b": int16(8)}: "foo",
			&map[string]interface{}{"a": "foo", "b": int32(8)}: "foo",
		},
		false,
	}, {
		map[interface{}]interface{}{
			&map[string]interface{}{"a": "foo", "b": int16(8)}: "foo",
			&map[string]interface{}{"a": "foo", "b": int8(8)}:  "foo",
		},
		map[interface{}]interface{}{
			&map[string]interface{}{"a": "foo", "b": int16(8)}: "foo",
		},
		false,
	}, {
		map[interface{}]interface{}{
			&map[string]interface{}{"a": "foo", "b": int16(8)}: "foo",
		},
		map[interface{}]interface{}{
			&map[string]interface{}{"a": "foo", "b": int16(8)}: "foo",
			&map[string]interface{}{"a": "foo", "b": int8(8)}:  "foo",
		},
		false,
	}, {
		[]string{},
		[]string{},
		true,
	}, {
		[]string{"foo", "bar"},
		[]string{"foo", "bar"},
		true,
	}, {
		[]string{"foo", "bar"},
		[]string{"foo"},
		false,
	}, {
		[]string{"foo"},
		[]string{"foo", "bar"},
		false,
	}, {
		[]string{"foo", "bar"},
		[]string{"bar", "foo"},
		false,
	}, {
		&[]string{},
		[]string{},
		false,
	}, {
		&[]string{},
		&[]string{},
		true,
	}, {
		&[]string{"foo", "bar"},
		&[]string{"foo", "bar"},
		true,
	}, {
		&[]string{"foo", "bar"},
		&[]string{"foo"},
		false,
	}, {
		&[]string{"foo"},
		&[]string{"foo", "bar"},
		false,
	}, {
		&[]string{"foo", "bar"},
		&[]string{"bar", "foo"},
		false,
	}, {
		comparable{a: 42},
		comparable{a: 42},
		true,
	}, {
		comparable{a: 42, t: t},
		comparable{a: 42},
		true,
	}, {
		comparable{a: 42},
		comparable{a: 42, t: t},
		true,
	}, {
		comparable{a: 42},
		comparable{a: 51},
		false,
	}, {
		builtinCompare{a: 42, b: "foo"},
		builtinCompare{a: 42, b: "foo"},
		true,
	}, {
		builtinCompare{a: 42, b: "foo"},
		builtinCompare{a: 42, b: "bar"},
		false,
	}}

	for _, test := range testcases {
		if actual := DeepEqual(test.a, test.b); actual != test.equal {
			t.Errorf("DeepEqual returned %v but we wanted %v for %#v == %#v",
				actual, test.equal, test.a, test.b)
		}
	}
}
