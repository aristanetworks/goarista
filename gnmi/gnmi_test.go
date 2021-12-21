// Copyright (c) 2021 Arista Networks, Inc.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the COPYING file.

package gnmi

import (
	"bytes"
	"os/exec"
	"testing"
)

func TestDependencies(t *testing.T) {
	cmd := exec.Command("go", "list", "-deps")
	out, err := cmd.Output()
	if err != nil {
		t.Fatal(err)
	}
	// Depending on the github.com/aristanetworks/glog is forbidden
	// because this package is often used with
	// github.com/openconfig/... packages which depend on
	// github.com/golang/glog. These two glog packages cannot be used
	// together in one binary because they try to register the same
	// flags.
	if bytes.Contains(out, []byte("github.com/aristanetworks/glog")) {
		t.Error("gnmi depends on github.com/aristanetworks/glog")
	}
}
