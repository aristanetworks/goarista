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
	"strings"

	"github.com/aristanetworks/goarista/netns"
	pb "github.com/openconfig/gnmi/proto/gnmi"
	"github.com/openconfig/gnmi/proto/gnmi_ext"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/encoding/gzip"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/proto"
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
	Addr string

	// File path to load data or raw cert data. Alternatively, raw data can be provided below.
	CAFile   string
	CertFile string
	KeyFile  string

	// Raw certificate data. If respective file is provided above, that is used instead.
	CAData   []byte
	CertData []byte
	KeyData  []byte

	Password     string
	Username     string
	TLS          bool
	Compression  string
	BDP          bool
	DialOptions  []grpc.DialOption
	Token        string
	GRPCMetadata map[string]string
}

// SubscribeOptions is the gNMI subscription request options
type SubscribeOptions struct {
	UpdatesOnly       bool
	Prefix            string
	Mode              string
	StreamMode        string
	SampleInterval    uint64
	SuppressRedundant bool
	HeartbeatInterval uint64
	Paths             [][]string
	Origin            string
	Target            string
	Extensions        []*gnmi_ext.Extension
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

		token = flag.String("token", "",
			"Authentication token")
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
		Token:       *token,
	}
	subscriptions := strings.Split(*subscribeFlag, ",")
	return cfg, subscriptions

}

// accessTokenCred implements credentials.PerRPCCredentials, the gRPC
// interface for credentials that need to attach security information
// to every RPC.
type accessTokenCred struct {
	bearerToken string
}

// newAccessTokenCredential constructs a new per-RPC credential from a token.
func newAccessTokenCredential(token string) credentials.PerRPCCredentials {
	bearerFmt := "Bearer %s"
	return &accessTokenCred{bearerToken: fmt.Sprintf(bearerFmt, token)}
}

func (a *accessTokenCred) GetRequestMetadata(ctx context.Context,
	uri ...string) (map[string]string, error) {
	authHeader := "Authorization"
	return map[string]string{
		authHeader: a.bearerToken,
	}, nil
}

func (a *accessTokenCred) RequireTransportSecurity() bool { return true }

// DialContextConn connects to a gnmi service and return a client connection
func DialContextConn(ctx context.Context, cfg *Config) (*grpc.ClientConn, error) {
	opts := append([]grpc.DialOption(nil), cfg.DialOptions...)

	if !cfg.BDP {
		// By default, the client and server will dynamically adjust the connection's
		// window size using the Bandwidth Delay Product (BDP).
		// See: https://grpc.io/blog/grpc-go-perf-improvements/
		// The default values for InitialWindowSize and InitialConnWindowSize are 65535.
		// If values less than 65535 are used, then BDP and dynamic windows are enabled.
		// Here, we disable the BDP and dynamic windows by setting these values >= 65535.
		// We set these values to (1 << 20) * 16 as this is the largest window size that
		// the BDP estimator could ever use.
		// See: https://github.com/grpc/grpc-go/blob/master/internal/transport/bdp_estimator.go
		const maxWindowSize int32 = (1 << 20) * 16
		opts = append(opts,
			grpc.WithInitialWindowSize(maxWindowSize),
			grpc.WithInitialConnWindowSize(maxWindowSize),
		)
	}

	switch cfg.Compression {
	case "":
	case "gzip":
		opts = append(opts, grpc.WithDefaultCallOptions(grpc.UseCompressor(gzip.Name)))
	default:
		return nil, fmt.Errorf("unsupported compression option: %q", cfg.Compression)
	}

	var err error
	caData := cfg.CAData
	certData := cfg.CertData
	keyData := cfg.KeyData
	if cfg.CAFile != "" {
		if caData, err = os.ReadFile(cfg.CAFile); err != nil {
			return nil, err
		}
	}
	if cfg.CertFile != "" {
		if certData, err = os.ReadFile(cfg.CertFile); err != nil {
			return nil, err
		}
	}
	if cfg.KeyFile != "" {
		if keyData, err = os.ReadFile(cfg.KeyFile); err != nil {
			return nil, err
		}
	}

	if cfg.TLS || len(caData) > 0 || len(certData) > 0 || cfg.Token != "" {
		tlsConfig := &tls.Config{}
		if len(caData) > 0 {
			cp := x509.NewCertPool()
			if !cp.AppendCertsFromPEM(caData) {
				return nil, fmt.Errorf("credentials: failed to append certificates")
			}
			tlsConfig.RootCAs = cp
		} else {
			tlsConfig.InsecureSkipVerify = true
		}
		if len(certData) > 0 {
			if len(keyData) == 0 {
				return nil, fmt.Errorf("no key provided for client certificate")
			}
			cert, err := tls.X509KeyPair(certData, keyData)
			if err != nil {
				return nil, err
			}
			tlsConfig.Certificates = []tls.Certificate{cert}
		}
		if cfg.Token != "" {
			opts = append(opts,
				grpc.WithPerRPCCredentials(newAccessTokenCredential(cfg.Token)))
		}
		opts = append(opts, grpc.WithTransportCredentials(credentials.NewTLS(tlsConfig)))
	} else {
		opts = append(opts, grpc.WithInsecure())
	}

	dial := func(ctx context.Context, addrIn string) (conn net.Conn, err error) {
		var network, nsName, addr string

		split := strings.Split(addrIn, "://")
		if l := len(split); l == 2 {
			network = split[0]
			addr = split[1]
		} else {
			network = "tcp"
			addr = split[0]
		}

		if !strings.HasPrefix(network, "unix") {
			if !strings.ContainsRune(addr, ':') {
				addr += ":" + defaultPort
			}

			nsName, addr, err = netns.ParseAddress(addr)
			if err != nil {
				return nil, err
			}
		}

		err = netns.Do(nsName, func() (err error) {
			conn, err = (&net.Dialer{}).DialContext(ctx, network, addr)
			return
		})
		return
	}

	opts = append(opts,
		grpc.WithContextDialer(dial),

		// Allows received protobuf messages to be larger than 4MB
		grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(math.MaxInt32)),
	)

	return grpc.DialContext(ctx, cfg.Addr, opts...)
}

// DialContext connects to a gnmi service and returns a client
func DialContext(ctx context.Context, cfg *Config) (pb.GNMIClient, error) {
	grpcconn, err := DialContextConn(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to dial: %s", err)
	}
	return pb.NewGNMIClient(grpcconn), nil
}

// Dial connects to a gnmi service and returns a client
func Dial(cfg *Config) (pb.GNMIClient, error) {
	return DialContext(context.Background(), cfg)
}

// NewContext returns a new context with username and password
// metadata if they are set in cfg, as well as any other metadata
// provided.
func NewContext(ctx context.Context, cfg *Config) context.Context {
	md := map[string]string{}
	for k, v := range cfg.GRPCMetadata {
		md[k] = v
	}
	if cfg.Username != "" {
		md["username"] = cfg.Username
		md["password"] = cfg.Password
	}
	if len(md) > 0 {
		ctx = metadata.NewOutgoingContext(ctx, metadata.New(md))
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
	if subscribeOptions.Target != "" {
		if subList.Prefix == nil {
			subList.Prefix = &pb.Path{}
		}
		subList.Prefix.Target = subscribeOptions.Target
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
			SuppressRedundant: subscribeOptions.SuppressRedundant,
			HeartbeatInterval: subscribeOptions.HeartbeatInterval,
		}
	}
	return &pb.SubscribeRequest{
		Extension: subscribeOptions.Extensions,
		Request: &pb.SubscribeRequest_Subscribe{
			Subscribe: subList,
		},
	}, nil
}

// HistorySnapshotExtension returns an Extension_History for the given
// time.
func HistorySnapshotExtension(t int64) *gnmi_ext.Extension_History {
	return &gnmi_ext.Extension_History{
		History: &gnmi_ext.History{
			Request: &gnmi_ext.History_SnapshotTime{
				SnapshotTime: t,
			},
		},
	}
}

// HistoryRangeExtension returns an Extension_History for the the
// specified start and end times.
func HistoryRangeExtension(s, e int64) *gnmi_ext.Extension_History {
	return &gnmi_ext.Extension_History{
		History: &gnmi_ext.History{
			Request: &gnmi_ext.History_Range{
				Range: &gnmi_ext.TimeRange{
					Start: s,
					End:   e,
				},
			},
		},
	}
}
