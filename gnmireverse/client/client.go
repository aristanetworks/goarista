// Copyright (c) 2020 Arista Networks, Inc.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the COPYING file.

package client

import (
	"bufio"
	"context"
	"crypto/tls"
	"crypto/x509"
	"flag"
	"fmt"
	"io/ioutil"
	"math"
	"net"
	"os"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/aristanetworks/glog"
	"github.com/aristanetworks/goarista/dscp"
	gnmilib "github.com/aristanetworks/goarista/gnmi"
	"github.com/aristanetworks/goarista/gnmireverse"
	"github.com/aristanetworks/goarista/netns"
	"github.com/cenkalti/backoff/v4"
	"github.com/openconfig/gnmi/proto/gnmi"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/encoding/gzip"
	"google.golang.org/grpc/grpclog"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/proto"
)

const (
	// errorLoopRetryMaxInterval caps the time between error loop retries.
	errorLoopRetryMaxInterval = time.Minute
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

type getList struct {
	openconfigPaths []*gnmi.Path
	eosNativePaths  []*gnmi.Path
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

func (l *getList) String() string {
	if l == nil {
		return ""
	}
	var pathStrs []string
	for _, path := range l.openconfigPaths {
		pathStrs = append(pathStrs, gnmilib.StrPath(path))
	}
	for _, path := range l.eosNativePaths {
		pathStrs = append(pathStrs, gnmilib.StrPath(path))
	}
	return strings.Join(pathStrs, ", ")
}

func parseInterval(s string) (time.Duration, int, error) {
	i := strings.LastIndexByte(s, '@')
	if i == -1 {
		return -1, -1, fmt.Errorf("SAMPLE subscription is missing interval: %q", s)
	}
	interval, err := time.ParseDuration(s[i+1:])
	if err != nil {
		return -1, i, fmt.Errorf("error parsing interval in %q: %s", s, err)
	}
	if interval < 0 {
		return -1, i, fmt.Errorf("negative interval not allowed: %q", s)
	}
	return interval, i, nil
}

func setSubscriptions(subs *[]subscription, s string, interval time.Duration) error {
	gnmiPath, err := gnmilib.ParseGNMIElements(gnmilib.SplitPath(s))
	if err != nil {
		return err
	}
	sub := subscription{p: gnmiPath, interval: interval}
	*subs = append(*subs, sub)
	return nil
}

// Set implements flag.Value interface
func (l *subscriptionList) Set(s string) error {
	interval, i, err := parseInterval(s)
	if err != nil {
		if i == -1 {
			// for subscription list, if there is no intervals, it's ok
			interval = 0
			i = len(s)
		} else {
			// invalid interval is found
			return err
		}
	}
	return setSubscriptions(&l.subs, s[:i], interval)
}

// Set implements flag.Value interface
func (l *sampleList) Set(s string) error {
	interval, i, err := parseInterval(s)
	if err != nil {
		// sample list must come with intervals
		return err
	}
	return setSubscriptions(&l.subs, s[:i], interval)
}

func (l *getList) Set(gnmiPathStr string) error {
	switch {
	case strings.HasPrefix(gnmiPathStr, "eos_native:"):
		gnmiPathStr = strings.TrimPrefix(gnmiPathStr, "eos_native:")
		eosNativePath, err := gnmilib.ParseGNMIElements(gnmilib.SplitPath(gnmiPathStr))
		if err != nil {
			return err
		}
		eosNativePath.Origin = "eos_native"
		l.eosNativePaths = append(l.eosNativePaths, eosNativePath)
	default:
		gnmiPathStr = strings.TrimPrefix(gnmiPathStr, "openconfig:")
		openconfigPath, err := gnmilib.ParseGNMIElements(gnmilib.SplitPath(gnmiPathStr))
		if err != nil {
			return err
		}
		l.openconfigPaths = append(l.openconfigPaths, openconfigPath)
	}
	return nil
}

func (l *getList) readGetPathsFile(filePath string) {
	file, err := os.Open(filePath)
	if err != nil {
		glog.Fatalf("failed to read Get paths file %q: %s", filePath, err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		if path := strings.TrimSpace(scanner.Text()); path != "" {
			l.Set(path)
		}
	}

	if err := scanner.Err(); err != nil {
		glog.Fatalf("failed to read Get paths file %q: %s", filePath, err)
	}
}

type config struct {
	// target config
	targetAddr        string
	username          string
	password          string
	targetTLSInsecure bool

	targetVal        string
	subTargetDefined subscriptionList
	subSample        sampleList
	origin           string

	getSampleInterval time.Duration
	getPaths          getList

	// collector config
	collectorAddr        string
	sourceAddr           string
	dscp                 int
	collectorTLS         bool
	collectorSkipVerify  bool
	collectorCert        string
	collectorKey         string
	collectorCA          string
	collectorCompression string
}

// Main initializes the gNMIReverse client.
func Main() {
	var cfg config
	flag.StringVar(&cfg.targetAddr, "target_addr", "127.0.0.1:6030",
		"address of the gNMI target in the form of [<vrf-name>/]address:port")
	flag.StringVar(&cfg.username, "username", "", "username to authenticate with target")
	flag.StringVar(&cfg.password, "password", "", "password to authenticate with target")
	flag.StringVar(&cfg.targetVal, "target_value", "",
		"value to use in the target field of the Subscribe")
	flag.Var(&cfg.subTargetDefined, "subscribe",
		"Path to subscribe with TARGET_DEFINED subscription mode.\n"+
			"To set a heartbeat interval include a suffix of @<heartbeat interval>.\n"+
			"The interval should include a unit, such as 's' for seconds or 'm' for minutes.\n"+
			"This option can be repeated multiple times.")
	flag.Var(&cfg.subSample, "sample",
		"Path to subscribe with SAMPLE subscription mode.\n"+
			"Paths must have suffix of @<sample interval>.\n"+
			"The interval should include a unit, such as 's' for seconds or 'm' for minutes.\n"+
			"For example to subscribe to interface counters with a 30 second sample interval:\n"+
			"  -sample /interfaces/interface/state/counters@30s\n"+
			"This option can be repeated multiple times.")
	flag.StringVar(&cfg.origin, "origin", "", "value for the origin field of the Subscribe")
	flag.BoolVar(&cfg.targetTLSInsecure, "target_tls_insecure", false,
		"use TLS connection with target and do not verify target certificate")

	flag.Var(&cfg.getPaths, "get", "Path to retrieve periodically using Get.\n"+
		"Arista EOS native origin paths can be specified with the prefix \"eos_native:\".\n"+
		"For example, eos_native:/Sysdb/hardware\n"+
		"This option can be repeated multiple times.")
	getPathsFile := flag.String("get_file", "", "Read file containing a list of paths"+
		" separated by newlines to retrieve periodically using Get.")
	getSampleIntervalStr := flag.String("get_sample_interval", "",
		"Interval between periodic Get requests (400ms, 2.5s, 1m, etc.)\n"+
			"Must be specified for Get and applies to all Get paths.")

	flag.StringVar(&cfg.collectorAddr, "collector_addr", "",
		"Address of collector in the form of [<vrf-name>/]host:port.\n"+
			"The host portion must be enclosed in square brackets "+
			"if it is a literal IPv6 address.\n"+
			"For example, -collector_addr mgmt/[::1]:1234")
	flag.StringVar(&cfg.sourceAddr, "source_addr", "",
		"Address to use as source in connection to collector in the form of ip[:port], or :port.\n"+
			"An IPv6 address must be enclosed in square brackets when specified with a port.\n"+
			"For example, [::1]:1234")
	flag.IntVar(&cfg.dscp, "collector_dscp", 0,
		"DSCP used on connection to collector, valid values 0-63")
	flag.StringVar(&cfg.collectorCompression, "collector_compression", "none",
		"compression method used when streaming to collector (none | gzip)")

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

	// No arguments are expected.
	if len(flag.Args()) > 0 {
		glog.Fatalf("unexpected arguments: %s", flag.Args())
	}

	// If -v is specified, enables gRPC logging at level corresponding to verbosity evel.
	if glog.V(1) {
		glogVStr := flag.Lookup("v").Value.String()
		logLevel, err := strconv.Atoi(glogVStr)
		if err != nil {
			glog.Infof("cannot parse %q", glogVStr)
		} else {
			grpclog.SetLoggerV2(
				grpclog.NewLoggerV2WithVerbosity(os.Stdout, os.Stdout, os.Stdout, logLevel))
		}
	}

	if cfg.collectorAddr == "" {
		glog.Fatal("collector address must be specified")
	}

	if *getPathsFile != "" {
		cfg.getPaths.readGetPathsFile(*getPathsFile)
	}

	if *getSampleIntervalStr != "" {
		getSampleInterval, err := time.ParseDuration(*getSampleIntervalStr)
		if err != nil {
			glog.Fatalf("Get sample interval %q invalid", *getSampleIntervalStr)
		}
		cfg.getSampleInterval = getSampleInterval
	}

	isSubscribe := len(cfg.subTargetDefined.subs) != 0 || len(cfg.subSample.subs) != 0
	isGet := len(cfg.getPaths.openconfigPaths) != 0 || len(cfg.getPaths.eosNativePaths) != 0

	if !isSubscribe && !isGet {
		glog.Fatal("Subscribe paths or Get paths must be specifed")
	}
	if !isGet && cfg.getSampleInterval != 0 {
		glog.Fatal("Get path must be specified with Get sample interval")
	}
	if isGet && cfg.getSampleInterval == 0 {
		glog.Fatal("Get sample interval must be specified with Get path")
	}

	if cfg.origin != "" {
		// Workaround for EOS BUG479731: set origin on paths, rather
		// than on the prefix.
		for _, sub := range cfg.subTargetDefined.subs {
			sub.p.Origin = cfg.origin
		}
		for _, sub := range cfg.subSample.subs {
			sub.p.Origin = cfg.origin
		}
		for _, get := range cfg.getPaths.openconfigPaths {
			get.Origin = cfg.origin
		}
		// If "eos_native" was specified by the global origin flag,
		// point Get paths to EOS native Get paths instead.
		if strings.ToLower(cfg.origin) == "eos_native" {
			cfg.getPaths.eosNativePaths = cfg.getPaths.openconfigPaths
			cfg.getPaths.openconfigPaths = nil
		}
	}

	destConn, err := dialCollector(&cfg)
	if err != nil {
		glog.Fatalf("error dialing destination %q: %s", cfg.collectorAddr, err)
	}
	targetConn, err := dialTarget(&cfg)
	if err != nil {
		glog.Fatalf("error dialing target %q: %s", cfg.targetAddr, err)
	}

	if isSubscribe {
		go streamResponses(streamSubscribeResponses(&cfg, destConn, targetConn))
	}
	if isGet {
		go streamResponses(streamGetResponses(&cfg, destConn, targetConn))
	}
	select {} // Wait forever
}

func streamResponses(streamResponsesFunc func(context.Context, *errgroup.Group)) {
	// Used for error loop detection and backoff retries.
	var lastErrorTime time.Time
	bo := backoff.NewExponentialBackOff()
	bo.MaxElapsedTime = 0 // Never stop
	bo.MaxInterval = errorLoopRetryMaxInterval
	bo.Reset()

	for {
		// Start publisher and client in a loop, each running in
		// their own goroutine. If either of them encounters an error,
		// retry.
		var eg *errgroup.Group
		eg, ctx := errgroup.WithContext(context.Background())
		streamResponsesFunc(ctx, eg)
		if err := eg.Wait(); err != nil {
			nowTime := time.Now()
			// If the last error was from a while ago, reset the backoff interval because
			// this error is not from an error loop.
			if lastErrorTime.Add(errorLoopRetryMaxInterval * 2).Before(nowTime) {
				bo.Reset()
			}
			lastErrorTime = nowTime
			glog.Infof("encountered error, retrying: %s", err)
			time.Sleep(bo.NextBackOff())
		}
	}
}

func streamSubscribeResponses(cfg *config, destConn, targetConn *grpc.ClientConn) func(
	context.Context, *errgroup.Group) {
	return func(ctx context.Context, eg *errgroup.Group) {
		c := make(chan *gnmi.SubscribeResponse)
		eg.Go(func() error {
			return publish(ctx, destConn, c)
		})
		eg.Go(func() error {
			return subscribe(ctx, cfg, targetConn, c)
		})
	}
}

func streamGetResponses(cfg *config, destConn, targetConn *grpc.ClientConn) func(
	context.Context, *errgroup.Group) {
	return func(ctx context.Context, eg *errgroup.Group) {
		c := make(chan *gnmi.GetResponse)
		eg.Go(func() error {
			return publishGet(ctx, destConn, c)
		})
		eg.Go(func() error {
			return sampleGet(ctx, cfg, targetConn, c)
		})
	}
}

func dialCollector(cfg *config) (*grpc.ClientConn, error) {
	var dialOptions []grpc.DialOption

	if cfg.collectorTLS {
		tlsConfig, err := newTLSConfig(cfg.collectorSkipVerify,
			cfg.collectorCert, cfg.collectorKey, cfg.collectorCA)
		if err != nil {
			return nil, fmt.Errorf("error creating TLS config for collector: %s", err)
		}
		dialOptions = append(dialOptions,
			grpc.WithTransportCredentials(credentials.NewTLS(tlsConfig)))
	} else {
		dialOptions = append(dialOptions, grpc.WithInsecure())
	}

	switch cfg.collectorCompression {
	case "", "none":
	case "gzip":
		dialOptions = append(dialOptions,
			grpc.WithDefaultCallOptions(grpc.UseCompressor(gzip.Name)))
	default:
		return nil, fmt.Errorf("unknown compression method %q", cfg.collectorCompression)
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

	var dialOptions []grpc.DialOption
	if cfg.targetTLSInsecure {
		dialOptions = append(dialOptions,
			grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{
				InsecureSkipVerify: true,
			})))
	} else {
		dialOptions = append(dialOptions, grpc.WithInsecure())
	}

	var d net.Dialer
	dialOptions = append(dialOptions,
		grpc.WithContextDialer(newVRFDialer(&d, nsName)),
		grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(math.MaxInt32)),
	)

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

func publishGet(ctx context.Context, destConn *grpc.ClientConn, c <-chan *gnmi.GetResponse) error {
	client := gnmireverse.NewGNMIReverseClient(destConn)
	stream, err := client.PublishGet(ctx, grpc.WaitForReady(true))
	if err != nil {
		return fmt.Errorf("error from PublishGet: %s", err)
	}
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case response := <-c:
			if glog.V(3) {
				glog.Infof("send Get response: size_bytes=%d num_notifs=%d",
					proto.Size(response), len(response.GetNotification()))
			}
			if glog.V(7) {
				glog.Infof("send Get response to collector: %v", response)
			}
			if err := stream.Send(response); err != nil {
				return fmt.Errorf("error from PublishGet.Send: %s", err)
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
				Path:              sub.p,
				Mode:              gnmi.SubscriptionMode_TARGET_DEFINED,
				HeartbeatInterval: uint64(sub.interval),
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

func sampleGet(ctx context.Context, cfg *config, targetConn *grpc.ClientConn,
	c chan<- *gnmi.GetResponse) error {
	client := gnmi.NewGNMIClient(targetConn)

	openconfigGetReq := &gnmi.GetRequest{
		Path: cfg.getPaths.openconfigPaths,
	}

	eosNativeGetReq := &gnmi.GetRequest{
		Path: cfg.getPaths.eosNativePaths,
	}

	if cfg.username != "" {
		ctx = metadata.NewOutgoingContext(ctx,
			metadata.Pairs(
				"username", cfg.username,
				"password", cfg.password),
		)
	}

	// Set up a ticker for a consistent interval to exclude the additional time taken
	// for issuing the Get request(s) and processing the response(s).
	ticker := time.NewTicker(cfg.getSampleInterval)
	defer ticker.Stop()

	for {
		var openConfigGetResponse *gnmi.GetResponse
		if len(cfg.getPaths.openconfigPaths) > 0 {
			if glog.V(5) {
				glog.Infof("send OpenConfig Get request to target: %v", openconfigGetReq)
			}
			var err error
			openConfigGetResponse, err = client.Get(ctx, openconfigGetReq, grpc.WaitForReady(true))
			if err != nil {
				return fmt.Errorf("error from OpenConfig Get: %s", err)
			}
			if glog.V(7) {
				glog.Infof("receive OpenConfig Get response: %v", openConfigGetResponse)
			}
		}

		// Issue separate Get request for EOS native paths because target may not support mixed
		// origin paths in the same Get request.
		var eosNativeGetResponse *gnmi.GetResponse
		if len(cfg.getPaths.eosNativePaths) > 0 {
			if glog.V(5) {
				glog.Infof("send EOS native Get request to target: %v", eosNativeGetReq)
			}
			var err error
			eosNativeGetResponse, err = client.Get(ctx, eosNativeGetReq, grpc.WaitForReady(true))
			if err != nil {
				return fmt.Errorf("error from EOS native Get: %s", err)
			}
			if glog.V(7) {
				glog.Infof("receive EOS native Get response: %v", eosNativeGetResponse)
			}
		}

		// Combine the Get responses.
		currentTime := time.Now().UnixNano()
		combinedGetResponse := combineGetResponses(
			currentTime, cfg.targetVal, openConfigGetResponse, eosNativeGetResponse)

		select {
		case <-ctx.Done():
			return ctx.Err()
		case c <- combinedGetResponse:
		}

		glog.V(5).Infof("wait for %s", cfg.getSampleInterval)
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}

// combineGetResponses combines the notifications of GetResponses to one GetResponse
// with the same timestamp and target prefix for all notifications.
func combineGetResponses(timestamp int64, target string,
	getResponses ...*gnmi.GetResponse) *gnmi.GetResponse {
	var totalNotifications int
	for _, res := range getResponses {
		totalNotifications += len(res.GetNotification())
	}
	combinedGetResponse := &gnmi.GetResponse{
		Notification: make([]*gnmi.Notification, 0, totalNotifications),
	}
	for _, res := range getResponses {
		for _, notif := range res.GetNotification() {
			// Workaround for EOS BUG568084: set timestamp on GetResponse notification.
			notif.Timestamp = timestamp
			if notif.GetPrefix() == nil {
				notif.Prefix = &gnmi.Path{}
			}
			notif.Prefix.Target = target
			combinedGetResponse.Notification = append(combinedGetResponse.Notification, notif)
		}
	}
	return combinedGetResponse
}
