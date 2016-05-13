// Copyright (C) 2016  Arista Networks, Inc.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the COPYING file.

package kafka

import (
	"os"

	"github.com/Shopify/sarama"
)

// NewClient returns a Kafka client
func NewClient(addresses []string) (sarama.Client, error) {
	config := sarama.NewConfig()
	hostname, err := os.Hostname()
	if err != nil {
		hostname = ""
	}
	config.ClientID = hostname
	config.Producer.Compression = sarama.CompressionSnappy
	config.Producer.Return.Successes = true

	return sarama.NewClient(addresses, config)
}
