// Copyright (c) 2022 Arista Networks, Inc.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the COPYING file.

package monitor

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/aristanetworks/glog"
)

func setGlogV(v string) error {
	// SetVGlobal silently errors Atoi, lets return that instead.
	_, err := strconv.Atoi(v)
	if err != nil {
		return fmt.Errorf("setGlogV: invalid int: %q", v)
	}
	glog.SetVGlobal(v)
	glog.Infof("monitor: set glog verbosity to %v", v)
	return nil
}

func setLogVerbosity(w http.ResponseWriter, r *http.Request) {
	usage := fmt.Sprintf("\nusage: curl -XPOST %s?glog=<glog verbosity>", r.URL.Path)
	if r.Method != "POST" {
		http.Error(w, "only supports POST method"+usage, http.StatusBadRequest)
		return
	}

	opts := r.URL.Query()
	didUpdate := false

	// set glog verbosity
	gv := opts.Get("glog")
	if gv != "" {
		if err := setGlogV(gv); err != nil {
			http.Error(w, err.Error()+usage, http.StatusBadRequest)
			return
		}
		didUpdate = true
	}

	if !didUpdate {
		http.Error(w, "bad request: no update"+usage, http.StatusBadRequest)
		return
	}

	fmt.Fprint(w, "OK\n")
}
