// Copyright (C) 2017  Arista Networks, Inc.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the COPYING file.

package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/aristanetworks/goarista/gnmi"
)

// TODO: Make this more clear
var help = `Usage of gnmicli:
gnmicli [options]
  capabilities
  get PATH+
  subscribe PATH+
  ((update|replace PATH JSON)|(delete PATH))+
`

func exitWithError(s string) {
	flag.Usage()
	fmt.Fprintln(os.Stderr, s)
	os.Exit(1)
}

type operation struct {
	opType string
	path   []string
	val    string
}

func main() {
	flag.Usage = func() {
		fmt.Fprintln(os.Stderr, help)
		flag.PrintDefaults()
	}
	flag.Parse()
	args := flag.Args()

	var setOps []*operation
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "capabilities":
			if len(setOps) != 0 {
				exitWithError("error: 'capabilities' not allowed after 'merge|replace|delete'")
			}
			exitWithError("error: 'capabilities' not supported")
			return
		case "get":
			if len(setOps) != 0 {
				exitWithError("error: 'get' not allowed after 'merge|replace|delete'")
			}
			exitWithError("error: 'get' not supported")
			return
		case "subscribe":
			if len(setOps) != 0 {
				exitWithError("error: 'subscribe' not allowed after 'merge|replace|delete'")
			}
			exitWithError("error: 'subscribe' not supported")
			return
		case "update", "replace", "delete":
			op := &operation{
				opType: args[i],
			}
			if len(args) == i+1 {
				exitWithError("error: missing path")
			}
			i++
			op.path = gnmi.SplitPath(args[i])
			if op.opType != "delete" {
				if len(args) == i+1 {
					exitWithError("error: missing JSON")
				}
				i++
				op.val = args[i]
			}
			setOps = append(setOps, op)
			exitWithError(fmt.Sprintf("error: '%s' not supported", op.opType))
		default:
			exitWithError(fmt.Sprintf("error: unknown operation %q", args[i]))
		}
	}
	_ = setOps
}
