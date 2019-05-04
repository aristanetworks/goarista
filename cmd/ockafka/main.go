// Copyright (c) 2016 Arista Networks, Inc.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the COPYING file.

// The occlient tool is a client for the gRPC service for getting and setting the
// OpenConfig configuration and state of a network device.
package main

import (
	"context"
	"flag"
	"fmt"
	"strings"
	"sync"

	client "github.com/aristanetworks/goarista/gnmi"
	"github.com/aristanetworks/goarista/kafka"
	"github.com/aristanetworks/goarista/kafka/gnmi"
	"github.com/aristanetworks/goarista/kafka/producer"

	pb "github.com/openconfig/gnmi/proto/gnmi"

	"github.com/Shopify/sarama"
	"github.com/aristanetworks/glog"
)

var keysFlag = flag.String("kafkakeys", "",
	"Keys for kafka messages (comma-separated, default: the value of -addrs). The key '"+
		client.HostnameArg+"' is replaced by the current hostname.")

func newProducer(addresses []string, topic, key, dataset string) (producer.Producer, error) {
	encodedKey := sarama.StringEncoder(key)
	p, err := producer.New(gnmi.NewEncoder(topic, encodedKey, dataset), addresses, nil)
	if err != nil {
		return nil, fmt.Errorf("Failed to create Kafka brokers: %s", err)
	}
	glog.Infof("Connected to Kafka brokers at %s", addresses)
	return p, nil
}

func main() {
	ctx := context.Background()
	config, subscriptions := client.ParseFlags()
	ctx = client.NewContext(ctx, config)
	grpcAddrs := strings.Split(config.Addr, ",")

	var keys []string
	var err error
	if *keysFlag == "" {
		keys = grpcAddrs
	} else {
		keys, err = client.ParseHostnames(*keysFlag)
		if err != nil {
			glog.Fatal(err)
		}
	}
	if len(grpcAddrs) != len(keys) {
		glog.Fatal("Please provide the same number of addresses and Kafka keys")
	}
	addresses := strings.Split(*kafka.Addresses, ",")
	wg := new(sync.WaitGroup)
	respChan := make(chan *pb.SubscribeResponse)
	defer close(respChan)
	errChan := make(chan error)
	defer close(errChan)
	for i, grpcAddr := range grpcAddrs {
		key := keys[i]
		p, err := newProducer(addresses, *kafka.Topic, key, grpcAddr)
		if err != nil {
			glog.Fatal(err)
		} else {
			glog.Infof("Initialized Kafka producer for %s", grpcAddr)
		}
		wg.Add(1)
		p.Start()
		defer p.Stop()
		c, err := client.Dial(config)
		if err != nil {
			glog.Fatal(err)
		}
		subscribeOptions := &client.SubscribeOptions{
			Paths: client.SplitPaths(subscriptions),
		}
		go client.Subscribe(ctx, c, subscribeOptions, respChan, errChan)
		for {
			select {
			case resp, open := <-respChan:
				if !open {
					return
				}
				p.Write(resp)
			case err := <-errChan:
				glog.Fatal(err)
			}
		}
	}
	wg.Wait()
}
