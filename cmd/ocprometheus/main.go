// Copyright (c) 2017 Arista Networks, Inc.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the COPYING file.

// The ocprometheus implements a Prometheus exporter for OpenConfig telemetry data.
package main

import (
	"context"
	"flag"
	"io/ioutil"
	"net/http"
	"strings"

	"github.com/aristanetworks/goarista/gnmi"

	"github.com/aristanetworks/glog"
	pb "github.com/openconfig/gnmi/proto/gnmi"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"golang.org/x/sync/errgroup"
)

func main() {
	// gNMI options
	gNMIcfg := &gnmi.Config{}
	flag.StringVar(&gNMIcfg.Addr, "addr", "localhost", "gNMI gRPC server `address`")
	flag.StringVar(&gNMIcfg.CAFile, "cafile", "", "Path to server TLS certificate file")
	flag.StringVar(&gNMIcfg.CertFile, "certfile", "", "Path to client TLS certificate file")
	flag.StringVar(&gNMIcfg.KeyFile, "keyfile", "", "Path to client TLS private key file")
	flag.StringVar(&gNMIcfg.Username, "username", "", "Username to authenticate with")
	flag.StringVar(&gNMIcfg.Password, "password", "", "Password to authenticate with")
	flag.BoolVar(&gNMIcfg.TLS, "tls", false, "Enable TLS")
	subscribePaths := flag.String("subscribe", "/", "Comma-separated list of paths to subscribe to")

	// program options
	listenaddr := flag.String("listenaddr", ":8080", "Address on which to expose the metrics")
	url := flag.String("url", "/metrics", "URL where to expose the metrics")
	configFlag := flag.String("config", "",
		"Config to turn OpenConfig telemetry into Prometheus metrics")

	flag.Parse()
	subscriptions := strings.Split(*subscribePaths, ",")
	if *configFlag == "" {
		glog.Fatal("You need specify a config file using -config flag")
	}
	cfg, err := ioutil.ReadFile(*configFlag)
	if err != nil {
		glog.Fatalf("Can't read config file %q: %v", *configFlag, err)
	}
	config, err := parseConfig(cfg)
	if err != nil {
		glog.Fatal(err)
	}

	// Ignore the default "subscribe-to-everything" subscription of the
	// -subscribe flag.
	if subscriptions[0] == "/" {
		subscriptions = subscriptions[1:]
	}
	// Add the subscriptions from the config file.
	subscriptions = append(subscriptions, config.Subscriptions...)

	coll := newCollector(config)
	prometheus.MustRegister(coll)
	ctx := gnmi.NewContext(context.Background(), gNMIcfg)
	client, err := gnmi.Dial(gNMIcfg)
	if err != nil {
		glog.Fatal(err)
	}

	respChan := make(chan *pb.SubscribeResponse)
	subscribeOptions := &gnmi.SubscribeOptions{
		Mode:       "stream",
		StreamMode: "target_defined",
		Paths:      gnmi.SplitPaths(subscriptions),
	}
	go handleSubscription(ctx, client, subscribeOptions, respChan, coll, gNMIcfg.Addr)
	http.Handle(*url, promhttp.Handler())
	glog.Fatal(http.ListenAndServe(*listenaddr, nil))
}

func handleSubscription(ctx context.Context, client pb.GNMIClient,
	subscribeOptions *gnmi.SubscribeOptions, respChan chan *pb.SubscribeResponse, coll *collector,
	addr string) {
	var g errgroup.Group
	g.Go(func() error { return gnmi.SubscribeErr(ctx, client, subscribeOptions, respChan) })
	for resp := range respChan {
		coll.update(addr, resp)
	}
	if err := g.Wait(); err != nil {
		glog.Fatal(err)
	}
}
