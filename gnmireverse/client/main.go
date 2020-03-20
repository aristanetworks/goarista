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
	"net"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/aristanetworks/goarista/dscp"
	gnmilib "github.com/aristanetworks/goarista/gnmi"
	"github.com/aristanetworks/goarista/gnmireverse"
	"github.com/aristanetworks/goarista/netns"

	"github.com/aristanetworks/glog"
	"github.com/openconfig/gnmi/proto/gnmi"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"
)

type subscriptionList struct {
	subs []subscription
}

type sampleList struct {
	subs []subscription
}

type subscription struct {
	p        *gnmi.Path
	interval time.Duration
}

func str(subs []subscription) string {
	s := make([]string, len(subs))
	for i, sub := range subs {
		s[i] = gnmilib.StrPath(sub.p)
		if sub.interval > 0 {
			s[i] += "@" + sub.interval.String()
		}
	}
	return strings.Join(s, ", ")
}

func (l *subscriptionList) String() string {
	if l == nil {
		return ""
	}
	return str(l.subs)
}

func (l *sampleList) String() string {
	if l == nil {
		return ""
	}
	return str(l.subs)
}

// Set implements flag.Value interface
func (l *subscriptionList) Set(s string) error {
	gnmiPath, err := gnmilib.ParseGNMIElements(gnmilib.SplitPath(s))
	if err != nil {
		return err
	}
	sub := subscription{p: gnmiPath}
	l.subs = append(l.subs, sub)
	return nil
}

// Set implements flag.Value interface
func (l *sampleList) Set(s string) error {
	i := strings.LastIndexByte(s, '@')
	if i == -1 {
		return fmt.Errorf("SAMPLE subscription is missing interval: %q", s)
	}
	interval, err := time.ParseDuration(s[i+1:])
	if err != nil {
		return fmt.Errorf("error parsing interval in %q: %s", s, err)
	}
	if interval < 0 {
		return fmt.Errorf("negative interval not allowed: %q", s)
	}
	gnmiPath, err := gnmilib.ParseGNMIElements(gnmilib.SplitPath(s[:i]))
	if err != nil {
		return err
	}
	sub := subscription{p: gnmiPath, interval: interval}
	l.subs = append(l.subs, sub)
	return nil
}

type config struct {
	// target config
	targetAddr string
	username   string
	password   string

	targetVal        string
	subTargetDefined subscriptionList
	subSample        sampleList

	// collector config
	collectorAddr       string
	sourceAddr          string
	dscp                int
	collectorTLS        bool
	collectorSkipVerify bool
	collectorCert       string
	collectorKey        string
	collectorCA         string
}

func main() {
	var cfg config
	flag.StringVar(&cfg.targetAddr, "target_addr", "127.0.0.1:6030",
		"address of the gNMI target in the form of [<vrf-name>/]address:port")
	flag.StringVar(&cfg.username, "username", "", "username to authenticate with target")
	flag.StringVar(&cfg.password, "password", "", "password to authenticate with target")
	flag.StringVar(&cfg.targetVal, "target_value", "",
		"value to use in the target field of the Subscribe")
	flag.Var(&cfg.subTargetDefined, "subscribe",
		"Path to subscribe with TARGET_DEFINED subscription mode.\n"+
			"This option can be repeated multiple times.")
	flag.Var(&cfg.subSample, "sample",
		"Path to subscribe with SAMPLE subscription mode.\n"+
			"Paths must have suffix of @<sample interval>.\n"+
			"The interval should include a unit, such as 's' for seconds or 'm' for minutes.\n"+
			"For example to subscribe to interface counters with a 30 second sample interval:\n"+
			"  -sample /interfaces/interface/state/counters@30s\n"+
			"This option can be repeated multiple times.")

	flag.StringVar(&cfg.collectorAddr, "collector_addr", "",
		"Address of collector in the form of [<vrf-name>/]host:port.\n"+
			"The host portion must be enclosed in square brackets "+
			"if it is a literal IPv6 address.\n"+
			"For example, -collector_addr mgmt/[::1]:1234")
	flag.StringVar(&cfg.sourceAddr, "source_addr", "",
		"Address to use as source in connection to collectorin the form of ip[:port], or :port.\n"+
			"An IPv6 address must be enclosed in square brackets when specified with a port.\n"+
			"For example, [::1]:1234")
	flag.IntVar(&cfg.dscp, "collector_dscp", 0,
		"DSCP used on connection to collector, valid values 0-63")

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
	targetConn, err := dialTarget(&cfg)
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

	nsName, addr, err := netns.ParseAddress(cfg.collectorAddr)
	if err != nil {
		return nil, fmt.Errorf("error parsing address: %s", err)
	}

	dialer, err := newDialer(cfg)
	if err != nil {
		return nil, err
	}

	dialOptions = append(dialOptions, grpc.WithContextDialer(newVRFDialer(dialer, nsName)))
	return grpc.Dial(addr, dialOptions...)
}

func newVRFDialer(d *net.Dialer, nsName string) func(context.Context, string) (net.Conn, error) {
	return func(ctx context.Context, addr string) (net.Conn, error) {
		var conn net.Conn
		err := netns.Do(nsName, func() error {
			c, err := d.DialContext(ctx, "tcp", addr)
			if err != nil {
				return err
			}
			conn = c
			return nil
		})

		return conn, err
	}
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

func newDialer(cfg *config) (*net.Dialer, error) {
	var d net.Dialer
	if cfg.sourceAddr != "" {
		var localAddr net.TCPAddr
		sourceIP, sourcePort, _ := net.SplitHostPort(cfg.sourceAddr)
		if sourceIP == "" {
			// This can happend if cfg.sourceAddr doesn't have a port
			sourceIP = cfg.sourceAddr
		}
		ip := net.ParseIP(sourceIP)
		if ip == nil {
			return nil, fmt.Errorf("failed to parse IP in source address: %q", sourceIP)
		}
		localAddr.IP = ip

		if sourcePort != "" {
			port, err := strconv.Atoi(sourcePort)
			if err != nil {
				return nil, fmt.Errorf("failed to parse port in source address: %q", sourcePort)
			}
			localAddr.Port = port
		}

		d.LocalAddr = &localAddr
	}

	if cfg.dscp != 0 {
		if cfg.dscp < 0 || cfg.dscp >= 64 {
			return nil, fmt.Errorf("DSCP value must be a value in the range 0-63, got %d", cfg.dscp)
		}
		// DSCP is the top 6 bits of the TOS byte
		tos := byte(cfg.dscp << 2)
		d.Control = func(network, address string, c syscall.RawConn) error {
			return dscp.SetTOS(network, c, tos)
		}
	}

	return &d, nil
}

func dialTarget(cfg *config) (*grpc.ClientConn, error) {
	nsName, addr, err := netns.ParseAddress(cfg.targetAddr)
	if err != nil {
		return nil, fmt.Errorf("error parsing address: %s", err)
	}

	var d net.Dialer
	dialOptions := []grpc.DialOption{
		grpc.WithInsecure(),
		grpc.WithContextDialer(newVRFDialer(&d, nsName)),
	}

	return grpc.Dial(addr, dialOptions...)
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

	for _, sub := range cfg.subTargetDefined.subs {
		subList.Subscription = append(subList.Subscription,
			&gnmi.Subscription{
				Path: sub.p,
				Mode: gnmi.SubscriptionMode_TARGET_DEFINED,
			},
		)
	}
	for _, sub := range cfg.subSample.subs {
		subList.Subscription = append(subList.Subscription,
			&gnmi.Subscription{
				Path:           sub.p,
				Mode:           gnmi.SubscriptionMode_SAMPLE,
				SampleInterval: uint64(sub.interval),
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
