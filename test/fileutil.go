// Copyright (C) 2015  Arista Networks, Inc.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the COPYING file.

package test

import (
	"io/ioutil"
	"os"
	"testing"
)

// CopyFile copies a file
func CopyFile(t *testing.T, srcPath, dstPath string) {
	src, err := os.Open(srcPath)
	if err != nil {
		t.Fatal(err)
	}
	defer src.Close()
	// This loads the entire file in memory, which is fine for small-ish files.
	input, err := ioutil.ReadAll(src)
	if err != nil {
		t.Fatal(err)
	}
	err = ioutil.WriteFile(dstPath, input, os.FileMode(0600))
	if err != nil {
		t.Fatal(err)
	}
}
