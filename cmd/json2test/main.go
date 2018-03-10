// Copyright (c) 2018 Arista Networks, Inc.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the COPYING file.

// json2test reformats 'go test -json' output as text as if the -json
// flag were not passed to go test. It is useful if you want to
// analyze go test -json output, but still want a human readable test
// log.
//
// Usage:
//
//  go test -json > out.txt; <analysis program> out.txt; cat out.txt | json2test
//
package main

import (
	"encoding/json"
	"io"
	"os"
	"time"
)

type testEvent struct {
	Time    time.Time // encodes as an RFC3339-format string
	Action  string
	Package string
	Test    string
	Elapsed float64 // seconds
	Output  string
}

func main() {
	writeTestOutput(os.Stdin, os.Stdout)
}

func writeTestOutput(in io.Reader, out io.Writer) error {
	d := json.NewDecoder(in)
	for {
		var e testEvent
		if err := d.Decode(&e); err != nil {
			break
		}
		switch e.Action {
		default:
			continue
		case "output":
		}

		if _, err := io.WriteString(out, e.Output); err != nil {
			return err
		}
	}
	return nil
}
