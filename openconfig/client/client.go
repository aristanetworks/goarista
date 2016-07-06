// Copyright (C) 2016  Arista Networks, Inc.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the COPYING file.

// Package client provides helper functions for OpenConfig CLI tools.
package client

import (
	"io"
	"strings"
	"sync"

	"github.com/aristanetworks/glog"
	"github.com/aristanetworks/goarista/openconfig"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

const defaultPort = "6042"

// Run creates a new gRPC client, sends subscriptions, and consumes responses.
// The given publish function is used to publish SubscribeResponses received
// for the given subscriptions, when connected to the given host, with the
// given user/pass pair, or the client-side cert specified in the gRPC opts.
// This function does not normally return so it should probably be run in its
// own goroutine.  When this function returns, the given WaitGroup is marked
// as done.
func Run(publish func(*openconfig.SubscribeResponse), wg *sync.WaitGroup,
	username, password, addr string, subscriptions []string,
	opts []grpc.DialOption) {

	defer wg.Done()
	if !strings.ContainsRune(addr, ':') {
		addr += ":" + defaultPort
	}
	conn, err := grpc.Dial(addr, opts...)
	if err != nil {
		glog.Fatalf("fail to dial: %s", err)
	}
	glog.Infof("Connected to %s", addr)
	defer conn.Close()
	client := openconfig.NewOpenConfigClient(conn)

	ctx := context.Background()
	if username != "" {
		ctx = metadata.NewContext(ctx, metadata.Pairs(
			"username", username,
			"password", password))
	}

	stream, err := client.Subscribe(ctx)
	if err != nil {
		glog.Fatalf("Subscribe failed: %s", err)
	}
	defer stream.CloseSend()

	for _, path := range subscriptions {
		sub := &openconfig.SubscribeRequest{
			Request: &openconfig.SubscribeRequest_Subscribe{
				Subscribe: &openconfig.SubscriptionList{
					Subscription: []*openconfig.Subscription{
						&openconfig.Subscription{
							Path: &openconfig.Path{Element: strings.Split(path, "/")},
						},
					},
				},
			},
		}

		glog.Infof("Sending subscribe request: %s", sub)
		err = stream.Send(sub)
		if err != nil {
			glog.Fatalf("Failed to subscribe: %s", err)
		}
	}

	for {
		resp, err := stream.Recv()
		if err != nil {
			if err != io.EOF {
				glog.Fatalf("Error received from the server: %s", err)
			}
			return
		}
		glog.V(3).Info(resp)
		publish(resp)
	}
}
