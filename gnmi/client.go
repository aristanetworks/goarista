// Copyright (c) 2017 Arista Networks, Inc.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the COPYING file.

package gnmi

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"flag"
	"fmt"
	"math"
	"net"
	"os"
	"time"

	"io/ioutil"
	"strings"

	"github.com/aristanetworks/goarista/netns"
	"github.com/golang/protobuf/proto"
	pb "github.com/openconfig/gnmi/proto/gnmi"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/encoding/gzip"
	"google.golang.org/grpc/metadata"
)

const (
	defaultPort = "6030"
	// HostnameArg is the value to be replaced by the actual hostname
	HostnameArg = "HOSTNAME"
)

// PublishFunc is the method to publish responses
type PublishFunc func(addr string, message proto.Message)

// ParseHostnames parses a comma-separated list of names and replaces HOSTNAME with the current
// hostname in it
func ParseHostnames(list string) ([]string, error) {
	items := strings.Split(list, ",")
	hostname, err := os.Hostname()
	if err != nil {
		return nil, err
	}
	names := make([]string, len(items))
	for i, name := range items {
		if name == HostnameArg {
			name = hostname
		}
		names[i] = name
	}
	return names, nil
}

// Config is the gnmi.Client config
type Config struct {
	Addr        string
	CAFile      string
	CertFile    string
	KeyFile     string
	Password    string
	Username    string
	TLS         bool
	Compression string
	DialOptions []grpc.DialOption
}

// SubscribeOptions is the gNMI subscription request options
type SubscribeOptions struct {
	UpdatesOnly       bool
	Prefix            string
	Mode              string
	StreamMode        string
	SampleInterval    uint64
	HeartbeatInterval uint64
	Paths             [][]string
	Origin            string
}

// ParseFlags reads arguments from stdin and returns a populated Config object and a list of
// paths to subscribe to
func ParseFlags() (*Config, []string) {
	// flags
	var (
		addrsFlag = flag.String("addrs", "localhost:6030",
			"Comma-separated list of addresses of OpenConfig gRPC servers. The address 'HOSTNAME' "+
				"is replaced by the current hostname.")

		caFileFlag = flag.String("cafile", "",
			"Path to server TLS certificate file")

		certFileFlag = flag.String("certfile", "",
			"Path to client TLS certificate file")

		keyFileFlag = flag.String("keyfile", "",
			"Path to client TLS private key file")

		passwordFlag = flag.String("password", "",
			"Password to authenticate with")

		usernameFlag = flag.String("username", "",
			"Username to authenticate with")

		tlsFlag = flag.Bool("tls", false,
			"Enable TLS")

		compressionFlag = flag.String("compression", "",
			"Type of compression to use")

		subscribeFlag = flag.String("subscribe", "",
			"Comma-separated list of paths to subscribe to upon connecting to the server")
	)
	flag.Parse()
	cfg := &Config{
		Addr:        *addrsFlag,
		CAFile:      *caFileFlag,
		CertFile:    *certFileFlag,
		KeyFile:     *keyFileFlag,
		Password:    *passwordFlag,
		Username:    *usernameFlag,
		TLS:         *tlsFlag,
		Compression: *compressionFlag,
	}
	subscriptions := strings.Split(*subscribeFlag, ",")
	return cfg, subscriptions

}

// Dial connects to a gnmi service and returns a client
func Dial(cfg *Config) (pb.GNMIClient, error) {
	opts := append([]grpc.DialOption(nil), cfg.DialOptions...)

	switch cfg.Compression {
	case "":
	case "gzip":
		opts = append(opts, grpc.WithDefaultCallOptions(grpc.UseCompressor(gzip.Name)))
	default:
		return nil, fmt.Errorf("unsupported compression option: %q", cfg.Compression)
	}

	if cfg.TLS || cfg.CAFile != "" || cfg.CertFile != "" {
		tlsConfig := &tls.Config{}
		if cfg.CAFile != "" {
			b, err := ioutil.ReadFile(cfg.CAFile)
			if err != nil {
				return nil, err
			}
			cp := x509.NewCertPool()
			if !cp.AppendCertsFromPEM(b) {
				return nil, fmt.Errorf("credentials: failed to append certificates")
			}
			tlsConfig.RootCAs = cp
		} else {
			tlsConfig.InsecureSkipVerify = true
		}
		if cfg.CertFile != "" {
			if cfg.KeyFile == "" {
				return nil, fmt.Errorf("please provide both -certfile and -keyfile")
			}
			cert, err := tls.LoadX509KeyPair(cfg.CertFile, cfg.KeyFile)
			if err != nil {
				return nil, err
			}
			tlsConfig.Certificates = []tls.Certificate{cert}
		}
		opts = append(opts, grpc.WithTransportCredentials(credentials.NewTLS(tlsConfig)))
	} else {
		opts = append(opts, grpc.WithInsecure())
	}

	if !strings.ContainsRune(cfg.Addr, ':') {
		cfg.Addr += ":" + defaultPort
	}

	dial := func(addrIn string, time time.Duration) (net.Conn, error) {
		var conn net.Conn
		nsName, addr, err := netns.ParseAddress(addrIn)
		if err != nil {
			return nil, err
		}

		err = netns.Do(nsName, func() error {
			var err error
			conn, err = net.Dial("tcp", addr)
			return err
		})
		return conn, err
	}

	opts = append(opts,
		grpc.WithDialer(dial),

		// Allows received protobuf messages to be larger than 4MB
		grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(math.MaxInt32)),
	)
	grpcconn, err := grpc.Dial(cfg.Addr, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to dial: %s", err)
	}
	return pb.NewGNMIClient(grpcconn), nil
}

// NewContext returns a new context with username and password
// metadata if they are set in cfg.
func NewContext(ctx context.Context, cfg *Config) context.Context {
	if cfg.Username != "" {
		ctx = metadata.NewOutgoingContext(ctx, metadata.Pairs(
			"username", cfg.Username,
			"password", cfg.Password))
	}
	return ctx
}

// NewGetRequest returns a GetRequest for the given paths
func NewGetRequest(paths [][]string, origin string) (*pb.GetRequest, error) {
	req := &pb.GetRequest{
		Path: make([]*pb.Path, len(paths)),
	}
	for i, p := range paths {
		gnmiPath, err := ParseGNMIElements(p)
		if err != nil {
			return nil, err
		}
		req.Path[i] = gnmiPath
		req.Path[i].Origin = origin
	}
	return req, nil
}

// NewSubscribeRequest returns a SubscribeRequest for the given paths
func NewSubscribeRequest(subscribeOptions *SubscribeOptions) (*pb.SubscribeRequest, error) {
	var mode pb.SubscriptionList_Mode
	switch subscribeOptions.Mode {
	case "once":
		mode = pb.SubscriptionList_ONCE
	case "poll":
		mode = pb.SubscriptionList_POLL
	case "":
		fallthrough
	case "stream":
		mode = pb.SubscriptionList_STREAM
	default:
		return nil, fmt.Errorf("subscribe mode (%s) invalid", subscribeOptions.Mode)
	}

	var streamMode pb.SubscriptionMode
	switch subscribeOptions.StreamMode {
	case "on_change":
		streamMode = pb.SubscriptionMode_ON_CHANGE
	case "sample":
		streamMode = pb.SubscriptionMode_SAMPLE
	case "":
		fallthrough
	case "target_defined":
		streamMode = pb.SubscriptionMode_TARGET_DEFINED
	default:
		return nil, fmt.Errorf("subscribe stream mode (%s) invalid", subscribeOptions.StreamMode)
	}

	prefixPath, err := ParseGNMIElements(SplitPath(subscribeOptions.Prefix))
	if err != nil {
		return nil, err
	}
	subList := &pb.SubscriptionList{
		Subscription: make([]*pb.Subscription, len(subscribeOptions.Paths)),
		Mode:         mode,
		UpdatesOnly:  subscribeOptions.UpdatesOnly,
		Prefix:       prefixPath,
	}
	for i, p := range subscribeOptions.Paths {
		gnmiPath, err := ParseGNMIElements(p)
		if err != nil {
			return nil, err
		}
		gnmiPath.Origin = subscribeOptions.Origin
		subList.Subscription[i] = &pb.Subscription{
			Path:              gnmiPath,
			Mode:              streamMode,
			SampleInterval:    subscribeOptions.SampleInterval,
			HeartbeatInterval: subscribeOptions.HeartbeatInterval,
		}
	}
	return &pb.SubscribeRequest{Request: &pb.SubscribeRequest_Subscribe{
		Subscribe: subList}}, nil
}
