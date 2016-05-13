// Copyright (C) 2016  Arista Networks, Inc.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the COPYING file.

// The occlient tool is a client for the gRPC service for getting and setting the
// OpenConfig configuration and state of a network device.
package main

import (
	"flag"
	"fmt"
	"strings"
	"sync"

	"github.com/Shopify/sarama"
	"github.com/aristanetworks/glog"
	"github.com/aristanetworks/goarista/kafka"
	"github.com/aristanetworks/goarista/kafka/openconfig"
	"github.com/aristanetworks/goarista/kafka/producer"
	pb "github.com/aristanetworks/goarista/openconfig"
	"github.com/aristanetworks/goarista/openconfig/client"
)

const defaultGRPCPort = "6042"

var keysFlag = flag.String("kafkakeys", "",
	"Keys for kafka messages (comma-separated, default: the value of -addrs")

func newProducer(addresses []string, topic, key string) (producer.Producer, error) {
	client, err := kafka.NewClient(addresses)
	if err != nil {
		return nil, fmt.Errorf("Failed to create Kafka client: %s", err)
	}
	encodedKey := sarama.StringEncoder(key)
	p, err := producer.New(topic, nil, client, encodedKey, openconfig.MessageEncoder)
	if err != nil {
		return nil, fmt.Errorf("Failed to create Kafka producer: %s", err)
	}
	return p, nil
}

func main() {
	username, password, subscriptions, grpcAddrs, opts := client.ParseFlags()

	if *keysFlag == "" {
		*keysFlag = strings.Join(grpcAddrs, ",")
	}
	keys := strings.Split(*keysFlag, ",")
	if len(grpcAddrs) != len(keys) {
		glog.Fatal("Please provide the same number of addresses and Kafka keys")
	}
	addresses := strings.Split(*kafka.Addresses, ",")
	wg := new(sync.WaitGroup)
	for i, grpcAddr := range grpcAddrs {
		key := keys[i]
		if !strings.ContainsRune(grpcAddr, ':') {
			grpcAddr += ":" + defaultGRPCPort
		}
		p, err := newProducer(addresses, *kafka.Topic, key)
		if err != nil {
			glog.Fatal("Failed to initialize producer: ", err)
		}
		publish := func(notif *pb.SubscribeResponse) {
			p.Write(notif)
		}
		wg.Add(1)
		go p.Run()
		go client.Run(publish, wg, username, password, grpcAddr, subscriptions, opts)
	}
	wg.Wait()
}
