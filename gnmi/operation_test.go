// Copyright (c) 2017 Arista Networks, Inc.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the COPYING file.

package gnmi

import (
	"testing"

	"github.com/aristanetworks/goarista/test"

	pb "github.com/openconfig/gnmi/proto/gnmi"
)

func TestNewSetRequest(t *testing.T) {
	pathFoo := &pb.Path{
		Element: []string{"foo"},
		Elem:    []*pb.PathElem{&pb.PathElem{Name: "foo"}},
	}
	pathCli := &pb.Path{
		Origin: "cli",
	}

	testCases := map[string]struct {
		setOps []*Operation
		exp    pb.SetRequest
	}{
		"delete": {
			setOps: []*Operation{&Operation{Type: "delete", Path: []string{"foo"}}},
			exp:    pb.SetRequest{Delete: []*pb.Path{pathFoo}},
		},
		"update": {
			setOps: []*Operation{&Operation{Type: "update", Path: []string{"foo"}, Val: "true"}},
			exp: pb.SetRequest{
				Update: []*pb.Update{&pb.Update{
					Path: pathFoo,
					Val: &pb.TypedValue{
						Value: &pb.TypedValue_JsonIetfVal{JsonIetfVal: []byte("true")}},
				}},
			},
		},
		"replace": {
			setOps: []*Operation{&Operation{Type: "replace", Path: []string{"foo"}, Val: "true"}},
			exp: pb.SetRequest{
				Replace: []*pb.Update{&pb.Update{
					Path: pathFoo,
					Val: &pb.TypedValue{
						Value: &pb.TypedValue_JsonIetfVal{JsonIetfVal: []byte("true")}},
				}},
			},
		},
		"cli-replace": {
			setOps: []*Operation{&Operation{Type: "replace", Path: []string{"cli"},
				Val: "hostname foo\nip routing"}},
			exp: pb.SetRequest{
				Replace: []*pb.Update{&pb.Update{
					Path: pathCli,
					Val: &pb.TypedValue{
						Value: &pb.TypedValue_AsciiVal{AsciiVal: "hostname foo\nip routing"}},
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
			if diff := test.Diff(tc.exp, *got); diff != "" {
				t.Errorf("unexpected diff: %s", diff)
			}
		})
	}
}
