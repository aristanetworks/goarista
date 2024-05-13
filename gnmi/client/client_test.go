// Copyright (c) 2023 Arista Networks, Inc.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the COPYING file.

package client

import (
	"testing"

	"github.com/aristanetworks/goarista/gnmi"
	"github.com/aristanetworks/goarista/test"
	pb "github.com/openconfig/gnmi/proto/gnmi"
)

func TestNewGetRequest(t *testing.T) {
	testCases := map[string]struct {
		pathParam *reqParams
		exp       *pb.GetRequest
	}{
		"ascii-cli": {
			pathParam: &reqParams{
				encoding: "ascii",
				origin:   "cli",
				paths:    []string{"show version"},
			},
			exp: &pb.GetRequest{
				Encoding: pb.Encoding_ASCII,
				Type:     pb.GetRequest_ALL,
				Path: []*pb.Path{{
					Origin:  "cli",
					Element: []string{"show version"},
					Elem: []*pb.PathElem{{
						Name: "show version",
					}},
				},
				},
			},
		},
		"default-cli": {
			pathParam: &reqParams{
				origin: "cli",
				paths:  []string{"show version"},
			},
			exp: &pb.GetRequest{
				Encoding: pb.Encoding_JSON,
				Type:     pb.GetRequest_ALL,
				Path: []*pb.Path{{
					Origin:  "cli",
					Element: []string{"show version"},
					Elem: []*pb.PathElem{{
						Name: "show version",
					}},
				},
				},
			},
		},
		"default-non-cli": {
			pathParam: &reqParams{paths: []string{"show version"}},
			exp: &pb.GetRequest{
				Encoding: pb.Encoding_JSON,
				Type:     pb.GetRequest_ALL,
				Path: []*pb.Path{{
					Element: []string{"show version"},
					Elem: []*pb.PathElem{{
						Name: "show version",
					}},
				},
				},
			},
		},
		"multiple-paths": {
			pathParam: &reqParams{
				encoding: "ascii",
				origin:   "cli",
				paths:    []string{"show version", "show running-config", "show history"},
			},
			exp: &pb.GetRequest{
				Encoding: pb.Encoding_ASCII,
				Type:     pb.GetRequest_ALL,
				Path: []*pb.Path{
					{
						Origin:  "cli",
						Element: []string{"show version"},
						Elem: []*pb.PathElem{{
							Name: "show version",
						}},
					},
					{
						Origin:  "cli",
						Element: []string{"show running-config"},
						Elem: []*pb.PathElem{{
							Name: "show running-config",
						}},
					},
					{
						Origin:  "cli",
						Element: []string{"show history"},
						Elem: []*pb.PathElem{{
							Name: "show history",
						}},
					}},
			},
		},
	}

	for name, tc := range testCases {
		got, err := newGetRequest(*tc.pathParam, "all")
		if err != nil {
			t.Fatalf("ERROR!\n%s: got error: %s, but expect no error\n", name, err.Error())
		}
		if !test.DeepEqual(got, tc.exp) {
			t.Fatalf("ERROR!\nTest Case: %s\nGot: %s,\nWant %s\n", name, got, tc.exp)
		}
	}
}

func TestNewSubscribeOptions(t *testing.T) {
	testCases := map[string]struct {
		pathParam *reqParams
		exp       *gnmi.SubscribeOptions
	}{
		"core": {
			pathParam: &reqParams{
				target: "target",
				origin: "cli",
				paths:  []string{"show version"},
			},
			exp: &gnmi.SubscribeOptions{
				Paths:  gnmi.SplitPaths([]string{"show version"}),
				Origin: "cli",
				Target: "target",
			},
		},
		"multi-paths": {
			pathParam: &reqParams{
				target: "target",
				origin: "cli",
				paths:  []string{"show version", "show running-config", "show history"},
			},
			exp: &gnmi.SubscribeOptions{
				Paths: gnmi.SplitPaths(
					[]string{"show version", "show running-config", "show history"},
				),
				Target: "target",
				Origin: "cli",
			},
		},
	}

	for name, tc := range testCases {
		got, err := newSubscribeOptions(*tc.pathParam, nil, new(gnmi.SubscribeOptions))
		if err != nil {
			t.Fatalf("ERROR!\n%s: got error: %s, but expect no error\n", name, err.Error())
		}
		if !test.DeepEqual(got, tc.exp) {
			t.Fatalf("ERROR!\nTest Case: %s\nGot: %+v,\nWant %+v\n", name, got, tc.exp)
		}
	}
}

// subcribe request does not support encoding
// test that it throws an error if encoding is given
func TestEncodingSubscribeOptions(t *testing.T) {
	testCases := map[string]struct {
		pathParam *reqParams
	}{
		"ASCII": {
			&reqParams{
				encoding: "ASCII",
				origin:   "cli",
				paths:    []string{"show version"},
			},
		},
		"bytes": {
			&reqParams{
				encoding: "bytes",
				origin:   "cli",
				target:   "target",
				paths:    []string{"show version"},
			},
		},
		"json": {
			&reqParams{
				encoding: "json",
				paths:    []string{"show version"},
			},
		},
		"json_ietf": {
			&reqParams{
				encoding: "json_ietf",
				origin:   "cli",
				paths:    []string{"show version"},
			},
		},
		"proto": {
			&reqParams{
				encoding: "proto",
				origin:   "OpenConfig",
				target:   "whatever",
				paths:    []string{"show version"},
			},
		},
		"dot": {
			&reqParams{
				encoding: ".",
				paths:    []string{"show version"},
			},
		},
	}

	for name, tc := range testCases {
		_, err := newSubscribeOptions(*tc.pathParam, nil, nil)
		if err == nil {
			t.Fatalf("ERROR!\n%s: got no error, but expect an error\n", name)
		}
	}
}

func TestNewSetOperations(t *testing.T) {
	testCases := map[string]struct {
		args []string
		exp  *gnmi.Operation
	}{
		"update": {
			args: []string{"update", "origin=cli", "target=target", "path", "100"},
			exp: &gnmi.Operation{
				Type:   "update",
				Origin: "cli",
				Target: "target",
				Val:    "100",
				Path:   gnmi.SplitPath("path"),
			},
		},
		"replace": {
			args: []string{"replace", "origin=cli", "target=target", "path", "100"},
			exp: &gnmi.Operation{
				Type:   "replace",
				Origin: "cli",
				Target: "target",
				Val:    "100",
				Path:   gnmi.SplitPath("path"),
			},
		},
		"delete": {
			args: []string{"delete", "origin=cli", "target=target", "path"},
			exp: &gnmi.Operation{
				Type:   "delete",
				Origin: "cli",
				Target: "target",
				Path:   gnmi.SplitPath("path"),
			},
		},
		"union_replace": {
			args: []string{"union_replace", "origin=cli", "target=target", "path", "100"},
			exp: &gnmi.Operation{
				Type:   "union_replace",
				Origin: "cli",
				Target: "target",
				Val:    "100",
				Path:   gnmi.SplitPath("path"),
			},
		},
	}

	for name, tc := range testCases {
		_, got, err := newSetOperation(0, tc.args, "")
		if err != nil {
			t.Fatalf("ERROR!\n%s: got error: %s, but expect no error\n", name, err.Error())
		}
		if !test.DeepEqual(got, tc.exp) {
			t.Fatalf("ERROR!\nTest Case: %s\nGot: %+v,\nWant %+v\n", name, got, tc.exp)
		}
	}
}

// update|replace|delete|union_replace request does not support encoding
// test that it throws an error if encoding is given
func TestEncodingNewSetOperations(t *testing.T) {
	testCases := map[string]struct {
		args []string
	}{
		// update
		"dot_update":       {[]string{"update", "encoding=.", "/", "val"}},
		"ASCII_update":     {[]string{"update", "encoding=ascii", "origin=cli", "/", "val"}},
		"bytes_update":     {[]string{"update", "encoding=bytes", "/", "val"}},
		"json_update":      {[]string{"update", "encoding=json", "target=what", "/", "val"}},
		"json_ieft_update": {[]string{"update", "encoding=json_ietf", "/", "val"}},
		"proto_update": {
			[]string{
				"update",
				"encoding=proto",
				"target=h",
				"origin=a",
				"/",
				"val"},
		},

		// replace
		"dot_replace":       {[]string{"replace", "encoding=.", "/", "val"}},
		"ASCII_replace":     {[]string{"replace", "encoding=ascii", "origin=cli", "/", "val"}},
		"bytes_replace":     {[]string{"replace", "encoding=bytes", "/", "val"}},
		"json_replace":      {[]string{"replace", "encoding=json", "target=what", "/", "val"}},
		"json_ieft_replace": {[]string{"replace", "encoding=json_ietf", "/", "val"}},
		"proto_replace": {
			[]string{
				"replace",
				"encoding=proto",
				"target=h",
				"origin=a",
				"/",
				"val"},
		},

		// delete
		"dot_delete":       {[]string{"delete", "encoding=.", "/"}},
		"ASCII_delete":     {[]string{"delete", "encoding=ascii", "origin=cli", "/"}},
		"bytes_delete":     {[]string{"delete", "encoding=bytes", "/"}},
		"json_delete":      {[]string{"delete", "encoding=json", "target=what", "/"}},
		"json_ieft_delete": {[]string{"delete", "encoding=json_ietf", "/"}},
		"proto_delete": {
			[]string{
				"delete",
				"encoding=proto",
				"target=h",
				"origin=a",
				"/"}},

		// union_replace
		"dot_union_replace": {[]string{"union_replace", "encoding=.", "/", "val"}},
		"ASCII_union_replace": {
			[]string{"union_replace", "encoding=ascii", "origin=cli", "/", "val"}},
		"bytes_union_replace": {[]string{"union_replace", "encoding=bytes", "/", "val"}},
		"json_union_replace": {
			[]string{"union_replace", "encoding=json", "target=what", "/", "val"}},
		"json_ieft_union_replace": {[]string{"union_replace", "encoding=json_ietf", "/", "val"}},
		"proto_union_replace": {
			[]string{
				"union_replace",
				"encoding=proto",
				"target=h",
				"origin=a",
				"/",
				"val"},
		},
	}
	for name, tc := range testCases {
		_, _, err := newSetOperation(0, tc.args, "")
		if err == nil {
			t.Fatalf("ERROR!\n%s: got no error, but expect an error\n", name)
		}
	}
}

// update|replace|union_replace operation needs a value
// test that it throws an error if missing
func TestMissingValueNewSetOperations(t *testing.T) {
	testCases := map[string]struct {
		args []string
	}{
		// update
		"update":        {[]string{"update", "/"}},
		"update_origin": {[]string{"update", "origin=cli", "/"}},
		"update_target": {[]string{"update", "target=what", "/"}},
		"update_both":   {[]string{"update", "target=h", "origin=a", "/"}},

		// replace
		"replace":        {[]string{"replace", "/"}},
		"replace_origin": {[]string{"replace", "origin=cli", "/"}},
		"replace_target": {[]string{"replace", "target=what", "/"}},
		"replace_both":   {[]string{"replace", "target=h", "origin=a", "/"}},

		// union_replace
		"union_replace":        {[]string{"union_replace", "/"}},
		"union_replace_origin": {[]string{"union_replace", "origin=cli", "/"}},
		"union_replace_target": {[]string{"union_replace", "target=what", "/"}},
		"union_replace_both":   {[]string{"union_replace", "target=h", "origin=a", "/"}},
	}

	for name, tc := range testCases {
		_, _, err := newSetOperation(0, tc.args, "")
		if err == nil {
			t.Fatalf("ERROR!\n%s: got no error, but expect an error\n", name)
		}
	}
}

// update|replace|delete|union_replace needs a path
// test that it throws an error if missing
func TestMissingPathsNewSetOperations(t *testing.T) {
	testCases := map[string]struct {
		args []string
	}{
		// update
		"update":        {[]string{"update"}},
		"update_origin": {[]string{"update", "origin=cli"}},
		"update_target": {[]string{"update", "target=what"}},
		"update_both":   {[]string{"update", "target=h", "origin=a"}},

		// replace
		"replace":        {[]string{"replace"}},
		"replace_origin": {[]string{"replace", "origin=cli"}},
		"replace_target": {[]string{"replace", "target=what"}},
		"replace_both":   {[]string{"replace", "target=h", "origin=a"}},

		// delete
		"delete":        {[]string{"delete"}},
		"delete_origin": {[]string{"delete", "origin=OpenConfig"}},
		"delete_target": {[]string{"delete", "target=target"}},
		"delete_both":   {[]string{"delete", "target=target", "origin=origin"}},

		// union_replace
		"union_replace":        {[]string{"union_replace"}},
		"union_replace_origin": {[]string{"union_replace", "origin=cli"}},
		"union_replace_target": {[]string{"union_replace", "target=what"}},
		"union_replace_both":   {[]string{"union_replace", "target=h", "origin=a"}},
	}

	for name, tc := range testCases {
		_, _, err := newSetOperation(0, tc.args, "")
		if err == nil {
			t.Fatalf("ERROR!\n%s: got no error, but expect an error\n", name)
		}
	}
}

// for update|replace|delete|union_replace
// test that it stops parsing at the correct args position
func TestArgsPosNewSetOperations(t *testing.T) {
	testCases := map[string]struct {
		args []string
	}{
		// update
		"update":        {[]string{"update", "/", "hi", "hi"}},
		"update_origin": {[]string{"update", "origin=cli", "/", "hi", "hi"}},
		"update_target": {[]string{"update", "target=what", "/", "hi", "hi"}},
		"update_both":   {[]string{"update", "target=h", "origin=a", "/", "hi", "hi"}},

		// replace
		"replace":        {[]string{"replace", "/", "hi", "hi"}},
		"replace_origin": {[]string{"replace", "origin=cli", "/", "hi", "hi"}},
		"replace_target": {[]string{"replace", "target=what", "/", "hi", "hi"}},
		"replace_both":   {[]string{"replace", "target=h", "origin=a", "/", "hi", "hi"}},

		// delete
		"delete":        {[]string{"delete", "/", "update"}},
		"delete_origin": {[]string{"delete", "origin=cli", "/", "next operation"}},
		"delete_target": {[]string{"delete", "target=what", "/", "next operation"}},
		"delete_both":   {[]string{"delete", "origin=cli", "target=tar", "/", "next operation"}},

		// union_replace
		"union_replace":        {[]string{"union_replace", "/", "hi", "hi"}},
		"union_replace_origin": {[]string{"union_replace", "origin=cli", "/", "hi", "hi"}},
		"union_replace_target": {[]string{"union_replace", "target=what", "/", "hi", "hi"}},
		"union_replace_both": {
			[]string{"union_replace", "target=h", "origin=a", "/", "hi", "hi"}},
	}
	for name, tc := range testCases {
		pos, _, err := newSetOperation(0, tc.args, "")
		expectedPos := len(tc.args) - 2
		if err != nil {
			t.Fatalf("ERROR!\n%s: got error: %s, but expect no error\n", name, err.Error())
		}
		if pos != expectedPos {
			t.Fatalf("ERROR!\n%s: got pos = %d, but expect pos = %d\n", name, pos, expectedPos)
		}
	}
}
