// Copyright (c) 2017 Arista Networks, Inc.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the COPYING file.

package gnmi

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"os"
	"testing"

	"github.com/aristanetworks/goarista/test"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"

	pb "github.com/openconfig/gnmi/proto/gnmi"
)

func TestNewSetRequest(t *testing.T) {
	pathFoo := &pb.Path{
		Element: []string{"foo"},
		Elem:    []*pb.PathElem{{Name: "foo"}},
	}
	pathCli := &pb.Path{
		Origin: "cli",
	}
	pathP4 := &pb.Path{
		Origin: "p4_config",
	}
	pathOC := &pb.Path{
		Origin: "openconfig",
	}

	fileData := []struct {
		name     string
		fileName string
		content  string
	}{{
		name:     "p4_config",
		fileName: "p4TestFile",
		content:  "p4_config test",
	}, {
		name:     "originCLIFileData",
		fileName: "originCLIFile",
		content: `enable
configure
hostname new`,
	}}

	fileNames := make([]string, 2, 2)
	for i, data := range fileData {
		f, err := ioutil.TempFile("", data.name)
		if err != nil {
			t.Errorf("cannot create test file for %s", data.name)
		}
		filename := f.Name()
		defer os.Remove(filename)
		fileNames[i] = filename
		if _, err := f.WriteString(data.content); err != nil {
			t.Errorf("cannot write test file for %s", data.name)
		}
		f.Close()
	}

	testCases := map[string]struct {
		setOps []*Operation
		exp    *pb.SetRequest
	}{
		"delete": {
			setOps: []*Operation{{Type: "delete", Path: []string{"foo"}}},
			exp:    &pb.SetRequest{Delete: []*pb.Path{pathFoo}},
		},
		"update": {
			setOps: []*Operation{{Type: "update", Path: []string{"foo"}, Val: "true"}},
			exp: &pb.SetRequest{
				Update: []*pb.Update{{
					Path: pathFoo,
					Val: &pb.TypedValue{
						Value: &pb.TypedValue_JsonIetfVal{JsonIetfVal: []byte("true")}},
				}},
			},
		},
		"replace": {
			setOps: []*Operation{{Type: "replace", Path: []string{"foo"}, Val: "true"}},
			exp: &pb.SetRequest{
				Replace: []*pb.Update{{
					Path: pathFoo,
					Val: &pb.TypedValue{
						Value: &pb.TypedValue_JsonIetfVal{JsonIetfVal: []byte("true")}},
				}},
			},
		},
		"cli-replace": {
			setOps: []*Operation{{Type: "replace", Origin: "cli",
				Val: "hostname foo\nip routing"}},
			exp: &pb.SetRequest{
				Replace: []*pb.Update{{
					Path: pathCli,
					Val: &pb.TypedValue{
						Value: &pb.TypedValue_AsciiVal{AsciiVal: "hostname foo\nip routing"}},
				}},
			},
		},
		"p4_config": {
			setOps: []*Operation{{Type: "replace", Origin: "p4_config",
				Val: fileNames[0]}},
			exp: &pb.SetRequest{
				Replace: []*pb.Update{{
					Path: pathP4,
					Val: &pb.TypedValue{
						Value: &pb.TypedValue_ProtoBytes{ProtoBytes: []byte(fileData[0].content)}},
				}},
			},
		},
		"target": {
			setOps: []*Operation{{Type: "replace", Target: "JPE1234567",
				Path: []string{"foo"}, Val: "true"}},
			exp: &pb.SetRequest{
				Prefix: &pb.Path{Target: "JPE1234567"},
				Replace: []*pb.Update{{
					Path: pathFoo,
					Val: &pb.TypedValue{
						Value: &pb.TypedValue_JsonIetfVal{JsonIetfVal: []byte("true")}},
				}},
			},
		},
		"openconfig origin": {
			setOps: []*Operation{{Type: "replace", Origin: "openconfig",
				Val: "true"}},
			exp: &pb.SetRequest{
				Replace: []*pb.Update{{
					Path: pathOC,
					Val: &pb.TypedValue{
						Value: &pb.TypedValue_JsonIetfVal{
							JsonIetfVal: []byte("true"),
						},
					},
				}},
			},
		},
		"originCLI file": {
			setOps: []*Operation{{Type: "update", Origin: "cli",
				Val: fileNames[1]}},
			exp: &pb.SetRequest{
				Update: []*pb.Update{{
					Path: pathCli,
					Val: &pb.TypedValue{
						Value: &pb.TypedValue_AsciiVal{AsciiVal: fileData[1].content}},
				}},
			},
		},
		"union_replace": {
			setOps: []*Operation{{Type: "union_replace", Path: []string{"foo"}, Val: "true"}},
			exp: &pb.SetRequest{
				UnionReplace: []*pb.Update{{
					Path: pathFoo,
					Val: &pb.TypedValue{
						Value: &pb.TypedValue_JsonIetfVal{JsonIetfVal: []byte("true")}},
				}},
			},
		},
		"union_replace openconfig and cli origin": {
			setOps: []*Operation{{Type: "union_replace", Origin: "openconfig", Val: "true"},
				{Type: "union_replace", Origin: "cli", Val: fileNames[1]}},
			exp: &pb.SetRequest{
				UnionReplace: []*pb.Update{{
					Path: pathOC,
					Val: &pb.TypedValue{
						Value: &pb.TypedValue_JsonIetfVal{JsonIetfVal: []byte("true")}},
				}, {
					Path: pathCli,
					Val: &pb.TypedValue{
						Value: &pb.TypedValue_AsciiVal{AsciiVal: fileData[1].content}},
				}},
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			got, err := newSetRequest(tc.setOps)
			if err != nil {
				t.Fatal(err)
			}
			if !proto.Equal(tc.exp, got) {
				t.Errorf("Exp: %v Got: %v", tc.exp, got)
			}
		})
	}
}

func TestStrUpdateVal(t *testing.T) {
	anyBytes, err := proto.Marshal(&pb.ModelData{Name: "foobar"})
	if err != nil {
		t.Fatal(err)
	}
	anyMessage := &anypb.Any{TypeUrl: "gnmi/ModelData", Value: anyBytes}

	for name, tc := range map[string]struct {
		update *pb.Update
		exp    string
	}{
		"JSON Value": {
			update: &pb.Update{
				Value: &pb.Value{
					Value: []byte(`{"foo":"bar"}`),
					Type:  pb.Encoding_JSON}},
			exp: `{"foo":"bar"}`,
		},
		"JSON_IETF Value": {
			update: &pb.Update{
				Value: &pb.Value{
					Value: []byte(`{"foo":"bar"}`),
					Type:  pb.Encoding_JSON_IETF}},
			exp: `{"foo":"bar"}`,
		},
		"BYTES Value": {
			update: &pb.Update{
				Value: &pb.Value{
					Value: []byte{0xde, 0xad},
					Type:  pb.Encoding_BYTES}},
			exp: "3q0=",
		},
		"PROTO Value": {
			update: &pb.Update{
				Value: &pb.Value{
					Value: []byte{0xde, 0xad},
					Type:  pb.Encoding_PROTO}},
			exp: "3q0=",
		},
		"ASCII Value": {
			update: &pb.Update{
				Value: &pb.Value{
					Value: []byte("foobar"),
					Type:  pb.Encoding_ASCII}},
			exp: "foobar",
		},
		"INVALID Value": {
			update: &pb.Update{
				Value: &pb.Value{
					Value: []byte("foobar"),
					Type:  pb.Encoding(42)}},
			exp: "foobar",
		},
		"StringVal": {
			update: &pb.Update{Val: &pb.TypedValue{
				Value: &pb.TypedValue_StringVal{StringVal: "foobar"}}},
			exp: "foobar",
		},
		"IntVal": {
			update: &pb.Update{Val: &pb.TypedValue{
				Value: &pb.TypedValue_IntVal{IntVal: -42}}},
			exp: "-42",
		},
		"UintVal": {
			update: &pb.Update{Val: &pb.TypedValue{
				Value: &pb.TypedValue_UintVal{UintVal: 42}}},
			exp: "42",
		},
		"BoolVal": {
			update: &pb.Update{Val: &pb.TypedValue{
				Value: &pb.TypedValue_BoolVal{BoolVal: true}}},
			exp: "true",
		},
		"BytesVal": {
			update: &pb.Update{Val: &pb.TypedValue{
				Value: &pb.TypedValue_BytesVal{BytesVal: []byte{0xde, 0xad}}}},
			exp: "3q0=",
		},
		"FloatVal": {
			update: &pb.Update{Val: &pb.TypedValue{
				Value: &pb.TypedValue_FloatVal{FloatVal: 3.14}}},
			exp: "3.14",
		},
		"DoubleVal": {
			update: &pb.Update{Val: &pb.TypedValue{
				Value: &pb.TypedValue_DoubleVal{DoubleVal: 3.14}}},
			exp: "3.14",
		},
		"DecimalVal": {
			update: &pb.Update{Val: &pb.TypedValue{
				Value: &pb.TypedValue_DecimalVal{
					DecimalVal: &pb.Decimal64{Digits: 3014, Precision: 3},
				}}},
			exp: "3.014",
		},
		"DecimalValWithLeadingZeros": {
			update: &pb.Update{Val: &pb.TypedValue{
				Value: &pb.TypedValue_DecimalVal{
					DecimalVal: &pb.Decimal64{Digits: 314, Precision: 6},
				}}},
			exp: "0.000314",
		},
		"DecimalValWithLeadingZerosInFraction": {
			update: &pb.Update{Val: &pb.TypedValue{
				Value: &pb.TypedValue_DecimalVal{
					DecimalVal: &pb.Decimal64{Digits: 3000141, Precision: 6},
				}}},
			exp: "3.000141",
		},
		"DecimalValWithZeroPrecision": {
			update: &pb.Update{Val: &pb.TypedValue{
				Value: &pb.TypedValue_DecimalVal{
					DecimalVal: &pb.Decimal64{Digits: 314, Precision: 0},
				}}},
			exp: "314.0",
		},
		"DecimalValWithNegativeFraction": {
			update: &pb.Update{Val: &pb.TypedValue{
				Value: &pb.TypedValue_DecimalVal{
					DecimalVal: &pb.Decimal64{Digits: -314, Precision: 3},
				}}},
			exp: "-0.314",
		},
		"LeafListVal": {
			update: &pb.Update{Val: &pb.TypedValue{
				Value: &pb.TypedValue_LeaflistVal{
					LeaflistVal: &pb.ScalarArray{Element: []*pb.TypedValue{
						&pb.TypedValue{Value: &pb.TypedValue_BoolVal{BoolVal: true}},
						&pb.TypedValue{Value: &pb.TypedValue_AsciiVal{AsciiVal: "foobar"}},
					}},
				}}},
			exp: "[true, foobar]",
		},
		"AnyVal": {
			update: &pb.Update{Val: &pb.TypedValue{
				Value: &pb.TypedValue_AnyVal{AnyVal: anyMessage}}},
			exp: anyMessage.String(),
		},
		"JsonVal": {
			update: &pb.Update{Val: &pb.TypedValue{
				Value: &pb.TypedValue_JsonVal{JsonVal: []byte(`{"foo":"bar"}`)}}},
			exp: `{"foo":"bar"}`,
		},
		"JsonVal_complex": {
			update: &pb.Update{Val: &pb.TypedValue{
				Value: &pb.TypedValue_JsonVal{JsonVal: []byte(`{"foo":"bar","baz":"qux"}`)}}},
			exp: `{
  "foo": "bar",
  "baz": "qux"
}`,
		},
		"JsonIetfVal": {
			update: &pb.Update{Val: &pb.TypedValue{
				Value: &pb.TypedValue_JsonIetfVal{JsonIetfVal: []byte(`{"foo":"bar"}`)}}},
			exp: `{"foo":"bar"}`,
		},
		"AsciiVal": {
			update: &pb.Update{Val: &pb.TypedValue{
				Value: &pb.TypedValue_AsciiVal{AsciiVal: "foobar"}}},
			exp: "foobar",
		},
		"ProtoBytes": {
			update: &pb.Update{Val: &pb.TypedValue{
				Value: &pb.TypedValue_ProtoBytes{ProtoBytes: anyBytes}}},
			exp: "CgZmb29iYXI=",
		},
	} {
		t.Run(name, func(t *testing.T) {
			got := StrUpdateVal(tc.update)
			if got != tc.exp {
				t.Errorf("Expected: %q Got: %q", tc.exp, got)
			}
		})
	}
}

func TestTypedValue(t *testing.T) {
	for tname, tcase := range map[string]struct {
		in  interface{}
		exp *pb.TypedValue
	}{
		"string": {
			in:  "foo",
			exp: &pb.TypedValue{Value: &pb.TypedValue_StringVal{StringVal: "foo"}},
		},
		"int": {
			in:  42,
			exp: &pb.TypedValue{Value: &pb.TypedValue_IntVal{IntVal: 42}},
		},
		"int64": {
			in:  int64(42),
			exp: &pb.TypedValue{Value: &pb.TypedValue_IntVal{IntVal: 42}},
		},
		"uint": {
			in:  uint(42),
			exp: &pb.TypedValue{Value: &pb.TypedValue_UintVal{UintVal: 42}},
		},
		"float32": {
			in:  float32(42.234123),
			exp: &pb.TypedValue{Value: &pb.TypedValue_FloatVal{FloatVal: 42.234123}},
		},
		"float64": {
			in:  float64(42.234124222222),
			exp: &pb.TypedValue{Value: &pb.TypedValue_DoubleVal{DoubleVal: 42.234124222222}},
		},
		"bool": {
			in:  true,
			exp: &pb.TypedValue{Value: &pb.TypedValue_BoolVal{BoolVal: true}},
		},
		"slice": {
			in: []interface{}{"foo", 1, uint(2), true},
			exp: &pb.TypedValue{Value: &pb.TypedValue_LeaflistVal{LeaflistVal: &pb.ScalarArray{
				Element: []*pb.TypedValue{
					&pb.TypedValue{Value: &pb.TypedValue_StringVal{StringVal: "foo"}},
					&pb.TypedValue{Value: &pb.TypedValue_IntVal{IntVal: 1}},
					&pb.TypedValue{Value: &pb.TypedValue_UintVal{UintVal: 2}},
					&pb.TypedValue{Value: &pb.TypedValue_BoolVal{BoolVal: true}},
				}}}},
		},
		"bytes": {
			in:  []byte("foo"),
			exp: &pb.TypedValue{Value: &pb.TypedValue_BytesVal{BytesVal: []byte("foo")}},
		},
		"typed val": {
			in:  &pb.TypedValue{Value: &pb.TypedValue_StringVal{StringVal: "foo"}},
			exp: &pb.TypedValue{Value: &pb.TypedValue_StringVal{StringVal: "foo"}},
		},
	} {
		t.Run(tname, func(t *testing.T) {
			if got := TypedValue(tcase.in); !test.DeepEqual(got, tcase.exp) {
				t.Errorf("Expected: %q Got: %q", tcase.exp, got)
			}
		})
	}
}

func TestExtractJSON(t *testing.T) {
	jsonFile, err := ioutil.TempFile("", "extractContent")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(jsonFile.Name())
	if _, err := jsonFile.Write([]byte(`"jsonFile"`)); err != nil {
		jsonFile.Close()
		t.Fatal(err)
	}
	if err := jsonFile.Close(); err != nil {
		t.Fatal(err)
	}

	for val, exp := range map[string][]byte{
		jsonFile.Name(): []byte(`"jsonFile"`),
		"foobar":        []byte(`"foobar"`),
		`"foobar"`:      []byte(`"foobar"`),
		"Val: true":     []byte(`"Val: true"`),
		"host42":        []byte(`"host42"`),
		"42":            []byte("42"),
		"-123.43":       []byte("-123.43"),
		"0xFFFF":        []byte("0xFFFF"),
		// Int larger than can fit in 32 bits should be quoted
		"0x8000000000":  []byte(`"0x8000000000"`),
		"-0x8000000000": []byte(`"-0x8000000000"`),
		"true":          []byte("true"),
		"false":         []byte("false"),
		"null":          []byte("null"),
		"{true: 42}":    []byte("{true: 42}"),
		"[]":            []byte("[]"),
	} {
		t.Run(val, func(t *testing.T) {
			got := extractContent(val, "")
			if !bytes.Equal(exp, got) {
				t.Errorf("Unexpected diff. Expected: %q Got: %q", exp, got)
			}
		})
	}
}

func TestExtractValue(t *testing.T) {
	cases := []struct {
		in  *pb.Update
		exp interface{}
	}{{
		in: &pb.Update{Val: &pb.TypedValue{
			Value: &pb.TypedValue_StringVal{StringVal: "foo"}}},
		exp: "foo",
	}, {
		in: &pb.Update{Val: &pb.TypedValue{
			Value: &pb.TypedValue_IntVal{IntVal: 123}}},
		exp: int64(123),
	}, {
		in: &pb.Update{Val: &pb.TypedValue{
			Value: &pb.TypedValue_UintVal{UintVal: 123}}},
		exp: uint64(123),
	}, {
		in: &pb.Update{Val: &pb.TypedValue{
			Value: &pb.TypedValue_BoolVal{BoolVal: true}}},
		exp: true,
	}, {
		in: &pb.Update{Val: &pb.TypedValue{
			Value: &pb.TypedValue_BytesVal{BytesVal: []byte{0xde, 0xad}}}},
		exp: []byte{0xde, 0xad},
	}, {
		in: &pb.Update{Val: &pb.TypedValue{
			Value: &pb.TypedValue_FloatVal{FloatVal: -12.34}}},
		exp: float32(-12.34),
	}, {
		in: &pb.Update{Val: &pb.TypedValue{
			Value: &pb.TypedValue_DoubleVal{DoubleVal: -12.34}}},
		exp: float64(-12.34),
	}, {
		in: &pb.Update{Val: &pb.TypedValue{
			Value: &pb.TypedValue_DecimalVal{DecimalVal: &pb.Decimal64{
				Digits: -1234, Precision: 2}}}},
		exp: &pb.Decimal64{Digits: -1234, Precision: 2},
	}, {
		in: &pb.Update{Val: &pb.TypedValue{
			Value: &pb.TypedValue_LeaflistVal{LeaflistVal: &pb.ScalarArray{
				Element: []*pb.TypedValue{
					&pb.TypedValue{Value: &pb.TypedValue_StringVal{StringVal: "foo"}},
					&pb.TypedValue{Value: &pb.TypedValue_IntVal{IntVal: 123}}}}}}},
		exp: []interface{}{"foo", int64(123)},
	}, {
		in: &pb.Update{Val: &pb.TypedValue{
			Value: &pb.TypedValue_JsonVal{JsonVal: []byte(`12.34`)}}},
		exp: json.Number("12.34"),
	}, {
		in: &pb.Update{Val: &pb.TypedValue{
			Value: &pb.TypedValue_JsonVal{JsonVal: []byte(`[12.34, 123, "foo"]`)}}},
		exp: []interface{}{json.Number("12.34"), json.Number("123"), "foo"},
	}, {
		in: &pb.Update{Val: &pb.TypedValue{
			Value: &pb.TypedValue_JsonVal{JsonVal: []byte(`{"foo":"bar"}`)}}},
		exp: map[string]interface{}{"foo": "bar"},
	}, {
		in: &pb.Update{Val: &pb.TypedValue{
			Value: &pb.TypedValue_JsonVal{JsonVal: []byte(`{"foo":45.67}`)}}},
		exp: map[string]interface{}{"foo": json.Number("45.67")},
	}, {
		in: &pb.Update{Val: &pb.TypedValue{
			Value: &pb.TypedValue_JsonIetfVal{JsonIetfVal: []byte(`{"foo":"bar"}`)}}},
		exp: map[string]interface{}{"foo": "bar"},
	}}
	for _, tc := range cases {
		out, err := ExtractValue(tc.in)
		if err != nil {
			t.Errorf(err.Error())
		}
		if !test.DeepEqual(tc.exp, out) {
			t.Errorf("Extracted value is incorrect. Expected %+v, got %+v", tc.exp, out)
		}
	}
}
