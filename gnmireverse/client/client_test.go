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
	"google.golang.org/grpc"
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
	runStreamGetResponsesTest(t, cfg, collectorErrChan, gnmiServer, gnmireverseServer)
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
	gnmiServer gnmi.GNMIServer, gnmireverseServer gnmireverse.GNMIReverseServer) {
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
		streamResponses(streamGetResponses(cfg, destConn, targetConn))
	}()

	// Check that the gNMIReverse collector server receives the expected Get response.
	if err := <-collectorErrChan; err != nil {
		t.Error(err)
	}
}

var testGetResponse = &gnmi.GetResponse{
	Notification: []*gnmi.Notification{{
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
	}},
}

// Mock gNMIReverse server checks if the published Get response matches the testGetResponse.
type gnmireverseServer struct {
	errChan chan error
	gnmireverse.UnsafeGNMIReverseServer
}

func (*gnmireverseServer) Publish(stream gnmireverse.GNMIReverse_PublishServer) error {
	return nil
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
	gnmi.UnsafeGNMIServer
}

func (*gnmiServer) Capabilities(context.Context, *gnmi.CapabilityRequest) (
	*gnmi.CapabilityResponse, error) {
	return nil, nil
}
func (*gnmiServer) Set(context.Context, *gnmi.SetRequest) (*gnmi.SetResponse, error) {
	return nil, nil
}
func (*gnmiServer) Subscribe(gnmi.GNMI_SubscribeServer) error {
	return nil
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
	runStreamGetResponsesTest(t, cfg, collectorErrChan, gnmiServer, gnmireverseServer)
}

var testEOSNativeGetResponse = &gnmi.GetResponse{
	Notification: []*gnmi.Notification{{
		Prefix: &gnmi.Path{
			Target: "baz",
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
	}},
}

var testCombinedGetResponse = &gnmi.GetResponse{
	Notification: []*gnmi.Notification{{
		Timestamp: 123,
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
	}, {
		Timestamp: 123,
		Prefix: &gnmi.Path{
			Target: "baz",
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
	}},
}

// Mock gNMIReverse server checks if the published Get response matches the
// testCombinedGetResponse.
type mixedOriginGNMIReverseServer struct {
	errChan chan error
	gnmireverse.UnsafeGNMIReverseServer
}

func (*mixedOriginGNMIReverseServer) Publish(stream gnmireverse.GNMIReverse_PublishServer) error {
	return nil
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
	gnmi.UnsafeGNMIServer
}

func (*mixedOriginGNMIServer) Capabilities(context.Context, *gnmi.CapabilityRequest) (
	*gnmi.CapabilityResponse, error) {
	return nil, nil
}
func (*mixedOriginGNMIServer) Set(context.Context, *gnmi.SetRequest) (
	*gnmi.SetResponse, error) {
	return nil, nil
}
func (*mixedOriginGNMIServer) Subscribe(gnmi.GNMI_SubscribeServer) error {
	return nil
}

func (*mixedOriginGNMIServer) Get(
	ctx context.Context, req *gnmi.GetRequest) (*gnmi.GetResponse, error) {
	if req.GetPath()[0].GetOrigin() == "eos_native" {
		return testEOSNativeGetResponse, nil
	}
	return testGetResponse, nil
}
