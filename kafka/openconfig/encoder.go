// Copyright (C) 2016  Arista Networks, Inc.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the COPYING file.

package openconfig

import (
	"encoding/json"
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

// jsonify maps a Notification into a JSON document
func jsonify(notification *openconfig.Notification) ([]byte, error) {
	prefix := notification.GetPrefix()
	root := make(map[string]interface{})
	prefixLeaf := root
	if prefix != nil {
		parent := root
		for _, element := range prefix.Element {
			node := map[string]interface{}{}
			parent[element] = node
			parent = node
		}
		prefixLeaf = parent
	}
	for _, update := range notification.GetUpdate() {
		parent := prefixLeaf
		path := update.GetPath()
		elementLen := len(path.Element)
		if elementLen > 1 {
			for _, element := range path.Element[:elementLen-2] {
				node, found := parent[element]
				if !found {
					node = map[string]interface{}{}
					parent[element] = node
				}
				var ok bool
				parent, ok = node.(map[string]interface{})
				if !ok {
					return nil, fmt.Errorf(
						"Node is of type %T (expected map[string]interface)", node)
				}
			}
		}
		value := update.GetValue()
		if value.Type != openconfig.Type_JSON {
			return nil, fmt.Errorf("Unexpected value type: %s", value.Type)
		}
		var unmarshaledValue interface{}
		if err := json.Unmarshal(value.Value, &unmarshaledValue); err != nil {
			return nil, err
		}
		parent[path.Element[elementLen-1]] = unmarshaledValue
	}
	return json.Marshal(root)
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
	updateJSON, err := jsonify(update)
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
