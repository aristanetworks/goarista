// Copyright (c) 2020 Arista Networks, Inc.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the COPYING file.

package modroot

import (
	"os"
	"path/filepath"
)

var modRoot string

// Path returns the returns the module root, as a better alternative to os.Getenv("GOPATH")
func Path() string {
	if modRoot != "" {
		return modRoot
	}
	dir, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	for {
		if fi, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil && !fi.IsDir() {
			modRoot = dir
			return dir
		}
		d := filepath.Dir(dir)
		if d == dir {
			break
		}
		dir = d
	}
	panic("no module root found!")
}
