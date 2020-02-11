// Copyright (c) 2017 Arista Networks, Inc.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the COPYING file.

package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"reflect"
	"strings"
	"syscall"
	"time"

	"github.com/aristanetworks/goarista/gnmi"

	"github.com/aristanetworks/glog"
	pb "github.com/openconfig/gnmi/proto/gnmi"
	"golang.org/x/sync/errgroup"
)

// TODO: Make this more clear
var help = `Usage of gnmi:
gnmi -addr [<VRF-NAME>/]ADDRESS:PORT [options...]
  capabilities
  get (origin=ORIGIN) PATH+
  subscribe (origin=ORIGIN) PATH+
  ((update|replace (origin=ORIGIN) PATH JSON|FILE)|(delete (origin=ORIGIN) PATH))+
`

func usageAndExit(s string) {
	flag.Usage()
	if s != "" {
		fmt.Fprintln(os.Stderr, s)
	}
	os.Exit(1)
}

var subscribePaths = []string{
	"Eos",
	"Kernel",
	"Smash/arp",
	"Smash/counters",
	"Smash/forwarding",
	"Smash/forwarding6",
	"Smash/hardware",
	"Smash/interface",
	"Smash/routing",
	"Smash/routing6",
	"Sysdb/bridging",
	"Sysdb/cell",
	"Sysdb/daemon",
	"Sysdb/environment",
	"Sysdb/hardware",
	"Sysdb/interface",
	"Sysdb/ip",
	"Sysdb/ip6",
	"Sysdb/l2discovery",
	"Sysdb/lag",
	"Sysdb/routing",
	"Sysdb/snmp",
	"Sysdb/sys",
}

func main() {
	cfg := &gnmi.Config{}
	flag.StringVar(&cfg.Addr, "addr", "", "Address of gNMI gRPC server with optional VRF name")
	flag.StringVar(&cfg.CAFile, "cafile", "", "Path to server TLS certificate file")
	flag.StringVar(&cfg.CertFile, "certfile", "", "Path to client TLS certificate file")
	flag.StringVar(&cfg.KeyFile, "keyfile", "", "Path to client TLS private key file")
	flag.StringVar(&cfg.Password, "password", "", "Password to authenticate with")
	flag.StringVar(&cfg.Username, "username", "", "Username to authenticate with")
	flag.StringVar(&cfg.Compression, "compression", "gzip", "Compression method. "+
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

	flag.Usage = func() {
		fmt.Fprintln(os.Stderr, help)
		flag.PrintDefaults()
	}
	flag.Parse()
	if cfg.Addr == "" {
		usageAndExit("error: address not specified")
	}

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

	signalC := make(chan os.Signal, 1)
	signal.Notify(signalC, syscall.SIGINT, syscall.SIGTERM)

	client, err := gnmi.Dial(cfg)
	if err != nil {
		glog.Fatal(err)
	}
	syncResponseSeen := make(chan struct{})

	ctx, cancel := context.WithCancel(gnmi.NewContext(context.Background(), cfg))
	respChan := make(chan *pb.SubscribeResponse)
	subscribeOptions.Paths = gnmi.SplitPaths(subscribePaths)
	var g errgroup.Group
	g.Go(func() error {
		// Wait for kill signal and a sync response
		for signalC != nil || syncResponseSeen != nil {
			select {
			case s := <-signalC:
				signalC = nil
				glog.Infof("received signal: %s", s)
			case <-syncResponseSeen:
				syncResponseSeen = nil
			}
		}
		// Then end the subscription
		cancel()
		return nil
	})
	g.Go(func() error {
		return gnmi.SubscribeErr(ctx, client, subscribeOptions, respChan)
	})
	portStatuses := make(map[string]map[string]*pb.TypedValue, len(expectedIntfs))

	var errs []error
	for resp := range respChan {
		errs = append(errs, filterLACPPortStatus(syncResponseSeen, portStatuses, resp)...)
	}
	if err := g.Wait(); err != nil {
		glog.Infof("subscribe returned: %s", err)
	}
	errs = append(errs, verifyPortStatuses(portStatuses)...)
	if len(errs) == 0 {
		glog.Info("no errors")
		return
	}
	glog.Error("saw errors")
	for _, err := range errs {
		glog.Error(err)
	}
	os.Exit(1)
}

var lacpPortStatusPrefix = []string{"Sysdb", "lag", "lacp", "status", "portStatus"}

func isLACPPortStatusDelete(notif *pb.Notification) bool {
	if len(notif.Delete) == 0 {
		return false
	}
	if len(notif.Prefix.Elem) > len(lacpPortStatusPrefix) {
		return false
	}
	// Check if Prefix is a prefix of lacpPortStatusPrefix
	for i, pe := range notif.Prefix.Elem {
		if lacpPortStatusPrefix[i] != pe.Name {
			return false
		}
	}
	return true
}

func isLACPPortStatus(notif *pb.Notification) bool {
	if isLACPPortStatusDelete(notif) {
		return true
	}
	if len(notif.Prefix.Elem) <= len(lacpPortStatusPrefix) {
		return false
	}
	// Check if lacpPortStatusPrefix is a prefix of notif.PathElem
	for i, pe := range lacpPortStatusPrefix {
		if notif.Prefix.Elem[i].Name != pe {
			return false
		}
	}
	return true
}

func filterLACPPortStatus(syncResponseSeen chan struct{}, m map[string]map[string]*pb.TypedValue,
	response *pb.SubscribeResponse) []error {
	switch resp := response.Response.(type) {
	case *pb.SubscribeResponse_Error:
		glog.Fatal(resp.Error.Message)
	case *pb.SubscribeResponse_SyncResponse:
		if !resp.SyncResponse {
			glog.Fatal("initial sync failed")
		}
		glog.Info("** sync response **")
		close(syncResponseSeen)
		return nil
	case *pb.SubscribeResponse_Update:
		if !isLACPPortStatus(resp.Update) {
			return nil
		}
		gnmi.LogSubscribeResponse(response)
		return collectPortStatus(m, resp.Update)
	}
	panic("shouldn't happen")
}

var (
	strT  = reflect.TypeOf("")
	boolT = reflect.TypeOf(false)
	objT  = reflect.TypeOf(map[string]interface{}{})
	numT  = reflect.TypeOf(json.Number(""))
)

var expectedFields = map[string]reflect.Type{
	"actorChurnState":                          strT,
	"actorCollectingDistributing/collecting":   boolT,
	"actorCollectingDistributing/distributing": boolT,
	"actorPort/key":                            numT,
	"actorPort/portId":                         objT,
	"actorPort/portPriority":                   numT,
	"actorPort/sysId":                          strT,
	"actorPort/sysPriority":                    numT,
	"actorState":                               objT,
	"actorSynchronized":                        boolT,
	"allowToWarmup":                            boolT,
	"errdisableCause":                          strT,
	"intfId":                                   strT,
	"muxSmState":                               strT,
	"name":                                     strT,
	"needToTx":                                 numT,
	"partnerChurnState":                        strT,
	"partnerCollectorMaxDelay":                 numT,
	"partnerDefaulted":                         boolT,
	"partnerPort/key":                          numT,
	"partnerPort/portId":                       objT,
	"partnerPort/portPriority":                 numT,
	"partnerPort/sysId":                        strT,
	"partnerPort/sysPriority":                  numT,
	"partnerState":                             objT,
	"portId":                                   numT,
	"rxSmReason":                               strT,
	"rxSmState":                                strT,
	"selected":                                 strT,
}

func collectPortStatus(m map[string]map[string]*pb.TypedValue, notif *pb.Notification) []error {
	var errs []error
	for _, d := range notif.Delete {
		intf := gnmi.StrPath(d)[1:]
		glog.Infof("saw delete of %s", intf)
		if strings.HasPrefix(intf, "Ethernet") {
			// interface deleted, mark that as a nil map
			m[intf] = nil
		}
	}

	prefix := notif.Prefix.Elem
	intf := prefix[len(prefix)-1].Name
	if !strings.HasPrefix(intf, "Ethernet") {
		glog.Errorf("expected last prefix element to have Ethernet, but saw %s", intf)
		return errs
	}
	for _, u := range notif.Update {
		field := gnmi.StrPath(u.Path)[1:]
		expectedT, ok := expectedFields[field]
		if !ok {
			errs = append(errs, fmt.Errorf("unexpected field: %s", field))
			continue
		}
		v, err := gnmi.ExtractValue(u)
		if err != nil {
			errs = append(errs, fmt.Errorf("extract value error: %s", err))
			continue
		}
		if reflect.TypeOf(v) != expectedT {
			errs = append(errs,
				fmt.Errorf("unexpected type for field %s. expected %s got %#v",
					field, expectedT, v))
		}
		portStatus, ok := m[intf]
		if !ok {
			portStatus = make(map[string]*pb.TypedValue, 29)
			m[intf] = portStatus
		}
		// if _, ok := portStatus[field]; ok {
		// 	fmt.Println("seeing field again:", field)
		// }
		portStatus[field] = u.Val
	}
	return errs
}

func verifyPortStatuses(m map[string]map[string]*pb.TypedValue) []error {
	var errs []error
	if len(m) != len(expectedIntfs) {
		errs = append(errs, fmt.Errorf("unexpected intf count. Saw %d, expected %d",
			len(m), len(expectedIntfs)))
	}
	for _, intf := range expectedIntfs {
		portStatus, ok := m[intf]
		if !ok {
			errs = append(errs, fmt.Errorf("missing intf %s", intf))
			continue
		}
		if len(portStatus) != len(expectedFields) {
			errs = append(errs, fmt.Errorf("missing some fields for intf %s: %v", intf, portStatus))
		}
		delete(m, intf)
	}
	for intf := range m {
		errs = append(errs, fmt.Errorf("saw extra intf: %s", intf))
	}
	return errs
}

// ghs252
var expectedIntfs = []string{
	"Ethernet1/1",
	"Ethernet1/3",
	"Ethernet1/4",
	"Ethernet10/1",
	"Ethernet10/2",
	"Ethernet10/3",
	"Ethernet10/4",
	"Ethernet11/1",
	"Ethernet11/2",
	"Ethernet11/3",
	"Ethernet11/4",
	"Ethernet12/1",
	"Ethernet12/2",
	"Ethernet12/3",
	"Ethernet12/4",
	"Ethernet13/1",
	"Ethernet13/2",
	"Ethernet13/3",
	"Ethernet13/4",
	"Ethernet14/1",
	"Ethernet14/2",
	"Ethernet14/3",
	"Ethernet15/1",
	"Ethernet15/2",
	"Ethernet15/3",
	"Ethernet18/1",
	"Ethernet18/2",
	"Ethernet18/3",
	"Ethernet18/4",
	"Ethernet19/1",
	"Ethernet19/2",
	"Ethernet19/3",
	"Ethernet19/4",
	"Ethernet2/1",
	"Ethernet2/2",
	"Ethernet2/3",
	"Ethernet2/4",
	"Ethernet20/1",
	"Ethernet20/2",
	"Ethernet20/3",
	"Ethernet20/4",
	"Ethernet21/1",
	"Ethernet21/2",
	"Ethernet21/3",
	"Ethernet21/4",
	"Ethernet22/1",
	"Ethernet22/2",
	"Ethernet22/3",
	"Ethernet23/1",
	"Ethernet23/2",
	"Ethernet23/3",
	"Ethernet3/1",
	"Ethernet3/2",
	"Ethernet3/3",
	"Ethernet3/4",
	"Ethernet4/1",
	"Ethernet4/2",
	"Ethernet4/3",
	"Ethernet4/4",
	"Ethernet5/1",
	"Ethernet5/2",
	"Ethernet5/3",
	"Ethernet5/4",
	"Ethernet6/1",
	"Ethernet6/2",
	"Ethernet6/3",
	"Ethernet6/4",
	"Ethernet7/1",
	"Ethernet7/2",
	"Ethernet7/3",
	"Ethernet7/4",
}

// var expectedIntfs_tg259_1 = []string{
// 	"Ethernet3/1/1",
// 	"Ethernet3/1/2",
// 	"Ethernet3/10/1",
// 	"Ethernet3/10/2",
// 	"Ethernet3/10/3",
// 	"Ethernet3/10/4",
// 	"Ethernet3/11/1",
// 	"Ethernet3/11/2",
// 	"Ethernet3/11/3",
// 	"Ethernet3/11/4",
// 	"Ethernet3/12/1",
// 	"Ethernet3/12/2",
// 	"Ethernet3/12/3",
// 	"Ethernet3/12/4",
// 	"Ethernet3/14/1",
// 	"Ethernet3/14/2",
// 	"Ethernet3/19/1",
// 	"Ethernet3/19/2",
// 	"Ethernet3/19/3",
// 	"Ethernet3/19/4",
// 	"Ethernet3/20/1",
// 	"Ethernet3/20/2",
// 	"Ethernet3/20/3",
// 	"Ethernet3/20/4",
// 	"Ethernet3/21/1",
// 	"Ethernet3/21/2",
// 	"Ethernet3/21/3",
// 	"Ethernet3/21/4",
// 	"Ethernet3/22/1",
// 	"Ethernet3/22/2",
// 	"Ethernet3/22/3",
// 	"Ethernet3/22/4",
// 	"Ethernet3/23/1",
// 	"Ethernet3/23/2",
// 	"Ethernet3/23/3",
// 	"Ethernet3/23/4",
// 	"Ethernet3/24/1",
// 	"Ethernet3/24/2",
// 	"Ethernet3/24/3",
// 	"Ethernet3/24/4",
// 	"Ethernet3/25/1",
// 	"Ethernet3/25/2",
// 	"Ethernet3/25/3",
// 	"Ethernet3/25/4",
// 	"Ethernet3/26/1",
// 	"Ethernet3/26/2",
// 	"Ethernet3/26/3",
// 	"Ethernet3/26/4",
// 	"Ethernet3/27/1",
// 	"Ethernet3/27/2",
// 	"Ethernet3/27/4",
// 	"Ethernet3/28/1",
// 	"Ethernet3/28/2",
// 	"Ethernet3/28/3",
// 	"Ethernet3/28/4",
// 	"Ethernet3/29/1",
// 	"Ethernet3/29/2",
// 	"Ethernet3/29/3",
// 	"Ethernet3/29/4",
// 	"Ethernet3/30/1",
// 	"Ethernet3/30/2",
// 	"Ethernet3/30/4",
// 	"Ethernet3/6/1",
// 	"Ethernet3/6/2",
// 	"Ethernet3/7/1",
// 	"Ethernet3/7/2",
// 	"Ethernet3/7/3",
// 	"Ethernet3/7/4",
// 	"Ethernet3/9/1",
// 	"Ethernet3/9/2",
// 	"Ethernet4/1/1",
// 	"Ethernet4/1/2",
// 	"Ethernet4/10/1",
// 	"Ethernet4/10/2",
// 	"Ethernet4/10/3",
// 	"Ethernet4/10/4",
// 	"Ethernet4/11/1",
// 	"Ethernet4/11/2",
// 	"Ethernet4/11/3",
// 	"Ethernet4/11/4",
// 	"Ethernet4/12/1",
// 	"Ethernet4/12/2",
// 	"Ethernet4/12/3",
// 	"Ethernet4/12/4",
// 	"Ethernet4/13/1",
// 	"Ethernet4/13/3",
// 	"Ethernet4/13/4",
// 	"Ethernet4/14/1",
// 	"Ethernet4/18/1",
// 	"Ethernet4/18/3",
// 	"Ethernet4/19/1",
// 	"Ethernet4/19/2",
// 	"Ethernet4/19/3",
// 	"Ethernet4/19/4",
// 	"Ethernet4/2/1",
// 	"Ethernet4/20/1",
// 	"Ethernet4/20/2",
// 	"Ethernet4/20/3",
// 	"Ethernet4/20/4",
// 	"Ethernet4/21/1",
// 	"Ethernet4/21/2",
// 	"Ethernet4/21/3",
// 	"Ethernet4/21/4",
// 	"Ethernet4/22/1",
// 	"Ethernet4/22/2",
// 	"Ethernet4/22/3",
// 	"Ethernet4/22/4",
// 	"Ethernet4/23/1",
// 	"Ethernet4/23/2",
// 	"Ethernet4/23/3",
// 	"Ethernet4/23/4",
// 	"Ethernet4/24/1",
// 	"Ethernet4/24/2",
// 	"Ethernet4/24/3",
// 	"Ethernet4/24/4",
// 	"Ethernet4/25/1",
// 	"Ethernet4/25/2",
// 	"Ethernet4/25/3",
// 	"Ethernet4/25/4",
// 	"Ethernet4/26/1",
// 	"Ethernet4/26/2",
// 	"Ethernet4/26/3",
// 	"Ethernet4/26/4",
// 	"Ethernet4/27/1",
// 	"Ethernet4/27/2",
// 	"Ethernet4/27/4",
// 	"Ethernet4/28/1",
// 	"Ethernet4/28/2",
// 	"Ethernet4/28/3",
// 	"Ethernet4/28/4",
// 	"Ethernet4/29/1",
// 	"Ethernet4/29/2",
// 	"Ethernet4/29/3",
// 	"Ethernet4/29/4",
// 	"Ethernet4/30/1",
// 	"Ethernet4/30/2",
// 	"Ethernet4/30/3",
// 	"Ethernet4/30/4",
// 	"Ethernet4/31/1",
// 	"Ethernet4/31/2",
// 	"Ethernet4/31/3",
// 	"Ethernet4/31/4",
// 	"Ethernet4/7/1",
// 	"Ethernet4/7/2",
// 	"Ethernet4/7/3",
// 	"Ethernet4/7/4",
// 	"Ethernet4/9/1",
// 	"Ethernet4/9/2",
// 	"Ethernet5/1/2",
// 	"Ethernet5/10/1",
// 	"Ethernet5/10/2",
// 	"Ethernet5/10/3",
// 	"Ethernet5/11/1",
// 	"Ethernet5/11/2",
// 	"Ethernet5/11/4",
// 	"Ethernet5/12/1",
// 	"Ethernet5/12/2",
// 	"Ethernet5/12/3",
// 	"Ethernet5/12/4",
// 	"Ethernet5/3/1",
// 	"Ethernet5/3/2",
// 	"Ethernet5/4/4",
// 	"Ethernet5/5/1",
// 	"Ethernet5/5/2",
// 	"Ethernet5/5/3",
// 	"Ethernet5/5/4",
// 	"Ethernet5/6/1",
// 	"Ethernet5/6/2",
// 	"Ethernet5/6/3",
// 	"Ethernet5/6/4",
// 	"Ethernet5/7/1",
// 	"Ethernet5/7/2",
// 	"Ethernet5/7/3",
// 	"Ethernet5/7/4",
// 	"Ethernet5/8/1",
// 	"Ethernet5/8/2",
// 	"Ethernet5/8/3",
// 	"Ethernet5/8/4",
// 	"Ethernet5/9/1",
// 	"Ethernet5/9/2",
// 	"Ethernet5/9/3",
// 	"Ethernet5/9/4",
// 	"Ethernet6/1/1",
// 	"Ethernet6/1/2",
// 	"Ethernet6/2/7",
// 	"Ethernet6/3/1",
// 	"Ethernet6/3/2",
// 	"Ethernet6/3/3",
// 	"Ethernet6/3/4",
// 	"Ethernet6/3/5",
// 	"Ethernet6/3/6",
// 	"Ethernet6/3/7",
// 	"Ethernet6/4/1",
// 	"Ethernet6/4/2",
// 	"Ethernet6/4/3",
// 	"Ethernet6/4/4",
// 	"Ethernet6/4/5",
// 	"Ethernet6/4/6",
// 	"Ethernet6/4/7",
// 	"Ethernet6/5/10",
// 	"Ethernet6/5/4",
// 	"Ethernet6/5/5",
// 	"Ethernet6/5/6",
// 	"Ethernet6/5/7",
// 	"Ethernet6/5/8",
// 	"Ethernet6/5/9",
// 	"Ethernet6/6/1",
// 	"Ethernet6/6/2",
// 	"Ethernet6/6/4",
// 	"Ethernet6/6/5",
// 	"Ethernet6/6/6",
// }
