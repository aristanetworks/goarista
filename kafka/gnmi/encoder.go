// Copyright (c) 2016 Arista Networks, Inc.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the COPYING file.

package gnmi

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/aristanetworks/goarista/elasticsearch"
	"github.com/aristanetworks/goarista/kafka"

	"github.com/Shopify/sarama"
	"github.com/aristanetworks/glog"
	"github.com/golang/protobuf/proto"
	"github.com/openconfig/gnmi/proto/gnmi"
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
	response *gnmi.SubscribeResponse
}

func (e UnhandledSubscribeResponseError) Error() string {
	return fmt.Sprintf("Unexpected type %T in subscribe response: %#v", e.response, e.response)
}

type elasticsearchMessageEncoder struct {
	*kafka.BaseEncoder
	topic   string
	dataset string
	key     sarama.Encoder
}

// NewEncoder creates and returns a new elasticsearch MessageEncoder
func NewEncoder(topic string, key sarama.Encoder, dataset string) kafka.MessageEncoder {
	baseEncoder := kafka.NewBaseEncoder("elasticsearch")
	return &elasticsearchMessageEncoder{
		BaseEncoder: baseEncoder,
		topic:       topic,
		dataset:     dataset,
		key:         key,
	}
}

func (e *elasticsearchMessageEncoder) Encode(message proto.Message) ([]*sarama.ProducerMessage,
	error) {
	response, ok := message.(*gnmi.SubscribeResponse)
	if !ok {
		return nil, UnhandledMessageError{message: message}
	}
	update := response.GetUpdate()
	if update == nil {
		return nil, UnhandledSubscribeResponseError{response: response}
	}
	updateMaps, err := elasticsearch.NotificationToMaps(e.dataset, update)
	if err != nil {
		return nil, err
	}
	messages := make([]*sarama.ProducerMessage, len(updateMaps))
	for i, updateMap := range updateMaps {
		updateJSON, err := json.Marshal(updateMap)
		if err != nil {
			return nil, err
		}
		glog.V(9).Infof("kafka: %s", updateJSON)

		messages[i] = &sarama.ProducerMessage{
			Topic:    e.topic,
			Key:      e.key,
			Value:    sarama.ByteEncoder(updateJSON),
			Metadata: kafka.Metadata{StartTime: time.Unix(0, update.Timestamp), NumMessages: 1},
		}
	}
	return messages, nil
}
