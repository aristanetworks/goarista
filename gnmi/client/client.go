// Copyright (c) 2017 Arista Networks, Inc.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the COPYING file.

package client

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"runtime/debug"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	aflag "github.com/aristanetworks/goarista/flag"
	"github.com/aristanetworks/goarista/gnmi"

	"github.com/aristanetworks/glog"
	pb "github.com/openconfig/gnmi/proto/gnmi"
	"github.com/openconfig/gnmi/proto/gnmi_ext"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/protobuf/proto"
)

// TODO: Make this more clear
var help = `Usage of gnmi:
gnmi -addr [<VRF-NAME>/]ADDRESS:PORT [options...]
  capabilities
  get ((encoding=ENCODING) (origin=ORIGIN) (target=TARGET) PATH+)+
  subscribe ((origin=ORIGIN) (target=TARGET) PATH+)+ 
  ((update|replace|union_replace (origin=ORIGIN) (target=TARGET) PATH JSON|FILE) |
   (delete (origin=ORIGIN) (target=TARGET) PATH))+
`

type reqParams struct {
	encoding string
	origin   string
	target   string
	paths    []string
}

func usageAndExit(s string) {
	flag.Usage()
	if s != "" {
		fmt.Fprintln(os.Stderr, s)
	}
	os.Exit(1)
}

// Main initializes the gNMI client.
func Main() {
	cfg := &gnmi.Config{}
	flag.StringVar(&cfg.Addr, "addr", "", "Address of gNMI gRPC server with optional VRF name")
	flag.StringVar(&cfg.CAFile, "cafile", "", "Path to server TLS certificate file")
	flag.StringVar(&cfg.CertFile, "certfile", "", "Path to client TLS certificate file")
	flag.StringVar(&cfg.KeyFile, "keyfile", "", "Path to client TLS private key file")
	flag.StringVar(&cfg.Password, "password", "", "Password to authenticate with")
	flag.StringVar(&cfg.Username, "username", "", "Username to authenticate with")
	flag.StringVar(&cfg.Compression, "compression", "", "Compression method. "+
		`Supported options: "" and "gzip"`)
	flag.BoolVar(&cfg.TLS, "tls", false, "Enable TLS")
	flag.StringVar(&cfg.TLSMinVersion, "tls-min-version", "",
		fmt.Sprintf("Set minimum TLS version for connection (%s)", gnmi.TLSVersions))
	flag.StringVar(&cfg.TLSMaxVersion, "tls-max-version", "",
		fmt.Sprintf("Set maximum TLS version for connection (%s)", gnmi.TLSVersions))
	flag.BoolVar(&cfg.BDP, "bdp", true,
		"Enable Bandwidth Delay Product (BDP) estimation and dynamic flow control window")
	outputVersion := flag.Bool("version", false, "print version information")

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
	historyStartStr := flag.String("history_start", "", "Historical data subscription "+
		"start time (nanoseconds since Unix epoch or RFC3339 format with nanosecond "+
		"precision, e.g., 2006-01-02T15:04:05.999999999+07:00)")
	historyEndStr := flag.String("history_end", "", "Historical data subscription "+
		"end time (nanoseconds since Unix epoch or RFC3339 format with nanosecond "+
		"precision, e.g., 2006-01-02T15:04:05.999999999+07:00)")
	historySnapshotStr := flag.String("history_snapshot", "", "Historical data subscription "+
		"snapshot time (nanoseconds since Unix epoch or RFC3339 format with nanosecond "+
		"precision, e.g., 2006-01-02T15:04:05.999999999+07:00)")
	dataTypeStr := flag.String("data_type", "all",
		"Get data type (all | config | state | operational)")
	flag.StringVar(&cfg.Token, "token", "", "Authentication token")
	grpcMetadata := aflag.Map{}
	flag.Var(grpcMetadata, "grpcmetadata",
		"key=value gRPC metadata fields, can be used repeatedly")

	debugMode := flag.String("debug", "", "Enable a debug mode:\n"+
		"  'proto' : print SubscribeResponses in protobuf text format\n"+
		"  'latency' : print timing numbers to help debug latency\n"+
		"  'throughput' : print number of notifications sent in a second\n"+
		"  'clog' : start a subscribe and then don't read any of the responses")

	keepaliveTimeStr := flag.String("keepalive_time", "", "Keepalive ping interval. "+
		"After inactivity of this duration, ping the server (30s, 2m, etc. Default 10s). "+
		"10s is the minimum value allowed. If a value less than 10s is supplied, 10s will be used")

	flag.Usage = func() {
		fmt.Fprintln(os.Stderr, help)
		flag.PrintDefaults()
	}
	flag.Parse()
	if *outputVersion {
		if info, ok := debug.ReadBuildInfo(); ok {
			for _, data := range info.Settings {
				if data.Key == "vcs.revision" {
					fmt.Printf("%s_%s", data.Value, info.GoVersion)
				}
			}
		} else {
			fmt.Printf("version information only available in go 1.18+")
		}
		return
	}

	if cfg.Addr == "" {
		usageAndExit("error: address not specified")
	}
	cfg.GRPCMetadata = grpcMetadata

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

	var histExt *gnmi_ext.Extension_History
	if *historyStartStr != "" || *historyEndStr != "" || *historySnapshotStr != "" {
		if *historySnapshotStr != "" {
			if *historyStartStr != "" || *historyEndStr != "" {
				usageAndExit("error: specified history start/end and snapshot time")
			}
			t, err := parseTime(*historySnapshotStr)
			if err != nil {
				usageAndExit(fmt.Sprintf("error: invalid snapshot time (%s): %s",
					*historySnapshotStr, err))
			}
			histExt = gnmi.HistorySnapshotExtension(t.UnixNano())
		} else {
			var s, e int64
			if *historyStartStr != "" {
				st, err := parseTime(*historyStartStr)
				if err != nil {
					usageAndExit(fmt.Sprintf("error: invalid start time (%s): %s",
						*historyStartStr, err))
				}
				s = st.UnixNano()
			}
			if *historyEndStr != "" {
				et, err := parseTime(*historyEndStr)
				if err != nil {
					usageAndExit(fmt.Sprintf("error: invalid end time (%s): %s",
						*historyEndStr, err))
				}
				e = et.UnixNano()
			}
			histExt = gnmi.HistoryRangeExtension(s, e)
		}
	}

	if *keepaliveTimeStr != "" {
		var keepaliveTime time.Duration
		var err error
		if keepaliveTime, err = time.ParseDuration(*keepaliveTimeStr); err != nil {
			usageAndExit(fmt.Sprintf("error: keepalive time (%s) invalid", *keepaliveTimeStr))
		}

		timeout := time.Duration(keepaliveTime * time.Second)
		cfg.DialOptions = append(cfg.DialOptions,
			grpc.WithKeepaliveParams(keepalive.ClientParameters{Time: timeout}))
	}

	args := flag.Args()

	ctx := gnmi.NewContext(context.Background(), cfg)
	client, err := gnmi.Dial(cfg)
	if err != nil {
		glog.Fatal(err)
	}

	var setOps []*gnmi.Operation
	for i := 0; i < len(args); i++ {
		op := args[i]
		switch op {
		case "capabilities":
			if len(setOps) != 0 {
				usageAndExit("error: 'capabilities' not allowed after" +
					" 'update|replace|delete|union_replace'")
			}
			err := gnmi.Capabilities(ctx, client)
			if err != nil {
				glog.Fatal(err)
			}
			return
		case "get":
			if len(setOps) != 0 {
				usageAndExit("error: 'get' not allowed after" +
					" 'update|replace|delete|union_replace'")
			}
			pathParams, argsParsed := parsereqParams(args[1:], false)
			if argsParsed == 0 {
				usageAndExit("error: missing path")
			}
			for _, pathParam := range pathParams {
				req, err := newGetRequest(pathParam, *dataTypeStr)
				if err != nil {
					usageAndExit("error: " + err.Error())
				}

				err = gnmi.GetWithRequest(ctx, client, req)
				if err != nil {
					glog.Fatal(err)
				}
			}

			return
		case "subscribe":
			if len(setOps) != 0 {
				usageAndExit("error: 'subscribe' not allowed after" +
					" 'update|replace|delete|union_replace'")
			}
			var g errgroup.Group
			pathParams, argsParsed := parsereqParams(args[1:], false)
			if argsParsed == 0 {
				usageAndExit("error: missing path")
			}
			for _, pathParam := range pathParams {
				subOptions, err := newSubscribeOptions(pathParam, histExt, subscribeOptions)
				if err != nil {
					usageAndExit("error: " + err.Error())
				}

				respChan := make(chan *pb.SubscribeResponse)
				g.Go(func() error {
					return gnmi.SubscribeErr(ctx, client, subOptions, respChan)
				})
				switch *debugMode {
				case "proto":
					for resp := range respChan {
						fmt.Println(resp)
					}
				case "latency":
					for resp := range respChan {
						printLatencyStats(resp)
					}
				case "throughput":
					handleThroughput(respChan)
				case "clog":
					// Don't read any subscription updates
					g.Wait()
				case "":
					go processSubscribeResponses(pathParam.origin, respChan)

				default:
					usageAndExit(fmt.Sprintf("unknown debug option: %q", *debugMode))
				}
			}
			if err := g.Wait(); err != nil {
				glog.Fatal(err)
			}
			return
		case "update", "replace", "delete", "union_replace":
			j, op, err := newSetOperation(i, args, *arbitrationStr)
			if err != nil {
				usageAndExit("error: " + err.Error())
			}
			if op != nil {
				setOps = append(setOps, op)
			}
			i = j
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

func newSetOperation(
	index int,
	args []string,
	arbitrationStr string) (int, *gnmi.Operation, error) {

	// ok if no args, if arbitration was specified
	if len(args) == index+1 && arbitrationStr == "" {
		return 0, nil, errors.New("missing path")
	}

	op := &gnmi.Operation{
		Type: args[index],
	}
	index++
	if len(args) <= index {
		return index, nil, nil
	}

	pathParams, argsParsed := parsereqParams(args[index:], true)
	index += argsParsed

	// process update | replace | delete | union_replace request one at a time
	pathParam := pathParams[0]

	// check that encoding is not set
	if pathParam.encoding != "" {
		return 0, nil, fmt.Errorf("encoding option is not supported for '%s'", op.Type)
	}

	if len(pathParam.paths) == 0 {
		return 0, nil, fmt.Errorf("missing path for '%s'", op.Type)
	}

	op.Path = gnmi.SplitPath(pathParam.paths[0])
	op.Origin = pathParam.origin
	op.Target = pathParam.target
	if op.Type == "delete" {
		// set index to be right before the next arg that needs to be processed
		index--
	} else {
		// no need for index-- since the value of update/replace/union_replace is right
		// before the next arg
		if len(args) == index {
			return 0, nil, errors.New("missing JSON or FILEPATH to data")
		}
		op.Val = args[index]
	}

	return index, op, nil
}

func newSubscribeOptions(
	pathParam reqParams,
	histExt *gnmi_ext.Extension_History,
	subscribeOptions *gnmi.SubscribeOptions) (*gnmi.SubscribeOptions, error) {

	origin := pathParam.origin
	target := pathParam.target
	paths := pathParam.paths
	encoding := pathParam.encoding

	// check that encoding is not set
	if encoding != "" {
		return nil, errors.New("encoding option is not supported for 'subscribe'")
	}

	subOptions := new(gnmi.SubscribeOptions)
	*subOptions = *subscribeOptions
	subOptions.Origin = origin
	subOptions.Target = target
	subOptions.Paths = gnmi.SplitPaths(paths)
	if histExt != nil {
		subOptions.Extensions = []*gnmi_ext.Extension{{
			Ext: histExt,
		}}
	}

	return subOptions, nil
}

func newGetRequest(pathParam reqParams, dataTypeStr string) (*pb.GetRequest, error) {
	origin := pathParam.origin
	target := pathParam.target
	paths := pathParam.paths
	encoding := pathParam.encoding

	req, err := gnmi.NewGetRequest(gnmi.SplitPaths(paths), origin)
	if err != nil {
		glog.Fatal(err)
	}

	// set target
	if target != "" {
		if req.Prefix == nil {
			req.Prefix = &pb.Path{}
		}
		req.Prefix.Target = target
	}

	// set type
	switch strings.ToLower(dataTypeStr) {
	case "", "all":
		req.Type = pb.GetRequest_ALL
	case "config":
		req.Type = pb.GetRequest_CONFIG
	case "state":
		req.Type = pb.GetRequest_STATE
	case "operational":
		req.Type = pb.GetRequest_OPERATIONAL
	default:
		return nil, fmt.Errorf("invalid data type (%s)", dataTypeStr)
	}

	// set encoding
	switch en := strings.ToLower(encoding); en {
	case "ascii":
		req.Encoding = pb.Encoding_ASCII
	case "json":
		req.Encoding = pb.Encoding_JSON
	case "json_ietf":
		req.Encoding = pb.Encoding_JSON_IETF
	case "proto":
		req.Encoding = pb.Encoding_PROTO
	case "bytes":
		req.Encoding = pb.Encoding_BYTES
	case "":
	default:
		return nil, fmt.Errorf(
			`invalid encoding '%s'
Supported encodings are (case insensitive):
- JSON
- Bytes
- Proto
- ASCII
- JSON_IETF`, en)
	}

	return req, nil
}

func processSubscribeResponses(origin string, respChan chan *pb.SubscribeResponse) {
	for resp := range respChan {
		if err := gnmi.LogSubscribeResponse(resp); err != nil {
			glog.Fatal(err)
		}
	}
}

// Parse string timestamp, first trying for ns since epoch, and then
// for RFC3339.
func parseTime(ts string) (time.Time, error) {
	if ti, err := strconv.ParseInt(ts, 10, 64); err == nil {
		return time.Unix(0, ti), nil
	}
	return time.Parse(time.RFC3339Nano, ts)
}

func parseStringOpt(s, prefix string) (string, bool) {
	if strings.HasPrefix(s, prefix+"=") {
		return strings.TrimPrefix(s, prefix+"="), true
	}
	return "", false
}

func parseOrigin(s string) (string, bool) {
	return parseStringOpt(s, "origin")
}

func parseTarget(s string) (string, bool) {
	return parseStringOpt(s, "target")
}

func parseEncoding(s string) (string, bool) {
	return parseStringOpt(s, "encoding")
}

func parsereqParams(args []string, maxOnePath bool) ([]reqParams, int) {
	var pathParam *reqParams = new(reqParams)
	var pathParams []reqParams
	var argsParsed int

	//           [[ Some Considerations ]]
	// - No need to follow encoding - origin - target - path as names are specified.
	// - PATHS+ still mark the end of a pathParam
	// - only path is required and everything else is optional
	// - there can be one or more paths
	// - there can be zero or one encoding, origin, and target.

	var isOriginSet bool
	var isTargetSet bool
	var isEncodingSet bool

	// check if the current config forms a pathParam
	// If yes, reset all the trackers and add it to pathParams
	// otherwise, don't do anything
	var checkGroup = func(isItemSet bool) {
		if isItemSet || len(pathParam.paths) > 0 {
			isOriginSet = false
			isTargetSet = false
			isEncodingSet = false
			pathParams = append(pathParams, *pathParam)
			pathParam = new(reqParams)
		}
	}

	for _, arg := range args {
		argsParsed++
		if o, ok := parseOrigin(arg); ok {
			checkGroup(isOriginSet)
			pathParam.origin = o
			isOriginSet = true
		} else if t, ok := parseTarget(arg); ok {
			checkGroup(isTargetSet)
			pathParam.target = t
			isTargetSet = true
		} else if e, ok := parseEncoding(arg); ok {
			checkGroup(isEncodingSet)
			pathParam.encoding = e
			isEncodingSet = true
		} else {
			pathParam.paths = append(pathParam.paths, arg)
			if maxOnePath {
				break
			}
		}
	}

	// The last pathParam wasn't added, add it now
	pathParams = append(pathParams, *pathParam)

	// validate that all reqParams have a valid path
	for _, param := range pathParams {
		if param.paths == nil {
			return pathParams, 0 // no path provided
		}
	}

	return pathParams, argsParsed
}

func printLatencyStats(s *pb.SubscribeResponse) {
	switch resp := s.Response.(type) {
	case *pb.SubscribeResponse_SyncResponse:
		fmt.Printf("now=%d sync_response=%t\n",
			time.Now().UnixNano(), resp.SyncResponse)
	case *pb.SubscribeResponse_Update:
		notif := resp.Update
		now := time.Now().UnixNano()
		fmt.Printf("now=%d timestamp=%d latency=%s prefix=%s size=%d updates=%d deletes=%d\n",
			now,
			notif.Timestamp,
			time.Duration(now-notif.Timestamp),
			gnmi.StrPath(notif.Prefix),
			proto.Size(s),
			len(notif.Update),
			len(notif.Delete),
		)
	}
}

func handleThroughput(respChan <-chan *pb.SubscribeResponse) {
	var notifs uint64
	var updates uint64
	go func() {
		var (
			lastNotifs  uint64
			lastUpdates uint64
			lastTime    = time.Now()
		)
		ticker := time.NewTicker(10 * time.Second)
		for t := range ticker.C {
			newNotifs := atomic.LoadUint64(&notifs)
			newUpdates := atomic.LoadUint64(&updates)
			dNotifs := newNotifs - lastNotifs
			dUpdates := newUpdates - lastUpdates
			since := t.Sub(lastTime).Seconds()
			lastNotifs = newNotifs
			lastUpdates = newUpdates
			lastTime = t
			fmt.Printf("%s: %f notifs/s %f updates/s\n",
				t, float64(dNotifs)/since, float64(dUpdates)/since)
		}
	}()

	for resp := range respChan {
		r, ok := resp.Response.(*pb.SubscribeResponse_Update)
		if !ok {
			continue
		}
		notif := r.Update
		atomic.AddUint64(&notifs, 1)
		atomic.AddUint64(&updates, uint64(len(notif.Update)+len(notif.Delete)))
	}
	return
}
