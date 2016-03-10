// Copyright (C) 2016  Arista Networks, Inc.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the COPYING file.

package openconfig

import (
	"fmt"
	"time"

	"github.com/Shopify/sarama"
	"github.com/aristanetworks/glog"
	"github.com/aristanetworks/goarista/kafka"
	"github.com/aristanetworks/goarista/openconfig"
	"github.com/golang/protobuf/proto"
)

// UnhandledMessageError is used for proto messages not matching the handled types
type UnhandledMessageError struct {
	message proto.Message
}

func (e UnhandledMessageError) Error() string {
	return fmt.Sprintf("Unexpected type %T in proto message: %#v", e.message, e.message)
}

// UnhandledSubscribeResponseError is used for subscribe responses not matching the handled types
type UnhandledSubscribeResponseError struct {
	response *openconfig.SubscribeResponse
}

func (e UnhandledSubscribeResponseError) Error() string {
	return fmt.Sprintf("Unexpected type %T in subscribe response: %#v", e.response, e.response)
}

// MessageEncoder defines the encoding from SubscribeResponse to sarama.ProducerMessage
func MessageEncoder(topic string, key sarama.Encoder,
	message proto.Message) (*sarama.ProducerMessage, error) {
	subscribeResponse, ok := message.(*openconfig.SubscribeResponse)
	if !ok {
		return nil, UnhandledMessageError{message: message}
	}
	if _, ok = subscribeResponse.Response.(*openconfig.SubscribeResponse_Update); !ok {
		return nil, UnhandledSubscribeResponseError{response: subscribeResponse}
	}
	value, err := proto.Marshal(message)
	if err != nil {
		glog.Errorf("Failed to encode SubscribeResponse: %s", err)
		return nil, err
	}
	return &sarama.ProducerMessage{
		Topic: topic,
		Key:   key,
		Value: sarama.ByteEncoder(value),
		// TODO: Add a monotonic clock source when one becomes available
		Metadata: kafka.Metadata{StartTime: time.Now(), NumMessages: 1},
	}, nil
}
