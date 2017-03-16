// Copyright (C) 2016  Arista Networks, Inc.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the COPYING file.

package openconfig

import (
	"encoding/json"
	"expvar"
	"fmt"

	"sync/atomic"
	"time"

	"github.com/Shopify/sarama"
	"github.com/aristanetworks/glog"
	"github.com/aristanetworks/goarista/elasticsearch"
	"github.com/aristanetworks/goarista/kafka"
	"github.com/aristanetworks/goarista/monitor"
	"github.com/aristanetworks/goarista/openconfig"
	"github.com/golang/protobuf/proto"

	pb "github.com/openconfig/reference/rpc/openconfig"
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
	response *pb.SubscribeResponse
}

func (e UnhandledSubscribeResponseError) Error() string {
	return fmt.Sprintf("Unexpected type %T in subscribe response: %#v", e.response, e.response)
}

// counter counts the number Sysdb clients we have, and is used to guarantee that we
// always have a unique name exported to expvar
var counter uint32

// elasticsearchMessageEncoder defines the encoding from SubscribeResponse to
// sarama.ProducerMessage for elasticsearch
type elasticsearchMessageEncoder struct {
	topic   string
	key     sarama.Encoder
	dataset string

	// Used for monitoring
	histogram    *monitor.Histogram
	numSuccesses monitor.Uint
	numFailures  monitor.Uint
}

// NewEncoder returns a new kafka.MessageEncoder
func NewEncoder(topic string, key sarama.Encoder, dataset string) kafka.MessageEncoder {

	// Setup monitoring structures
	histName := "kafkaProducerHistogram_elasticsearch"
	statsName := "messagesStats"
	if id := atomic.AddUint32(&counter, 1); id > 1 {
		histName = fmt.Sprintf("%s-%d", histName, id)
		statsName = fmt.Sprintf("%s-%d", statsName, id)
	}
	hist := monitor.NewHistogram(histName, 32, 0.3, 1000, 0)

	statsMap := expvar.NewMap(statsName)

	e := &elasticsearchMessageEncoder{
		topic:     topic,
		key:       key,
		dataset:   dataset,
		histogram: hist,
	}

	statsMap.Set("successes", &e.numSuccesses)
	statsMap.Set("failures", &e.numFailures)

	return e
}

// Encode encodes the proto message to a sarama.ProducerMessage
func (e *elasticsearchMessageEncoder) Encode(message proto.Message) (*sarama.ProducerMessage,
	error) {
	response, ok := message.(*pb.SubscribeResponse)
	if !ok {
		return nil, UnhandledMessageError{message: message}
	}
	update := response.GetUpdate()
	if update == nil {
		return nil, UnhandledSubscribeResponseError{response: response}
	}
	updateMap, err := openconfig.NotificationToMap(e.dataset, update,
		elasticsearch.EscapeFieldName)
	if err != nil {
		return nil, err
	}
	// Convert time to ms to make elasticsearch happy
	updateMap["timestamp"] = updateMap["timestamp"].(int64) / 1000000
	updateJSON, err := json.Marshal(updateMap)
	if err != nil {
		return nil, err
	}
	glog.V(9).Infof("kafka: %s", updateJSON)
	return &sarama.ProducerMessage{
		Topic:    e.topic,
		Key:      e.key,
		Value:    sarama.ByteEncoder(updateJSON),
		Metadata: kafka.Metadata{StartTime: time.Unix(0, update.Timestamp), NumMessages: 1},
	}, nil
}

// HandleSuccess process the metadata of messages from kafka producer Successes channel
func (e *elasticsearchMessageEncoder) HandleSuccess(msg *sarama.ProducerMessage) {
	metadata := msg.Metadata.(kafka.Metadata)
	// TODO: Add a monotonic clock source when one becomes available
	e.histogram.UpdateLatencyValues(metadata.StartTime, time.Now())
	e.numSuccesses.Add(uint64(metadata.NumMessages))
}

// HandleError process the metadata of messages from kafka producer Errors channel
func (e *elasticsearchMessageEncoder) HandleError(msg *sarama.ProducerError) {
	metadata := msg.Msg.Metadata.(kafka.Metadata)
	// TODO: Add a monotonic clock source when one becomes available
	e.histogram.UpdateLatencyValues(metadata.StartTime, time.Now())
	glog.Errorf("Kafka Producer error: %s", msg.Error())
	e.numFailures.Add(uint64(metadata.NumMessages))

}
