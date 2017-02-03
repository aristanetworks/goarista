// Copyright (C) 2017  Arista Networks, Inc.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the COPYING file.

package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/aristanetworks/goarista/gnmi"
	gnmipb "github.com/openconfig/reference/rpc/gnmi"
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
	var cfg gnmi.Config
	flag.StringVar(&cfg.Addr, "addr", "", "Address of gNMI gRPC server")
	flag.StringVar(&cfg.CAFile, "cafile", "", "Path to server TLS certificate file")
	flag.StringVar(&cfg.CertFile, "certfile", "", "Path to client TLS certificate file")
	flag.StringVar(&cfg.KeyFile, "keyfile", "", "Path to client TLS private key file")
	flag.StringVar(&cfg.Password, "password", "", "Password to authenticate with")
	flag.StringVar(&cfg.Username, "username", "", "Username to authenticate with")
	flag.BoolVar(&cfg.TLS, "tls", false, "Enable TLS")

	flag.Usage = func() {
		fmt.Fprintln(os.Stderr, help)
		flag.PrintDefaults()
	}
	flag.Parse()
	args := flag.Args()

	ctx := gnmi.NewContext(context.Background(), cfg)
	client := gnmi.Dial(cfg)

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
			err := get(ctx, client, gnmi.SplitPaths(args[i+1:]))
			if err != nil {
				log.Fatal(err)
			}
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

func get(ctx context.Context, gnmiClient gnmipb.GNMIClient, paths [][]string) error {
	req := gnmi.NewGetRequest(paths)
	resp, err := gnmiClient.Get(ctx, req)
	if err != nil {
		return err
	}
	for _, notif := range resp.Notification {
		for _, update := range notif.Update {
			fmt.Printf("%s:\n", gnmi.JoinPath(update.Path.Element))
			fmt.Println(string(update.Value.Value))
		}
	}
	return nil
}
