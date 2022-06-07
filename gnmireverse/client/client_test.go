// Copyright (c) 2020 Arista Networks, Inc.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the COPYING file.

package client

import (
	"context"
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/aristanetworks/glog"
	"github.com/aristanetworks/goarista/gnmireverse"
	"github.com/openconfig/gnmi/proto/gnmi"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
)

func TestSampleList(t *testing.T) {
	for name, tc := range map[string]struct {
		arg string

		error    bool
		path     *gnmi.Path
		interval time.Duration
	}{
		"working": {
			arg: "/foos/foo[name=bar]/baz@30s",

			path: &gnmi.Path{Elem: []*gnmi.PathElem{
				&gnmi.PathElem{Name: "foos"},
				&gnmi.PathElem{Name: "foo",
					Key: map[string]string{"name": "bar"}},
				&gnmi.PathElem{Name: "baz"},
			}},
			interval: 30 * time.Second,
		},
		"no_interval": {
			arg:   "/foos/foo[name=bar]/baz",
			error: true,
		},
		"empty_interval": {
			arg:   "/foos/foo[name=bar]/baz@",
			error: true,
		},
		"invalid_path": {
			arg:   "/foos/foo[name=bar]]/baz@30s",
			error: true,
		},
	} {
		t.Run(name, func(t *testing.T) {
			var l sampleList
			err := l.Set(tc.arg)
			if err != nil {
				if !tc.error {
					t.Fatalf("unexpected error: %s", err)
				}
				return
			} else if tc.error {
				t.Fatal("expected error and didn't get one")
			}

			sub := l.subs[0]
			sub.p.Element = nil // Ignore the backward compatible path
			if !proto.Equal(tc.path, sub.p) {
				t.Errorf("Paths don't match. Expected: %s Got: %s",
					tc.path, sub.p)
			}
			if tc.interval != sub.interval {
				t.Errorf("Intervals don't match. Expected %s Got: %s",
					tc.interval, sub.interval)
			}
			str := l.String()
			if tc.arg != str {
				t.Errorf("Unexpected String() result: Expected: %q Got: %q", tc.arg, str)
			}
		})
	}
}

func TestSubscriptionList(t *testing.T) {
	for name, tc := range map[string]struct {
		arg string

		error    bool
		path     *gnmi.Path
		interval time.Duration
	}{
		"working": {
			arg: "/foos/foo[name=bar]/baz@30s",

			path: &gnmi.Path{Elem: []*gnmi.PathElem{
				&gnmi.PathElem{Name: "foos"},
				&gnmi.PathElem{Name: "foo",
					Key: map[string]string{"name": "bar"}},
				&gnmi.PathElem{Name: "baz"},
			}},
			interval: 30 * time.Second,
		},
		"no_interval": {
			arg: "/foos/foo[name=bar]/baz",
			path: &gnmi.Path{Elem: []*gnmi.PathElem{
				&gnmi.PathElem{Name: "foos"},
				&gnmi.PathElem{Name: "foo",
					Key: map[string]string{"name": "bar"}},
				&gnmi.PathElem{Name: "baz"},
			}},
		},
		"empty_interval": {
			arg:   "/foos/foo[name=bar]/baz@",
			error: true,
		},
		"invalid_path": {
			arg:   "/foos/foo[name=bar]]/baz@30s",
			error: true,
		},
	} {
		t.Run(name, func(t *testing.T) {
			var l subscriptionList
			err := l.Set(tc.arg)
			if err != nil {
				if !tc.error {
					t.Fatalf("unexpected error: %s", err)
				}
				return
			} else if tc.error {
				t.Fatal("expected error and didn't get one")
			}

			sub := l.subs[0]
			sub.p.Element = nil // Ignore the backward compatible path
			if !proto.Equal(tc.path, sub.p) {
				t.Errorf("Paths don't match. Expected: %s Got: %s",
					tc.path, sub.p)
			}
			if tc.interval != sub.interval {
				t.Errorf("Intervals don't match. Expected %s Got: %s",
					tc.interval, sub.interval)
			}
			str := l.String()
			if tc.arg != str {
				t.Errorf("Unexpected String() result: Expected: %q Got: %q", tc.arg, str)
			}
		})
	}
}

func TestStreamGetResponses(t *testing.T) {
	// Set the Get paths list.
	var cfgGetList getList
	cfgGetList.Set("/foo/bar")

	cfg := &config{
		targetVal:         "baz",
		targetAddr:        getTestAddress(t),
		collectorAddr:     getTestAddress(t),
		getPaths:          cfgGetList,
		getSampleInterval: time.Second,
	}

	collectorErrChan := make(chan error, 1)
	gnmiServer := &gnmiServer{}
	gnmireverseServer := &gnmireverseServer{
		errChan: collectorErrChan,
	}
	runStreamGetResponsesTest(t, cfg, collectorErrChan,
		gnmiServer, gnmireverseServer, streamGetResponses)
}

// getTestAddress gets a localhost address with a random unused port.
func getTestAddress(t *testing.T) string {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to find an available port: %s", err)
	}
	defer listener.Close()
	return listener.Addr().String()
}

// runStreamGetResponsesTest runs the gNMIReverse Get client with the mock gNMI server and
// mock gNMIReverse server and checks if the collectorErrChan receives an error.
func runStreamGetResponsesTest(t *testing.T, cfg *config, collectorErrChan chan error,
	gnmiServer gnmi.GNMIServer, gnmireverseServer gnmireverse.GNMIReverseServer,
	streamResponsesFunc func(*config, *grpc.ClientConn, *grpc.ClientConn) func(
		context.Context, *errgroup.Group)) {
	// Start the mock gNMI target server.
	targetGRPCServer := grpc.NewServer()
	gnmi.RegisterGNMIServer(targetGRPCServer, gnmiServer)
	targetListener, err := net.Listen("tcp", cfg.targetAddr)
	if err != nil {
		t.Fatal(err)
	}
	glog.V(1).Infof("gNMI target server listening on %s", cfg.targetAddr)
	go func() {
		if err := targetGRPCServer.Serve(targetListener); err != nil {
			t.Error(err)
		}
	}()

	// Start the mock gNMIReverse collector server.
	collectorGRPCServer := grpc.NewServer()
	gnmireverse.RegisterGNMIReverseServer(collectorGRPCServer, gnmireverseServer)
	collectorListener, err := net.Listen("tcp", cfg.collectorAddr)
	if err != nil {
		t.Fatal(err)
	}
	glog.V(1).Infof("gNMIReverse collector server listening on %s", cfg.collectorAddr)
	go func() {
		if err := collectorGRPCServer.Serve(collectorListener); err != nil {
			t.Error(err)
		}
	}()

	// Start the gNMIReverse client to stream GetResponses from target to collector.
	destConn, err := dialCollector(cfg)
	if err != nil {
		glog.Fatalf("error dialing destination %q: %s", cfg.collectorAddr, err)
	}
	targetConn, err := dialTarget(cfg)
	if err != nil {
		glog.Fatalf("error dialing target %q: %s", cfg.targetAddr, err)
	}
	glog.V(1).Infof("gNMIReverse client publish Get response from %s to %s",
		targetConn.Target(), destConn.Target())
	go func() {
		streamResponses(streamResponsesFunc(cfg, destConn, targetConn))
	}()

	// Check that the gNMIReverse collector server receives the expected Get response.
	if err := <-collectorErrChan; err != nil {
		t.Error(err)
	}
}

var testNotification = &gnmi.Notification{
	Prefix: &gnmi.Path{
		Target: "baz",
	},
	Update: []*gnmi.Update{{
		Path: &gnmi.Path{
			Elem: []*gnmi.PathElem{
				{Name: "foo"},
				{Name: "bar"},
			},
		},
		Val: &gnmi.TypedValue{
			Value: &gnmi.TypedValue_IntVal{
				IntVal: 1,
			},
		},
	}},
}

var testGetResponse = &gnmi.GetResponse{
	Notification: []*gnmi.Notification{testNotification},
}

// Mock gNMIReverse server checks if the published Get response matches the testGetResponse.
type gnmireverseServer struct {
	errChan chan error
	gnmireverse.UnimplementedGNMIReverseServer
}

func (s *gnmireverseServer) PublishGet(stream gnmireverse.GNMIReverse_PublishGetServer) error {
	for {
		res, err := stream.Recv()
		if err != nil {
			return err
		}
		// Overwrite the timestamp so notification can be compared.
		res.Notification[0].Timestamp = 0
		if !proto.Equal(testGetResponse, res) {
			s.errChan <- fmt.Errorf(
				"Get response not equal: want %v, got %v", testGetResponse, res)
		} else {
			s.errChan <- nil
		}
	}
}

// Mock gNMI server returns testGetResponse for Get.
type gnmiServer struct {
	gnmi.UnimplementedGNMIServer
}

func (*gnmiServer) Get(ctx context.Context, req *gnmi.GetRequest) (*gnmi.GetResponse, error) {
	return testGetResponse, nil
}

func TestCombineGetResponses(t *testing.T) {
	for name, tc := range map[string]struct {
		getResponses        []*gnmi.GetResponse
		combinedGetResponse *gnmi.GetResponse
	}{
		"0_notifs_0_notifs_total_0_notifs": {
			getResponses: []*gnmi.GetResponse{
				{},
				{},
			},
			combinedGetResponse: &gnmi.GetResponse{},
		},
		"1_notif_0_notif_total_1_notif": {
			getResponses: []*gnmi.GetResponse{
				testGetResponse,
				{},
			},
			combinedGetResponse: testGetResponse,
		},
		"0_notif_1_notif_total_1_notif": {
			getResponses: []*gnmi.GetResponse{
				{},
				testEOSNativeGetResponse,
			},
			combinedGetResponse: testEOSNativeGetResponse,
		},
		"1_notif_1_notif_total_2_notifs": {
			getResponses: []*gnmi.GetResponse{
				testGetResponse,
				testEOSNativeGetResponse,
			},
			combinedGetResponse: testCombinedGetResponse,
		},
	} {
		t.Run(name, func(t *testing.T) {
			timestamp := int64(123)
			target := "baz"
			combinedGetResponse := combineGetResponses(timestamp, target, tc.getResponses...)
			if !proto.Equal(tc.combinedGetResponse, combinedGetResponse) {
				t.Errorf("combined Get responses do not match, expected: %v, got: %v",
					tc.combinedGetResponse, combinedGetResponse)
			}
		})
	}
}

func TestStreamMixedOriginGetResponses(t *testing.T) {
	// Set the Get paths list with one OpenConfig and one EOS native path.
	var cfgGetList getList
	cfgGetList.Set("openconfig:/foo/bar")
	cfgGetList.Set("eos_native:/a/b")

	cfg := &config{
		targetVal:         "baz",
		targetAddr:        getTestAddress(t),
		collectorAddr:     getTestAddress(t),
		getPaths:          cfgGetList,
		getSampleInterval: time.Second,
	}

	collectorErrChan := make(chan error, 1)
	gnmiServer := &mixedOriginGNMIServer{}
	gnmireverseServer := &mixedOriginGNMIReverseServer{
		errChan: collectorErrChan,
	}
	runStreamGetResponsesTest(t, cfg, collectorErrChan,
		gnmiServer, gnmireverseServer, streamGetResponses)
}

var testEOSNativeNotification = &gnmi.Notification{
	Timestamp: 123,
	Prefix: &gnmi.Path{
		Target: "baz",
		Origin: "eos_native",
	},
	Update: []*gnmi.Update{{
		Path: &gnmi.Path{
			Elem: []*gnmi.PathElem{
				{Name: "a"},
				{Name: "b"},
			},
		},
		Val: &gnmi.TypedValue{
			Value: &gnmi.TypedValue_StringVal{
				StringVal: "c",
			},
		},
	}},
}

var testEOSNativeGetResponse = &gnmi.GetResponse{
	Notification: []*gnmi.Notification{testEOSNativeNotification},
}

var testCombinedGetResponse = &gnmi.GetResponse{
	Notification: []*gnmi.Notification{testNotification, testEOSNativeNotification},
}

// Mock gNMIReverse server checks if the published Get response matches the
// testCombinedGetResponse.
type mixedOriginGNMIReverseServer struct {
	errChan chan error
	gnmireverse.UnimplementedGNMIReverseServer
}

func (s *mixedOriginGNMIReverseServer) PublishGet(
	stream gnmireverse.GNMIReverse_PublishGetServer) error {
	for {
		res, err := stream.Recv()
		if err != nil {
			return err
		}
		// Overwrite the OpenConfig and EOS native notification timestamps so notification
		// can be compared.
		res.Notification[0].Timestamp = 123
		res.Notification[1].Timestamp = 123
		if !proto.Equal(testCombinedGetResponse, res) {
			s.errChan <- fmt.Errorf(
				"Get response not equal: want %v, got %v", testGetResponse, res)
		} else {
			s.errChan <- nil
		}
	}
}

// Mock gNMI server returns for Get the testGetResponse for OpenConfig origins and
// testEOSNativeGetResponse for EOS native origins.
type mixedOriginGNMIServer struct {
	gnmi.UnimplementedGNMIServer
}

func (*mixedOriginGNMIServer) Get(
	ctx context.Context, req *gnmi.GetRequest) (*gnmi.GetResponse, error) {
	if req.GetPath()[0].GetOrigin() == "eos_native" {
		return testEOSNativeGetResponse, nil
	}
	return testGetResponse, nil
}

func TestStreamGetResponsesModeSubscribe(t *testing.T) {
	// Set the Get paths list with one OpenConfig and one EOS native path.
	var cfgGetList getList
	cfgGetList.Set("/foo/bar")
	cfgGetList.Set("eos_native:/a/b")

	cfg := &config{
		targetVal:         "baz",
		targetAddr:        getTestAddress(t),
		collectorAddr:     getTestAddress(t),
		getPaths:          cfgGetList,
		getSampleInterval: time.Second,
	}

	collectorErrChan := make(chan error, 1)
	gnmiServer := &gnmiServerGetModeSubscribe{}
	gnmireverseServer := &gnmireverseServerGetModeSubscribe{
		errChan: collectorErrChan,
	}
	runStreamGetResponsesTest(t, cfg, collectorErrChan,
		gnmiServer, gnmireverseServer, streamGetResponsesModeSubscribe)
}

func TestStreamGetResponsesModeSubscribeOnceNotSupported(t *testing.T) {
	// Set the Get paths list with one OpenConfig and one EOS native path.
	var cfgGetList getList
	cfgGetList.Set("/foo/bar")
	cfgGetList.Set("eos_native:/a/b")

	cfg := &config{
		targetVal:         "baz",
		targetAddr:        getTestAddress(t),
		collectorAddr:     getTestAddress(t),
		getPaths:          cfgGetList,
		getSampleInterval: time.Second,
	}

	collectorErrChan := make(chan error, 1)
	gnmiServer := &gnmiServerGetModeSubscribe{
		subscribeOnceNotSupported: true,
	}
	gnmireverseServer := &gnmireverseServerGetModeSubscribe{
		errChan: collectorErrChan,
	}
	runStreamGetResponsesTest(t, cfg, collectorErrChan,
		gnmiServer, gnmireverseServer, streamGetResponsesModeSubscribe)
}

var testSubscribeResponse = &gnmi.SubscribeResponse{
	Response: &gnmi.SubscribeResponse_Update{
		Update: testNotification,
	},
}

var testEOSNativeSubscribeResponse = &gnmi.SubscribeResponse{
	Response: &gnmi.SubscribeResponse_Update{
		Update: testEOSNativeNotification,
	},
}

// Mock gNMIReverse server checks if the published Get response matches the
// testCombinedGetResponse.
type gnmireverseServerGetModeSubscribe struct {
	errChan chan error
	gnmireverse.UnimplementedGNMIReverseServer
}

func (s *gnmireverseServerGetModeSubscribe) PublishGet(
	stream gnmireverse.GNMIReverse_PublishGetServer) error {
	for {
		res, err := stream.Recv()
		if err != nil {
			return err
		}
		if !proto.Equal(testCombinedGetResponse, res) {
			s.errChan <- fmt.Errorf(
				"Get response not equal: want %v, got %v", testCombinedGetResponse, res)
		} else {
			s.errChan <- nil
		}
	}
}

// gNMI server which mocks the Subscribe behaviour and sends expected
// responses for OpenConfig and EOS native path subscriptions.
type gnmiServerGetModeSubscribe struct {
	subscribeOnceNotSupported bool
	gnmi.UnimplementedGNMIServer
}

func (s *gnmiServerGetModeSubscribe) Subscribe(stream gnmi.GNMI_SubscribeServer) error {
	req, err := stream.Recv()
	if err != nil {
		return err
	}
	origin := req.GetSubscribe().GetSubscription()[0].GetPath().GetOrigin()
	switch origin {
	case "":
		return openconfigSubscribePoll(stream, req)
	case "eos_native":
		mode := req.GetSubscribe().GetMode()
		switch mode {
		case gnmi.SubscriptionList_ONCE:
			if s.subscribeOnceNotSupported {
				return status.Errorf(codes.Unimplemented, "Subscribe ONCE mode not supported")
			}
			return eosNativeSubscribeOnce(stream, req)
		case gnmi.SubscriptionList_STREAM:
			return eosNativeSubscribeStream(stream)
		default:
			return fmt.Errorf("unexpected Subscribe mode: %s", mode.String())
		}
	default:
		return fmt.Errorf("unexpected Subscribe origin: %s", origin)
	}
}

var syncResponse = &gnmi.SubscribeResponse{
	Response: &gnmi.SubscribeResponse_SyncResponse{
		SyncResponse: true,
	},
}

// openconfigSubscribePoll mocks the server Subscribe POLL behavior.
func openconfigSubscribePoll(stream gnmi.GNMI_SubscribeServer, req *gnmi.SubscribeRequest) error {
	if !(req.GetSubscribe().GetMode() == gnmi.SubscriptionList_POLL &&
		req.GetSubscribe().GetUpdatesOnly()) {
		return fmt.Errorf("expect SubscribeRequest mode=POLL and updates_only=true,"+
			" got mode=%s and updates_only=%t", req.GetSubscribe().GetMode(),
			req.GetSubscribe().GetUpdatesOnly())
	}
	// Send initial sync response.
	if err := stream.Send(syncResponse); err != nil {
		return err
	}

	for {
		// Await for poll trigger request.
		req, err := stream.Recv()
		if err != nil {
			return err
		}
		if req.GetPoll() == nil {
			return fmt.Errorf("expect Subscribe POLL trigger request, got %s", req)
		}
		// Send one notification.
		if err := stream.Send(testSubscribeResponse); err != nil {
			return err
		}
		// Mark the end of the poll updates with a sync response.
		if err := stream.Send(syncResponse); err != nil {
			return err
		}
	}
}

// eosNativeSubscribeOnce mocks the server Subscribe ONCE behavior.
func eosNativeSubscribeOnce(stream gnmi.GNMI_SubscribeServer, req *gnmi.SubscribeRequest) error {
	if !req.GetSubscribe().GetUpdatesOnly() {
		// Send one notification.
		if err := stream.Send(testEOSNativeSubscribeResponse); err != nil {
			return err
		}
	}
	// Mark the end of updates with a sync response.
	return stream.Send(syncResponse)
}

// eosNativeSubscribeStream mocks the server Subscribe STREAM behavior.
func eosNativeSubscribeStream(stream gnmi.GNMI_SubscribeServer) error {
	// Send one notification.
	if err := stream.Send(testEOSNativeSubscribeResponse); err != nil {
		return err
	}
	// Mark the end of updates with a sync response.
	if err := stream.Send(syncResponse); err != nil {
		return err
	}
	// Wait for the gNMIReverse client to close the stream.
	<-stream.Context().Done()
	return nil
}
