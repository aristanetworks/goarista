// Copyright (c) 2021 Arista Networks, Inc.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the COPYING file.

package glog

import "github.com/aristanetworks/glog"

// Glog is an empty type that allows to pass glog as a logger, implementing logger.Logger
type Glog struct {
	// default value of glog.Level is 0
	InfoLevel glog.Level
}

// Info logs at the info level
func (g *Glog) Info(args ...interface{}) {
	glog.V(g.InfoLevel).Info(args...)
}

// Infof logs at the info level, with format
func (g *Glog) Infof(format string, args ...interface{}) {
	glog.V(g.InfoLevel).Infof(format, args...)
}

// Error logs at the error level
func (g *Glog) Error(args ...interface{}) {
	glog.Error(args...)
}

// Errorf logs at the error level, with format
func (g *Glog) Errorf(format string, args ...interface{}) {
	glog.Errorf(format, args...)
}

// Fatal logs at the fatal level
func (g *Glog) Fatal(args ...interface{}) {
	glog.Fatal(args...)
}

// Fatalf logs at the fatal level, with format
func (g *Glog) Fatalf(format string, args ...interface{}) {
	glog.Fatalf(format, args...)
}
