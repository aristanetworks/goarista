// Copyright (c) 2015 Arista Networks, Inc.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the COPYING file.

package key

import (
	"fmt"
	"math"
	"testing"
)

func TestStringify(t *testing.T) {
	testcases := []struct {
		name   string
		input  interface{}
		output string // or expected panic error message.
	}{{
		name:   "nil",
		input:  nil,
		output: "<nil>",
	}, {
		name:   "struct{}",
		input:  struct{}{},
		output: "Unable to stringify type struct {}: struct {}{}",
	}, {
		name:   "string",
		input:  "foobar",
		output: "foobar",
	}, {
		name:   "valid non-ASCII UTF-8 string",
		input:  "日本語",
		output: "日本語",
	}, {
		name:   "invalid UTF-8 string 1",
		input:  string([]byte{0xef, 0xbf, 0xbe, 0xbe, 0xbe, 0xbe, 0xbe}),
		output: "77++vr6+vg==",
	}, {
		name:   "invalid UTF-8 string 2",
		input:  string([]byte{0xef, 0xbf, 0xbe, 0xbe, 0xbe, 0xbe, 0xbe, 0x23}),
		output: "77++vr6+viM=",
	}, {
		name:   "invalid UTF-8 string 3",
		input:  string([]byte{0xef, 0xbf, 0xbe, 0xbe, 0xbe, 0xbe, 0xbe, 0x23, 0x24}),
		output: "77++vr6+viMk",
	}, {
		name:   "uint8",
		input:  uint8(43),
		output: "43",
	}, {
		name:   "uint16",
		input:  uint16(43),
		output: "43",
	}, {
		name:   "uint32",
		input:  uint32(43),
		output: "43",
	}, {
		name:   "uint64",
		input:  uint64(43),
		output: "43",
	}, {
		name:   "max uint64",
		input:  uint64(math.MaxUint64),
		output: "18446744073709551615",
	}, {
		name:   "int8",
		input:  int8(-32),
		output: "-32",
	}, {
		name:   "int16",
		input:  int16(-32),
		output: "-32",
	}, {
		name:   "int32",
		input:  int32(-32),
		output: "-32",
	}, {
		name:   "int64",
		input:  int64(-32),
		output: "-32",
	}, {
		name:   "true",
		input:  true,
		output: "true",
	}, {
		name:   "false",
		input:  false,
		output: "false",
	}, {
		name:   "float32",
		input:  float32(2.345),
		output: "f1075188859",
	}, {
		name:   "float64",
		input:  float64(-34.6543),
		output: "f-4593298060402564373",
	}, {
		name: "map[string]interface{}",
		input: map[string]interface{}{
			"b": uint32(43),
			"a": "foobar",
			"ex": map[string]interface{}{
				"d": "barfoo",
				"c": uint32(45),
			},
		},
		output: "foobar_43_45_barfoo",
	}, {
		name: "nil inside map[string]interface{}",
		input: map[string]interface{}{
			"n": nil,
		},
		output: "<nil>",
	}, {
		name: "[]interface{}",
		input: []interface{}{
			uint32(42),
			true,
			"foo", NewMap(
				New("a"), "b",
				New("b"), "c"),
		},
		output: "42,true,foo,a=b_b=c",
	}, {
		name:   "pointer",
		input:  NewPointer(Path{New("foo"), New("bar")}),
		output: "{/foo/bar}",
	}, {
		name:   "[]byte",
		input:  []byte{0x1},
		output: "AQ==",
	}, {
		name:   "map with []byte",
		input:  map[string]interface{}{"key1": []byte{0x1}, "key2": uint32(42)},
		output: "AQ==_42",
	}}

	for _, tcase := range testcases {
		// Pardon the contraption used to catch panic's in error cases.
		func() {
			defer func() {
				if e := recover(); e != nil {
					if tcase.output != e.(error).Error() {
						t.Errorf("Test %s: Error returned: %q but wanted %q",
							tcase.name, e, tcase.output)
					}
				}
			}()

			result := stringify(tcase.input)
			if tcase.output != result {
				t.Errorf("Test %s: Result is different\nReceived: %s\nExpected: %s",
					tcase.name, result, tcase.output)
			}
			keyString := StringKey(New(tcase.input))
			if tcase.output != keyString {
				t.Errorf("Test %s: KeyString result is different\nReceived: %s\nExpected: %s",
					tcase.name, keyString, tcase.output)
			}
		}()
	}
}

var benchcases = []Key{
	New(nil),
	New("foobar"),
	New(string([]byte{0xef, 0xbf, 0xbe, 0xbe, 0xbe, 0xbe, 0xbe})),
	New(uint64(123456)),
	New(int8(-32)),
	New(true),
	New(float32(2.345)),
	New(float64(-34.6543)),
	New(map[string]interface{}{
		"b": uint32(43),
		"a": "foobar",
		"ex": map[string]interface{}{
			"d": "barfoo",
			"c": uint32(45),
		},
	}),
	New([]interface{}{
		uint32(42),
		true,
		"foo", NewMap(
			New("a"), "b",
			New("b"), "c"),
	}),
	New(NewPointer(Path{New("foo"), New("bar")})),
}

var str string

func BenchmarkStringification(b *testing.B) {
	for _, k := range benchcases {
		b.Run(fmt.Sprintf("SI_%T", k.Key()), func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				str, _ = StringifyInterface(k.Key())
			}
		})
		b.Run(fmt.Sprintf("SK_%T", k.Key()), func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				str = StringKey(k)
			}
		})
	}
}
