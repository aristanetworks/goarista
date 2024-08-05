// Copyright (c) 2021 Arista Networks, Inc.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the COPYING file.

package glog

import (
	"bytes"
	"io"

	"github.com/aristanetworks/glog"
)

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

// SuppressLines adds filtering to glog output so that all lines containing
// any of the supplied substrings are removed. It returns a function that
// reverts the glog output writer to the previous one (without filtering).
// A typical use case is test functions where certain warning or error messages
// are expected, so they only add noise to the output of `go test`.
//
// Example usage:
//
//	import aglog "github.com/aristanetworks/goarista/glog"
//	func TestExampleFunction(t *testing.T) {
//		reset := aglog.SuppressLines(
//			`Warning: non-ASCII value in test A`,
//			`Test B is failing with exit code 0`,
//			`Test C exceeds timeout of 60 seconds`,
//		)
//		defer reset()
//		...
//	}
func SuppressLines(substrToSuppress ...string) func() {
	bytesToSuppress := make([][]byte, len(substrToSuppress))
	for i, substr := range substrToSuppress {
		bytesToSuppress[i] = []byte(substr)
	}
	fw := &filterWriter{bytesToSuppress: bytesToSuppress}
	prev := glog.SetOutput(fw)
	fw.writer = prev
	return func() {
		fw.flush()
		glog.SetOutput(prev)
	}
}

type filterWriter struct {
	writer          io.Writer
	buffer          bytes.Buffer
	bytesToSuppress [][]byte
	err             error
}

func (fw *filterWriter) Write(data []byte) (n int, err error) {
	if fw.err != nil {
		return 0, fw.err
	}

	fw.buffer.Write(data)
	for bytes.IndexByte(fw.buffer.Bytes(), '\n') != -1 {
		byteLine, readErr := fw.buffer.ReadBytes('\n')
		if readErr != nil {
			break
		}
		skipLine := false
		for _, subslice := range fw.bytesToSuppress {
			if bytes.Contains(byteLine, subslice) {
				skipLine = true
				break
			}
		}
		if !skipLine {
			_, err = fw.writer.Write(byteLine)
			if err != nil {
				fw.err = err
				return len(data), err
			}
		}
	}
	return len(data), nil
}

func (fw *filterWriter) flush() {
	if fw.err != nil {
		return
	}

	if fw.buffer.Available() > 0 {
		fw.buffer.WriteTo(fw.writer)
	}
}
