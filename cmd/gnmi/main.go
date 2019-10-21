// Copyright (c) 2017 Arista Networks, Inc.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the COPYING file.

package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/aristanetworks/goarista/gnmi"

	"github.com/aristanetworks/glog"
	pb "github.com/openconfig/gnmi/proto/gnmi"
	"github.com/openconfig/gnmi/proto/gnmi_ext"
	"golang.org/x/sync/errgroup"
)

// TODO: Make this more clear
var help = `Usage of gnmi:
gnmi -addr [<VRF-NAME>/]ADDRESS:PORT [options...]
  capabilities
  get (origin=ORIGIN) PATH+
  subscribe (origin=ORIGIN) PATH+
  ((update|replace (origin=ORIGIN) PATH JSON|FILE)|(delete (origin=ORIGIN) PATH))+
`

func usageAndExit(s string) {
	flag.Usage()
	if s != "" {
		fmt.Fprintln(os.Stderr, s)
	}
	os.Exit(1)
}

func main() {
	cfg := &gnmi.Config{}
	flag.StringVar(&cfg.Addr, "addr", "", "Address of gNMI gRPC server with optional VRF name")
	flag.StringVar(&cfg.CAFile, "cafile", "", "Path to server TLS certificate file")
	flag.StringVar(&cfg.CertFile, "certfile", "", "Path to client TLS certificate file")
	flag.StringVar(&cfg.KeyFile, "keyfile", "", "Path to client TLS private key file")
	flag.StringVar(&cfg.Password, "password", "", "Password to authenticate with")
	flag.StringVar(&cfg.Username, "username", "", "Username to authenticate with")
	flag.StringVar(&cfg.Compression, "compression", "gzip", "Compression method. "+
		`Supported options: "" and "gzip"`)
	flag.BoolVar(&cfg.TLS, "tls", false, "Enable TLS")

	subscribeOptions := &gnmi.SubscribeOptions{}
	flag.StringVar(&subscribeOptions.Prefix, "prefix", "", "Subscribe prefix path")
	flag.BoolVar(&subscribeOptions.UpdatesOnly, "updates_only", false,
		"Subscribe to updates only (false | true)")
	flag.StringVar(&subscribeOptions.Mode, "mode", "stream",
		"Subscribe mode (stream | once | poll)")
	flag.StringVar(&subscribeOptions.StreamMode, "stream_mode", "target_defined",
		"Subscribe stream mode, only applies for stream subscriptions "+
			"(target_defined | on_change | sample)")
	sampleIntervalStr := flag.String("sample_interval", "0", "Subscribe sample interval, "+
		"only applies for sample subscriptions (400ms, 2.5s, 1m, etc.)")
	heartbeatIntervalStr := flag.String("heartbeat_interval", "0", "Subscribe heartbeat "+
		"interval, only applies for on-change subscriptions (400ms, 2.5s, 1m, etc.)")
	arbitrationStr := flag.String("arbitration", "", "master arbitration identifier "+
		"([<role_id>:]<election_id>)")

	flag.Usage = func() {
		fmt.Fprintln(os.Stderr, help)
		flag.PrintDefaults()
	}
	flag.Parse()
	if cfg.Addr == "" {
		usageAndExit("error: address not specified")
	}

	var sampleInterval, heartbeatInterval time.Duration
	var err error
	if sampleInterval, err = time.ParseDuration(*sampleIntervalStr); err != nil {
		usageAndExit(fmt.Sprintf("error: sample interval (%s) invalid", *sampleIntervalStr))
	}
	subscribeOptions.SampleInterval = uint64(sampleInterval)
	if heartbeatInterval, err = time.ParseDuration(*heartbeatIntervalStr); err != nil {
		usageAndExit(fmt.Sprintf("error: heartbeat interval (%s) invalid", *heartbeatIntervalStr))
	}
	subscribeOptions.HeartbeatInterval = uint64(heartbeatInterval)

	args := flag.Args()

	ctx := gnmi.NewContext(context.Background(), cfg)
	client, err := gnmi.Dial(cfg)
	if err != nil {
		glog.Fatal(err)
	}

	var setOps []*gnmi.Operation
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "capabilities":
			if len(setOps) != 0 {
				usageAndExit("error: 'capabilities' not allowed after 'merge|replace|delete'")
			}
			err := gnmi.Capabilities(ctx, client)
			if err != nil {
				glog.Fatal(err)
			}
			return
		case "get":
			if len(setOps) != 0 {
				usageAndExit("error: 'get' not allowed after 'merge|replace|delete'")
			}
			origin, ok := parseOrigin(args[i+1])
			if ok {
				i++
			}
			err := gnmi.Get(ctx, client, gnmi.SplitPaths(args[i+1:]), origin)
			if err != nil {
				glog.Fatal(err)
			}
			return
		case "subscribe":
			if len(setOps) != 0 {
				usageAndExit("error: 'subscribe' not allowed after 'merge|replace|delete'")
			}
			origin, ok := parseOrigin(args[i+1])
			if ok {
				i++
			}
			respChan := make(chan *pb.SubscribeResponse)
			subscribeOptions.Origin = origin
			subscribeOptions.Paths = gnmi.SplitPaths(args[i+1:])
			var g errgroup.Group
			g.Go(func() error {
				return gnmi.SubscribeErr(ctx, client, subscribeOptions, respChan)
			})
			for resp := range respChan {
				if err := gnmi.LogSubscribeResponse(resp); err != nil {
					glog.Fatal(err)
				}
			}
			if err := g.Wait(); err != nil {
				glog.Fatal(err)
			}
			return
		case "update", "replace", "delete":
			// ok if no args, if arbitration was specified
			if len(args) == i+1 && *arbitrationStr == "" {
				usageAndExit("error: missing path")
			}
			op := &gnmi.Operation{
				Type: args[i],
			}
			i++
			if len(args) <= i {
				break
			}
			var ok bool
			op.Origin, ok = parseOrigin(args[i])
			if ok {
				i++
			}
			op.Path = gnmi.SplitPath(args[i])
			if op.Type != "delete" {
				if len(args) == i+1 {
					usageAndExit("error: missing JSON or FILEPATH to data")
				}
				i++
				op.Val = args[i]
			}
			setOps = append(setOps, op)
		default:
			usageAndExit(fmt.Sprintf("error: unknown operation %q", args[i]))
		}
	}
	arb, err := gnmi.ArbitrationExt(*arbitrationStr)
	if err != nil {
		glog.Fatal(err)
	}
	var exts []*gnmi_ext.Extension
	if arb != nil {
		exts = append(exts, arb)
	}
	err = gnmi.Set(ctx, client, setOps, exts...)
	if err != nil {
		glog.Fatal(err)
	}

}

func parseOrigin(s string) (string, bool) {
	if strings.HasPrefix(s, "origin=") {
		return strings.TrimPrefix(s, "origin="), true
	}
	return "", false
}
