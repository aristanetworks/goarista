// Copyright (c) 2020 Arista Networks, Inc.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the COPYING file.

package main

import (
	"testing"
	"time"

	"github.com/golang/protobuf/proto"
	"github.com/openconfig/gnmi/proto/gnmi"
)

func TestSampleList(t *testing.T) {
	for name, tc := range map[string]struct {
		arg string

		error    bool
		path     *gnmi.Path
		interval time.Duration
	}{
		"working": {
			arg: "/foos/foo[name=bar]/baz@30s",

			path: &gnmi.Path{Elem: []*gnmi.PathElem{
				&gnmi.PathElem{Name: "foos"},
				&gnmi.PathElem{Name: "foo",
					Key: map[string]string{"name": "bar"}},
				&gnmi.PathElem{Name: "baz"},
			}},
			interval: 30 * time.Second,
		},
		"no_interval": {
			arg:   "/foos/foo[name=bar]/baz",
			error: true,
		},
		"empty_interval": {
			arg:   "/foos/foo[name=bar]/baz@",
			error: true,
		},
		"invalid_path": {
			arg:   "/foos/foo[name=bar]]/baz@30s",
			error: true,
		},
	} {
		t.Run(name, func(t *testing.T) {
			var l sampleList
			err := l.Set(tc.arg)
			if err != nil {
				if !tc.error {
					t.Fatalf("unexpected error: %s", err)
				}
				return
			} else if tc.error {
				t.Fatal("expected error and didn't get one")
			}

			sub := l.subs[0]
			sub.p.Element = nil // Ignore the backward compatible path
			if !proto.Equal(tc.path, sub.p) {
				t.Errorf("Paths don't match. Expected: %s Got: %s",
					tc.path, sub.p)
			}
			if tc.interval != sub.interval {
				t.Errorf("Intervals don't match. Expected %s Got: %s",
					tc.interval, sub.interval)
			}
			str := l.String()
			if tc.arg != str {
				t.Errorf("Unexpected String() result: Expected: %q Got: %q", tc.arg, str)
			}
		})
	}
}
