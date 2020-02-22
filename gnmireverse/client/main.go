// Copyright (c) 2020 Arista Networks, Inc.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the COPYING file.

package main

import (
	"context"
	"flag"
	"fmt"
	"strings"

	gnmilib "github.com/aristanetworks/goarista/gnmi"
	"github.com/aristanetworks/goarista/gnmireverse"

	"github.com/aristanetworks/glog"
	"github.com/openconfig/gnmi/proto/gnmi"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

type multiPath struct {
	p []*gnmi.Path
}

func (m *multiPath) String() string {
	if m == nil {
		return ""
	}
	s := make([]string, len(m.p))
	for i, p := range m.p {
		s[i] = gnmilib.StrPath(p)
	}
	return strings.Join(s, ", ")
}

// Set implements flag.Value interface
func (m *multiPath) Set(s string) error {
	gnmiPath, err := gnmilib.ParseGNMIElements(gnmilib.SplitPath(s))
	if err != nil {
		return err
	}
	m.p = append(m.p, gnmiPath)
	return nil
}

func main() {
	targetAddr := flag.String("target_addr", "127.0.0.1:6030", "address of the gNMI target")
	destAddr := flag.String("collector_addr", "",
		"address of collector in the form of [<vrf-name>/]address:port")
	target := flag.String("target_value", "",
		"value to use in the target field of the Subscribe")
	paths := multiPath{}
	flag.Var(&paths, "subscribe",
		"Path to subscribe to. This option can be repeated multiple times.")

	username := flag.String("username", "", "username to authenticate with target")
	password := flag.String("password", "", "password to authenticate with target")
	sourceAddr := flag.String("source_addr", "", "addr to use as source in connection to collector")

	clientCert := flag.String("cert", "",
		"path to certificate to use to authenticate with collector")
	ca := flag.String("ca", "", "path to CA to verify collector")

	flag.Parse()

	var (
		_ = sourceAddr
		_ = clientCert
		_ = ca
	)

	// TODO: handle vrf, sourceAddr, clientCert
	destConn, err := grpc.Dial(*destAddr)
	if err != nil {
		glog.Fatalf("error dialing destination %q: %s", *destAddr, err)
	}
	targetConn, err := grpc.Dial(*targetAddr)
	if err != nil {
		glog.Fatalf("error dialing target %q: %s", *targetAddr, err)
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
			return subscribe(ctx, targetConn, c, *username, *password, *target, paths.p)
		})
		err := eg.Wait()
		if err != nil {
			glog.Errorf("encountered error, retrying: %s", err)
		}
	}
}

func publish(ctx context.Context, destConn *grpc.ClientConn,
	c <-chan *gnmi.SubscribeResponse) error {
	client := gnmireverse.NewGNMIReverseClient(destConn)
	stream, err := client.Publish(ctx)
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

func subscribe(ctx context.Context, targetConn *grpc.ClientConn,
	c chan<- *gnmi.SubscribeResponse, username, password, target string, paths []*gnmi.Path) error {
	client := gnmi.NewGNMIClient(targetConn)
	subList := &gnmi.SubscriptionList{
		Prefix: &gnmi.Path{Target: target},
	}

	for _, p := range paths {
		subList.Subscription = append(subList.Subscription,
			&gnmi.Subscription{
				Path: p,
				Mode: gnmi.SubscriptionMode_TARGET_DEFINED,
			},
		)
	}
	request := &gnmi.SubscribeRequest{
		Request: &gnmi.SubscribeRequest_Subscribe{
			Subscribe: subList,
		},
	}

	if username != "" {
		ctx = metadata.NewOutgoingContext(ctx,
			metadata.Pairs(
				"username", username,
				"password", password),
		)
	}
	stream, err := client.Subscribe(ctx)
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
