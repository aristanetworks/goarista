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
	"regexp"
	"strings"

	"github.com/aristanetworks/goarista/gnmi"

	"github.com/aristanetworks/glog"
	pb "github.com/openconfig/gnmi/proto/gnmi"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"golang.org/x/sync/errgroup"
)

// regex to match tags in descriptions e.g. "[foo][bar=baz]"
const defaultDescriptionRegex = `\[([^=\]]+)(=[^]]+)?]`

func main() {
	// gNMI options
	gNMIcfg := &gnmi.Config{}
	flag.StringVar(&gNMIcfg.Addr, "addr", "localhost", "gNMI gRPC server `address`")
	flag.StringVar(&gNMIcfg.CAFile, "cafile", "", "Path to server TLS certificate file")
	flag.StringVar(&gNMIcfg.CertFile, "certfile", "", "Path to client TLS certificate file")
	flag.StringVar(&gNMIcfg.KeyFile, "keyfile", "", "Path to client TLS private key file")
	flag.StringVar(&gNMIcfg.Username, "username", "", "Username to authenticate with")
	flag.StringVar(&gNMIcfg.Password, "password", "", "Password to authenticate with")
	descRegex := flag.String("description-regex", defaultDescriptionRegex, "custom regex to"+
		" extract labels from description nodes")
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
	// Add to the subscriptions in the config file.
	config.addSubscriptions(subscriptions)

	r := regexp.MustCompile(*descRegex)
	coll := newCollector(config, r)
	prometheus.MustRegister(coll)
	ctx := gnmi.NewContext(context.Background(), gNMIcfg)
	client, err := gnmi.Dial(gNMIcfg)
	if err != nil {
		glog.Fatal(err)
	}

	g, gCtx := errgroup.WithContext(ctx)
	for origin, paths := range config.subsByOrigin {
		subscribeOptions := &gnmi.SubscribeOptions{
			Mode:       "stream",
			StreamMode: "target_defined",
			Paths:      gnmi.SplitPaths(paths),
			Origin:     origin,
		}
		g.Go(func() error {
			return handleSubscription(gCtx, client, subscribeOptions, coll,
				gNMIcfg.Addr)
		})
	}
	http.Handle(*url, promhttp.Handler())
	go func() error {
		return http.ListenAndServe(*listenaddr, nil)
	}()
	if err := g.Wait(); err != nil {
		glog.Fatal(err)
	}
}

func handleSubscription(ctx context.Context, client pb.GNMIClient,
	subscribeOptions *gnmi.SubscribeOptions, coll *collector,
	addr string) error {
	respChan := make(chan *pb.SubscribeResponse)
	go func() {
		for resp := range respChan {
			coll.update(addr, resp)
		}
	}()
	return gnmi.SubscribeErr(ctx, client, subscribeOptions, respChan)
}
