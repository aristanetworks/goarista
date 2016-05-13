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
	"github.com/golang/protobuf/proto"
)

const defaultPort = "6042"

var keysFlag = flag.String("kafkakeys", "",
	"Keys for kafka messages (comma-separated, default: the value of -addrs")

func newProducer(addresses []string, topic, key string) (producer.Producer, error) {
	client, err := kafka.NewClient(addresses)
	if err != nil {
		return nil, fmt.Errorf("Failed to create Kafka client: %s", err)
	}
	ch := make(chan proto.Message)
	encodedKey := sarama.StringEncoder(key)
	p, err := producer.New(topic, ch, client, encodedKey, openconfig.MessageEncoder)
	if err != nil {
		return nil, fmt.Errorf("Failed to create Kafka producer: %s", err)
	}
	return p, nil
}

func main() {
	username, password, subscriptions, addrs, opts := client.ParseFlags()

	if *keysFlag == "" {
		*keysFlag = strings.Join(addrs, ",")
	}
	keys := strings.Split(*keysFlag, ",")
	if len(addrs) != len(keys) {
		glog.Fatal("Please provide the same number of addresses and Kafka keys")
	}
	var addresses []string
	if *kafka.Addresses != "" {
		addresses = strings.Split(*kafka.Addresses, ",")
	}
	wg := new(sync.WaitGroup)
	for i, addr := range addrs {
		key := keys[i]
		if !strings.ContainsRune(addr, ':') {
			addr += ":" + defaultPort
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
		go client.Run(publish, wg, username, password, addr, subscriptions, opts)
	}
	wg.Wait()
}
