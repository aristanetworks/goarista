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
	response, ok := message.(*openconfig.SubscribeResponse)
	if !ok {
		return nil, UnhandledMessageError{message: message}
	}
	update := response.GetUpdate()
	if update == nil {
		return nil, UnhandledSubscribeResponseError{response: response}
	}
	updateJSON, err := openconfig.NotificationToJSONDocument(update)
	if err != nil {
		return nil, err
	}
	glog.V(9).Infof("kafka: %s", updateJSON)
	return &sarama.ProducerMessage{
		Topic:    topic,
		Key:      key,
		Value:    sarama.ByteEncoder(updateJSON),
		Metadata: kafka.Metadata{StartTime: time.Unix(0, update.Timestamp), NumMessages: 1},
	}, nil
}
