// Copyright (C) 2016  Arista Networks, Inc.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the COPYING file.

package main

import (
	"fmt"
	"os"

	"github.com/Shopify/sarama"
	"github.com/aristanetworks/goarista/kafka/openconfig"
	"github.com/aristanetworks/goarista/kafka/producer"
	"github.com/golang/protobuf/proto"
)

func newKafkaProducer(kafkaAddresses []string,
	kafkaTopic, kafkaKey string) (producer.Producer, error) {
	kafkaConfig := sarama.NewConfig()
	hostname, err := os.Hostname()
	if err != nil {
		hostname = ""
	}
	kafkaConfig.ClientID = hostname
	kafkaConfig.Producer.Compression = sarama.CompressionSnappy
	kafkaConfig.Producer.Return.Successes = true

	kafkaClient, err := sarama.NewClient(kafkaAddresses, kafkaConfig)
	if err != nil {
		return nil, fmt.Errorf("Failed to create Kafka client: %s", err)
	}
	kafkaChan := make(chan proto.Message)
	key := sarama.StringEncoder(kafkaKey)
	kafkaProducer, err := producer.New(kafkaTopic, kafkaChan, kafkaClient, key,
		openconfig.MessageEncoder)
	if err != nil {
		return nil, fmt.Errorf("Failed to create Kafka producer: %s", err)
	}
	return kafkaProducer, nil
}
