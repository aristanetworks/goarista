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
	n, err := strconv.Atoi(v)
	if err != nil {
		return fmt.Errorf("setGlogV: invalid int: %q", v)
	}
	glog.SetVGlobal(glog.Level(n))
	glog.Infof("monitor: set glog verbosity to %v", v)
	return nil
}

func logErr(w http.ResponseWriter, err string, code int) {
	err = fmt.Sprintf("loglevel error: %v (code %v)", err, code)
	glog.Error(err)
	http.Error(w, err, code)
}

func setLogVerbosity(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		logErr(w, "only supports POST method", http.StatusBadRequest)
		return
	}

	if err := r.ParseForm(); err != nil {
		logErr(w, "could not parse form: "+err.Error(), http.StatusBadRequest)
		return

	}
	opts := r.Form
	didUpdate := false

	// set glog verbosity
	gv := opts.Get("glog")
	if gv != "" {
		if err := setGlogV(gv); err != nil {
			logErr(w, "could not set glog: "+err.Error(), http.StatusBadRequest)
			return
		}
		didUpdate = true
	}

	if !didUpdate {
		logErr(w, "bad request: no change", http.StatusBadRequest)
		return
	}

	fmt.Fprint(w, "OK\n")
}
