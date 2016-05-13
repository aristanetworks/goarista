// Copyright (C) 2016  Arista Networks, Inc.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the COPYING file.

// The occlient tool is a client for the gRPC service for getting and setting the
// OpenConfig configuration and state of a network device.
package main

import (
	"flag"
	"strings"
	"sync"

	"github.com/aristanetworks/goarista/kafka"
	"github.com/aristanetworks/goarista/openconfig/client"

	"github.com/aristanetworks/glog"
	pb "github.com/aristanetworks/goarista/openconfig"
)

const defaultPort = "6042"

var kafkaKeysFlag = flag.String("kafkakey", "",
	"Keys for kafka messages (comma-separated, default: the value of -addrs")

func main() {
	username, password, subscriptions, addrs, opts := client.ParseFlags()

	if *kafkaKeysFlag == "" {
		*kafkaKeysFlag = strings.Join(addrs, ",")
	}
	kafkaKeys := strings.Split(*kafkaKeysFlag, ",")
	if len(addrs) != len(kafkaKeys) {
		glog.Fatal("Please provide the same number of addresses and Kafka keys")
	}
	var kafkaAddresses []string
	if *kafka.Addresses != "" {
		kafkaAddresses = strings.Split(*kafka.Addresses, ",")
	}
	wg := new(sync.WaitGroup)
	for i, addr := range addrs {
		kafkaKey := kafkaKeys[i]
		if !strings.ContainsRune(addr, ':') {
			addr += ":" + defaultPort
		}
		p, err := newKafkaProducer(kafkaAddresses, *kafka.Topic, kafkaKey)
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
