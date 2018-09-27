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

	"github.com/aristanetworks/goarista/kafka"
	"github.com/aristanetworks/goarista/kafka/openconfig"
	"github.com/aristanetworks/goarista/kafka/producer"
	"github.com/aristanetworks/goarista/gnmi"

	"github.com/Shopify/sarama"
	"github.com/aristanetworks/glog"
	pb "github.com/openconfig/gnmi/proto/gnmi"
)

var keyFlag = flag.String("kafkakey", "",
	"Key for kafka messages (default: the value of -addr")

func newProducer(addresses []string, topic, key, dataset string) (producer.Producer, error) {
	encodedKey := sarama.StringEncoder(key)
	p, err := producer.New(openconfig.NewEncoder(topic, encodedKey, dataset), addresses, nil)
	if err != nil {
		return nil, fmt.Errorf("Failed to create Kafka brokers: %s", err)
	}
	glog.Infof("Connected to Kafka brokers at %s", addresses)
	return p, nil
}

func main() {
	cfg := &gnmi.Config{}
	flag.StringVar(&cfg.Addr, "addr", "", "Address of gNMI gRPC server with optional VRF name")
	flag.StringVar(&cfg.CAFile, "cafile", "", "Path to server TLS certificate file")
	flag.StringVar(&cfg.CertFile, "certfile", "", "Path to client TLS certificate file")
	flag.StringVar(&cfg.KeyFile, "keyfile", "", "Path to client TLS private key file")
	flag.StringVar(&cfg.Password, "password", "", "Password to authenticate with")
	flag.StringVar(&cfg.Username, "username", "", "Username to authenticate with")
	flag.BoolVar(&cfg.TLS, "tls", false, "Enable TLS")
	subscribePaths := flag.String("subscribe", "/", "Comma-separated list of paths to subscribe to")
	flag.Parse()

	var key string
	if key = *keyFlag; key == "" {
		key = cfg.Addr
	}
	addresses := strings.Split(*kafka.Addresses, ",")
	p, err := newProducer(addresses, *kafka.Topic, key, cfg.Addr)
	if err != nil {
		glog.Fatal(err)
	} else {
		glog.Infof("Initialized Kafka producer for %s", cfg.Addr)
	}
	p.Start()
	defer p.Stop()

	subscriptions := strings.Split(*subscribePaths, ",")
	ctx := gnmi.NewContext(context.Background(), cfg)
	client, err := gnmi.Dial(cfg)
	if err != nil {
		glog.Fatal(err)
	}
	respChan := make(chan *pb.SubscribeResponse)
	errChan := make(chan error)
	defer close(errChan)
	subscribeOptions := &gnmi.SubscribeOptions{
		Mode:       "stream",
		StreamMode: "target_defined",
		Paths:      gnmi.SplitPaths(subscriptions),
	}
	go gnmi.Subscribe(ctx, client, subscribeOptions, respChan, errChan)
	for {
		select {
		case resp := <-respChan:
			func(response *pb.SubscribeResponse) {
				glog.V(9).Infof("publishing message: %v", *response)
				p.Write(response)
			}(resp)
		case err := <-errChan:
			glog.Fatal(err)
		}
	}
}
