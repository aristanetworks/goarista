// Copyright (c) 2019 Arista Networks, Inc.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the COPYING file.

package gnmi

import (
	"fmt"
	"testing"

	"github.com/aristanetworks/goarista/test"

	"github.com/openconfig/gnmi/proto/gnmi_ext"
)

func arbitration(role string, id *gnmi_ext.Uint128) *gnmi_ext.Extension {
	arb := &gnmi_ext.MasterArbitration{
		Role:       &gnmi_ext.Role{Id: role},
		ElectionId: id,
	}
	ext := gnmi_ext.Extension_MasterArbitration{MasterArbitration: arb}
	return &gnmi_ext.Extension{Ext: &ext}
}

func electionID(high, low uint64) *gnmi_ext.Uint128 {
	return &gnmi_ext.Uint128{High: high, Low: low}
}

func TestArbitrationExt(t *testing.T) {
	testCases := map[string]struct {
		s   string
		ext *gnmi_ext.Extension
		err error
	}{
		"empty": {},
		"no_role": {
			s:   "1",
			ext: arbitration("", electionID(0, 1)),
		},
		"with_role": {
			s:   "admin:1",
			ext: arbitration("admin", electionID(0, 1)),
		},
		"large_no_role": {
			s:   "9223372036854775807",
			ext: arbitration("", electionID(0, 9223372036854775807)),
		},
		"large_with_role": {
			s:   "admin:18446744073709551615",
			ext: arbitration("admin", electionID(0, 18446744073709551615)),
		},
		"invalid": {
			s:   "cat",
			err: fmt.Errorf("badly formed arbitration id (%s)", "cat"),
		},
		"invalid_too_many_colons": {
			s:   "dog:1:2",
			err: fmt.Errorf("badly formed arbitration id (%s)", "dog:1:2"),
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			ext, err := ArbitrationExt(tc.s)
			if !test.DeepEqual(tc.ext, ext) {
				t.Errorf("Expected %#v, got %#v", tc.ext, ext)
			}
			if !test.DeepEqual(tc.err, err) {
				t.Errorf("Expected %v, got %v", tc.err, err)
			}
		})
	}
}
