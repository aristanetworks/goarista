// Copyright (c) 2024 Arista Networks, Inc.  All rights reserved.
// Arista Networks, Inc. Confidential and Proprietary.
// Subject to Arista Networks, Inc.'s, EULA.
// INTERNAL USE ONLY. NOT FOR DISTRIBUTION.

package glog

import (
	"bytes"
	"strings"
	"testing"

	aglog "github.com/aristanetworks/glog"
)

func TestSuppressLines(t *testing.T) {
	b := &bytes.Buffer{}

	aglog.SetOutput(b)

	aglog.Info("Before suppression: not *excluded*")

	reset := SuppressLines("*excluded*", "[excluded]")

	aglog.Warning("initial stuff... *excluded*, not in output")
	aglog.Error("not excluded")
	aglog.Info("multiple lines -- this one is included\nbut this line is [excluded].")

	reset()

	aglog.Info("Lines after reset are not *excluded*.")

	got := strings.Split(b.String(), "\n")

	expected := []string{
		"Before suppression: not *excluded*",
		"not excluded",
		"multiple lines -- this one is included",
		"Lines after reset are not *excluded*.",
		"", // from final newline
	}

	match := func() bool {
		if len(got) != len(expected) {
			t.Logf("Unexpected number of lines; expected=%v, got=%v", len(expected), len(got))
			return false
		}

		r := true
		for i, gline := range got {
			exp := expected[i]
			if !strings.Contains(gline, exp) {
				t.Logf("Mismatch in line %v", i)
				r = false
			}
		}
		return r
	}

	if !match() {
		t.Log("Expected substrings:")
		for i, exp := range expected {
			t.Logf("  [%v] %#v", i, exp)
		}
		t.Log("Got:")
		for i, gline := range got {
			t.Logf("  [%v] %#v", i, gline)
		}
		t.Fail()
	}
}
