// Copyright (c) 2021 Arista Networks, Inc.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the COPYING file.

package logger

// Logger is an interface to pass a generic logger without depending on either golang/glog or
// aristanetworks/glog
type Logger interface {
	// Info logs at the info level
	Info(args ...interface{})
	// Infof logs at the info level, with format
	Infof(format string, args ...interface{})
	// Error logs at the error level
	Error(args ...interface{})
	// Errorf logs at the error level, with format
	Errorf(format string, args ...interface{})
	// Fatal logs at the fatal level
	Fatal(args ...interface{})
	// Fatalf logs at the fatal level, with format
	Fatalf(format string, args ...interface{})
}
