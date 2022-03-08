// Copyright (c) 2017 Arista Networks, Inc.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the COPYING file.

package client

import (
	"context"
	"flag"
	"fmt"
	"os"
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
	"google.golang.org/protobuf/proto"
)

// TODO: Make this more clear
var help = `Usage of gnmi:
gnmi -addr [<VRF-NAME>/]ADDRESS:PORT [options...]
  capabilities
  get (origin=ORIGIN) (target=TARGET) PATH+
  subscribe (origin=ORIGIN) (target=TARGET) PATH+
  ((update|replace (origin=ORIGIN) (target=TARGET) PATH JSON|FILE) |
   (delete (origin=ORIGIN) (target=TARGET) PATH))+
`

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

	debug := flag.String("debug", "", "Enable a debug mode:\n"+
		"  'proto' : print SubscribeResponses in protobuf text format\n"+
		"  'latency' : print timing numbers to help debug latency\n"+
		"  'throughput' : print number of notifications sent in a second\n"+
		"  'clog' : start a subscribe and then don't read any of the responses")

	flag.Usage = func() {
		fmt.Fprintln(os.Stderr, help)
		flag.PrintDefaults()
	}
	flag.Parse()
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
			origin, target, paths, _ := parseOriginTargetPaths(args[1:], false)
			req, err := gnmi.NewGetRequest(gnmi.SplitPaths(paths), origin)
			if err != nil {
				glog.Fatal(err)
			}
			if target != "" {
				if req.Prefix == nil {
					req.Prefix = &pb.Path{}
				}
				req.Prefix.Target = target
			}
			switch strings.ToLower(*dataTypeStr) {
			case "", "all":
				req.Type = pb.GetRequest_ALL
			case "config":
				req.Type = pb.GetRequest_CONFIG
			case "state":
				req.Type = pb.GetRequest_STATE
			case "operational":
				req.Type = pb.GetRequest_OPERATIONAL
			default:
				usageAndExit(fmt.Sprintf("error: invalid data type (%s)", *dataTypeStr))
			}

			err = gnmi.GetWithRequest(ctx, client, req)
			if err != nil {
				glog.Fatal(err)
			}
			return
		case "subscribe":
			if len(setOps) != 0 {
				usageAndExit("error: 'subscribe' not allowed after 'merge|replace|delete'")
			}
			origin, target, paths, _ := parseOriginTargetPaths(args[1:], false)
			respChan := make(chan *pb.SubscribeResponse)
			subscribeOptions.Origin = origin
			subscribeOptions.Target = target
			subscribeOptions.Paths = gnmi.SplitPaths(paths)
			if histExt != nil {
				subscribeOptions.Extensions = []*gnmi_ext.Extension{{
					Ext: histExt,
				}}
			}
			var g errgroup.Group
			g.Go(func() error {
				return gnmi.SubscribeErr(ctx, client, subscribeOptions, respChan)
			})
			switch *debug {
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
				for resp := range respChan {
					if err := gnmi.LogSubscribeResponse(resp); err != nil {
						glog.Fatal(err)
					}
				}
			default:
				usageAndExit(fmt.Sprintf("unknown debug option: %q", *debug))
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
			origin, target, paths, argsParsed := parseOriginTargetPaths(args[i:], true)
			i += argsParsed
			op.Path = gnmi.SplitPath(paths[0])
			op.Origin = origin
			op.Target = target
			if op.Type != "delete" {
				if len(args) == i {
					usageAndExit("error: missing JSON or FILEPATH to data")
				}
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

func parseOriginTargetPaths(args []string, maxOnePath bool) (origin,
	target string, paths []string, argsParsed int) {
	for i, arg := range args {
		argsParsed++
		if i < 2 {
			if o, ok := parseOrigin(arg); ok {
				origin = o
				continue
			}
			if t, ok := parseTarget(arg); ok {
				target = t
				continue
			}
		}
		paths = append(paths, arg)
		if len(paths) > 0 && maxOnePath {
			return
		}
	}
	return
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
