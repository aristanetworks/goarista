// Copyright (c) 2025 Arista Networks, Inc.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the COPYING file.

package gnmi

import (
	"fmt"
	"testing"

	"github.com/aristanetworks/goarista/test"
)

func TestParseAddress(t *testing.T) {
	testCases := []struct {
		addrIn  string
		network string
		nsName  string
		addr    string
		err     error
	}{{
		addrIn:  "mgmt/127.0.0.1:6030",
		network: "tcp",
		nsName:  "ns-mgmt",
		addr:    "127.0.0.1:6030",
	}, {
		addrIn:  "[::1]:9339",
		network: "tcp",
		addr:    "[::1]:9339",
	}, {
		addrIn:  "unix:///var/run/gnmiServer.sock",
		network: "unix",
		addr:    "/var/run/gnmiServer.sock",
	}, {
		addrIn:  "http://example:80",
		network: "http",
		addr:    "example:80",
	}, {
		addrIn: "invalid/invalid/invalid",
		err: fmt.Errorf(
			"Could not parse out a <vrf-name>/address for invalid/invalid/invalid:6030"),
	}}
	for _, tc := range testCases {
		network, nsName, addr, err := ParseAddress(tc.addrIn)
		if tc.network != network {
			t.Errorf("network: want %q, got %q", tc.network, network)
		}
		if tc.nsName != nsName {
			t.Errorf("nsName: want %q, got %q", tc.nsName, nsName)
		}
		if tc.addr != addr {
			t.Errorf("addr: want %q, got %q", tc.addr, addr)
		}
		if !test.DeepEqual(tc.err, err) {
			t.Errorf("err: want %q, got %q", tc.err, err)
		}
	}
}
