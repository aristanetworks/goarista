// Copyright (c) 2020 Arista Networks, Inc.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the COPYING file.

package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"flag"
	"fmt"
	"io/ioutil"
	"strings"

	gnmilib "github.com/aristanetworks/goarista/gnmi"
	"github.com/aristanetworks/goarista/gnmireverse"

	"github.com/aristanetworks/glog"
	"github.com/openconfig/gnmi/proto/gnmi"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"
)

type multiPath struct {
	p []*gnmi.Path
}

func (m *multiPath) String() string {
	if m == nil {
		return ""
	}
	s := make([]string, len(m.p))
	for i, p := range m.p {
		s[i] = gnmilib.StrPath(p)
	}
	return strings.Join(s, ", ")
}

// Set implements flag.Value interface
func (m *multiPath) Set(s string) error {
	gnmiPath, err := gnmilib.ParseGNMIElements(gnmilib.SplitPath(s))
	if err != nil {
		return err
	}
	m.p = append(m.p, gnmiPath)
	return nil
}

type config struct {
	// target config
	targetAddr string
	username   string
	password   string

	targetVal string
	paths     multiPath

	// collector config
	collectorAddr       string
	sourceAddr          string
	collectorTLS        bool
	collectorSkipVerify bool
	collectorCert       string
	collectorKey        string
	collectorCA         string
}

func main() {
	var cfg config
	flag.StringVar(&cfg.targetAddr, "target_addr", "127.0.0.1:6030", "address of the gNMI target")
	flag.StringVar(&cfg.username, "username", "", "username to authenticate with target")
	flag.StringVar(&cfg.password, "password", "", "password to authenticate with target")
	flag.StringVar(&cfg.targetVal, "target_value", "",
		"value to use in the target field of the Subscribe")
	flag.Var(&cfg.paths, "subscribe",
		"Path to subscribe to. This option can be repeated multiple times.")

	flag.StringVar(&cfg.collectorAddr, "collector_addr", "",
		"address of collector in the form of [<vrf-name>/]address:port")
	flag.StringVar(&cfg.sourceAddr, "source_addr", "",
		"addr to use as source in connection to collector")

	flag.BoolVar(&cfg.collectorTLS, "collector_tls", true, "use TLS in connection with collector")
	flag.BoolVar(&cfg.collectorSkipVerify, "collector_tls_skipverify", false,
		"don't verify collector's certificate (insecure)")
	flag.StringVar(&cfg.collectorCert, "collector_certfile", "",
		"path to TLS certificate file to authenticate with collector")
	flag.StringVar(&cfg.collectorKey, "collector_keyfile", "",
		"path to TLS key file to authenticate with collector")
	flag.StringVar(&cfg.collectorCA, "collector_cafile", "",
		"path to TLS CA file to verify collector (leave empty to use host's root CA set)")

	flag.Parse()

	destConn, err := dialCollector(&cfg)
	if err != nil {
		glog.Fatalf("error dialing destination %q: %s", cfg.collectorAddr, err)
	}
	targetConn, err := grpc.Dial(cfg.targetAddr, grpc.WithInsecure())
	if err != nil {
		glog.Fatalf("error dialing target %q: %s", cfg.targetAddr, err)
	}

	for {
		// Start publisher and subscriber in a loop, each running in
		// their own goroutine. If either of them encounters an error,
		// retry.
		eg, ctx := errgroup.WithContext(context.Background())
		// c is used to send subscribe responses from subscriber to
		// publisher.
		c := make(chan *gnmi.SubscribeResponse)
		eg.Go(func() error {
			return publish(ctx, destConn, c)
		})
		eg.Go(func() error {
			return subscribe(ctx, &cfg, targetConn, c)
		})
		err := eg.Wait()
		if err != nil {
			glog.Errorf("encountered error, retrying: %s", err)
		}
	}
}

// TODO: handle VRF, sourceAddr, DSCP
func dialCollector(cfg *config) (*grpc.ClientConn, error) {
	var dialOptions []grpc.DialOption

	if cfg.collectorTLS {
		tlsConfig, err := newTLSConfig(cfg.collectorSkipVerify,
			cfg.collectorCert, cfg.collectorKey, cfg.collectorCA)
		if err != nil {
			glog.Fatalf("error creating TLS config for collector: %s", err)
		}
		dialOptions = append(dialOptions,
			grpc.WithTransportCredentials(credentials.NewTLS(tlsConfig)))
	} else {
		dialOptions = append(dialOptions, grpc.WithInsecure())
	}

	return grpc.Dial(cfg.collectorAddr, dialOptions...)
}

func newTLSConfig(skipVerify bool, certFile, keyFile, caFile string) (*tls.Config,
	error) {
	var tlsConfig tls.Config
	if skipVerify {
		tlsConfig.InsecureSkipVerify = true
	} else if caFile != "" {
		b, err := ioutil.ReadFile(caFile)
		if err != nil {
			return nil, err
		}
		cp := x509.NewCertPool()
		if !cp.AppendCertsFromPEM(b) {
			return nil, fmt.Errorf("credentials: failed to append certificates")
		}
		tlsConfig.RootCAs = cp
	}
	if certFile != "" {
		if keyFile == "" {
			return nil, fmt.Errorf("please provide both -collector_certfile and -collector_keyfile")
		}
		cert, err := tls.LoadX509KeyPair(certFile, keyFile)
		if err != nil {
			return nil, err
		}
		tlsConfig.Certificates = []tls.Certificate{cert}
	}
	return &tlsConfig, nil
}

func publish(ctx context.Context, destConn *grpc.ClientConn,
	c <-chan *gnmi.SubscribeResponse) error {
	client := gnmireverse.NewGNMIReverseClient(destConn)
	stream, err := client.Publish(ctx, grpc.WaitForReady(true))
	if err != nil {
		return fmt.Errorf("error from Publish: %s", err)
	}
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case response := <-c:
			if err := stream.Send(response); err != nil {
				return fmt.Errorf("error from Publish.Send: %s", err)
			}
		}
	}
}

func subscribe(ctx context.Context, cfg *config, targetConn *grpc.ClientConn,
	c chan<- *gnmi.SubscribeResponse) error {
	client := gnmi.NewGNMIClient(targetConn)
	subList := &gnmi.SubscriptionList{
		Prefix: &gnmi.Path{Target: cfg.targetVal},
	}

	for _, p := range cfg.paths.p {
		subList.Subscription = append(subList.Subscription,
			&gnmi.Subscription{
				Path: p,
				Mode: gnmi.SubscriptionMode_TARGET_DEFINED,
			},
		)
	}
	request := &gnmi.SubscribeRequest{
		Request: &gnmi.SubscribeRequest_Subscribe{
			Subscribe: subList,
		},
	}

	if cfg.username != "" {
		ctx = metadata.NewOutgoingContext(ctx,
			metadata.Pairs(
				"username", cfg.username,
				"password", cfg.password),
		)
	}
	stream, err := client.Subscribe(ctx, grpc.WaitForReady(true))
	if err != nil {
		return fmt.Errorf("error from Subscribe: %s", err)
	}
	if err := stream.Send(request); err != nil {
		return fmt.Errorf("error sending SubscribeRequest: %s", err)
	}

	for {
		resp, err := stream.Recv()
		if err != nil {
			return fmt.Errorf("error from Subscribe.Recv: %s", err)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case c <- resp:
		}
	}
}
