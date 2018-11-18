// Copyright (c) 2016 Arista Networks, Inc.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the COPYING file.

package producer

import (
	"encoding/json"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/aristanetworks/goarista/kafka/openconfig"
	"github.com/aristanetworks/goarista/test"

	"github.com/Shopify/sarama"
	pb "github.com/openconfig/gnmi/proto/gnmi"
	"github.com/openconfig/gnmi/value"
)

type mockAsyncProducer struct {
	input     chan *sarama.ProducerMessage
	successes chan *sarama.ProducerMessage
	errors    chan *sarama.ProducerError
}

func newMockAsyncProducer() *mockAsyncProducer {
	return &mockAsyncProducer{
		input:     make(chan *sarama.ProducerMessage),
		successes: make(chan *sarama.ProducerMessage),
		errors:    make(chan *sarama.ProducerError)}
}

func (p *mockAsyncProducer) AsyncClose() {
	panic("Not implemented")
}

func (p *mockAsyncProducer) Close() error {
	close(p.successes)
	close(p.errors)
	return nil
}

func (p *mockAsyncProducer) Input() chan<- *sarama.ProducerMessage {
	return p.input
}

func (p *mockAsyncProducer) Successes() <-chan *sarama.ProducerMessage {
	return p.successes
}

func (p *mockAsyncProducer) Errors() <-chan *sarama.ProducerError {
	return p.errors
}

func newNotification(path []string, timestamp *time.Time) *pb.Notification {
	sv, _ := value.FromScalar(timestamp.String())
	return &pb.Notification{
		Timestamp: timestamp.UnixNano() / 1e6,
		Update: []*pb.Update{
			{
				Path: &pb.Path{Element: path},
				Val:  sv,
			},
		},
	}
}

func TestKafkaProducer(t *testing.T) {
	mock := newMockAsyncProducer()
	toDB := make(chan *pb.SubscribeResponse)
	topic := "occlient"
	systemID := "Foobar"
	toDBProducer := &producer{
		notifsChan:    toDB,
		kafkaProducer: mock,
		encoder:       openconfig.NewEncoder(topic, sarama.StringEncoder(systemID), ""),
		done:          make(chan struct{}),
		wg:            sync.WaitGroup{},
	}

	toDBProducer.Start()

	path := []string{"foo", "bar"}
	timestamp := time.Now()
	response := &pb.SubscribeResponse{
		Response: &pb.SubscribeResponse_Update{
			Update: newNotification(path, &timestamp),
		},
	}
	document := map[string]interface{}{
		"timestamp": timestamp.UnixNano() / 1e6,
		"updates": map[string]interface{}{
			"/" + strings.Join(path, "/"): timestamp.String(),
		},
	}

	toDB <- response

	kafkaMessage := <-mock.input
	if kafkaMessage.Topic != topic {
		t.Errorf("Unexpected Topic: %s, expecting %s", kafkaMessage.Topic, topic)
	}
	key, err := kafkaMessage.Key.Encode()
	if err != nil {
		t.Fatalf("Error encoding key: %s", err)
	}
	if string(key) != systemID {
		t.Errorf("Kafka message didn't have expected key: %s, expecting %s", string(key), systemID)
	}

	valueBytes, err := kafkaMessage.Value.Encode()
	if err != nil {
		t.Fatalf("Error encoding value: %s", err)
	}
	var result interface{}
	err = json.Unmarshal(valueBytes, &result)
	if err != nil {
		t.Errorf("Error decoding into JSON: %s", err)
	}
	if !test.DeepEqual(document["updates"], result.(map[string]interface{})["updates"]) {
		t.Errorf("Protobuf sent from Kafka Producer does not match original.\nOriginal: %#v\nNew:%#v",
			document, result)
	}
	toDBProducer.Stop()
}

type mockMsg struct{}

func (m mockMsg) Reset()         {}
func (m mockMsg) String() string { return "" }
func (m mockMsg) ProtoMessage()  {}

func TestProducerStartStop(t *testing.T) {
	// this test checks that Start() followed by Stop() doesn't cause any race conditions.

	mock := newMockAsyncProducer()
	toDB := make(chan *pb.SubscribeResponse)
	topic := "occlient"
	systemID := "Foobar"
	p := &producer{
		notifsChan:    toDB,
		kafkaProducer: mock,
		encoder:       openconfig.NewEncoder(topic, sarama.StringEncoder(systemID), ""),
		done:          make(chan struct{}),
	}

	path := []string{"foo", "bar"}
	timestamp := time.Now()
	msg := &pb.SubscribeResponse{
		Response: &pb.SubscribeResponse_Update{
			Update: newNotification(path, &timestamp),
		},
	}

	done := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-mock.input:
			case <-done:
				return
			}
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-done:
				return
			default:
			}
			p.Write(msg)
		}
	}()
	p.Start()
	p.Write(msg)
	p.Stop()
	close(done)
	wg.Wait()
}
