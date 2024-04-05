// Copyright (c) 2017 Arista Networks, Inc.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the COPYING file.

package main

import (
	"context"
	"crypto/tls"
	"flag"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/aristanetworks/goarista/gnmi"

	"github.com/aristanetworks/glog"
	hec "github.com/aristanetworks/splunk-hec-go"
	pb "github.com/openconfig/gnmi/proto/gnmi"
	"golang.org/x/sync/errgroup"
)

func exitWithError(s string) {
	fmt.Fprintln(os.Stderr, s)
	os.Exit(1)
}

func main() {
	// gNMI options
	cfg := &gnmi.Config{}
	flag.StringVar(&cfg.Addr, "addr", "localhost", "gNMI gRPC server `address`")
	flag.StringVar(&cfg.CAFile, "cafile", "", "Path to server TLS certificate file")
	flag.StringVar(&cfg.CertFile, "certfile", "", "Path to client TLS certificate file")
	flag.StringVar(&cfg.KeyFile, "keyfile", "", "Path to client TLS private key file")
	flag.StringVar(&cfg.Username, "username", "", "Username to authenticate with")
	flag.StringVar(&cfg.Password, "password", "", "Password to authenticate with")
	flag.BoolVar(&cfg.TLS, "tls", false, "Enable TLS")
	flag.StringVar(&cfg.TLSMinVersion, "tls-min-version", "",
		fmt.Sprintf("Set minimum TLS version for connection (%s)", gnmi.TLSVersions))
	flag.StringVar(&cfg.TLSMaxVersion, "tls-max-version", "",
		fmt.Sprintf("Set maximum TLS version for connection (%s)", gnmi.TLSVersions))
	subscribePaths := flag.String("paths", "/", "Comma-separated list of paths to subscribe to")

	// Splunk options
	splunkURLs := flag.String("splunkurls", "https://localhost:8088",
		"Comma-separated list of URLs of the Splunk servers")
	splunkToken := flag.String("splunktoken", "", "Token to connect to the Splunk servers")
	splunkIndex := flag.String("splunkindex", "", "Index for the data in Splunk")

	flag.Parse()

	// gNMI connection
	ctx := gnmi.NewContext(context.Background(), cfg)
	// Store the address without the port so it can be used as the host in the Splunk event.
	addr := cfg.Addr
	client, err := gnmi.Dial(cfg)
	if err != nil {
		glog.Fatal(err)
	}

	// Splunk connection
	urls := strings.Split(*splunkURLs, ",")
	cluster := hec.NewCluster(urls, *splunkToken)
	cluster.SetHTTPClient(&http.Client{
		Transport: &http.Transport{
			// TODO: add flags for TLS
			TLSClientConfig: &tls.Config{
				// TODO: add flag to enable TLS
				InsecureSkipVerify: true,
			},
		},
	})

	// gNMI subscription
	respChan := make(chan *pb.SubscribeResponse)
	paths := strings.Split(*subscribePaths, ",")
	subscribeOptions := &gnmi.SubscribeOptions{
		Mode:       "stream",
		StreamMode: "target_defined",
		Paths:      gnmi.SplitPaths(paths),
	}
	var g errgroup.Group
	g.Go(func() error { return gnmi.SubscribeErr(ctx, client, subscribeOptions, respChan) })

	// Forward subscribe responses to Splunk
	for resp := range respChan {
		// We got a subscribe response
		response := resp.GetResponse()
		update, ok := response.(*pb.SubscribeResponse_Update)
		if !ok {
			continue
		}

		// Convert the response into a map[string]interface{}
		notification, err := gnmi.NotificationToMap(update.Update)
		if err != nil {
			exitWithError(err.Error())
		}

		// Build the Splunk event
		path := notification["path"].(string)
		delete(notification, "path")
		timestamp := notification["timestamp"].(int64)
		delete(notification, "timestamp")
		// Should this be configurable?
		sourceType := "openconfig"
		event := &hec.Event{
			Host:       &addr,
			Index:      splunkIndex,
			Source:     &path,
			SourceType: &sourceType,
			Event:      notification,
		}
		event.SetTime(time.Unix(timestamp/1e9, timestamp%1e9))

		// Write the event to Splunk
		if err := cluster.WriteEvent(event); err != nil {
			exitWithError("failed to write event: " + err.Error())
		}

	}
	if err := g.Wait(); err != nil {
		exitWithError(err.Error())
	}
}
