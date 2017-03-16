// Copyright (C) 2016  Arista Networks, Inc.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the COPYING file.

package producer

import (
	"os"
	"sync"

	"github.com/Shopify/sarama"
	"github.com/aristanetworks/glog"
	"github.com/aristanetworks/goarista/kafka"
	"github.com/aristanetworks/goarista/kafka/openconfig"
	"github.com/golang/protobuf/proto"
)

// Producer forwards messages recvd on a channel to kafka.
type Producer interface {
	Start()
	Write(proto.Message)
	Stop()
}

type producer struct {
	notifsChan    chan proto.Message
	kafkaProducer sarama.AsyncProducer
	encoder       kafka.MessageEncoder
	done          chan struct{}
	wg            sync.WaitGroup
}

// New creates new Kafka producer
func New(notifsChan chan proto.Message, encoder kafka.MessageEncoder,
	kafkaAddresses []string, kafkaConfig *sarama.Config) (Producer, error) {
	if notifsChan == nil {
		notifsChan = make(chan proto.Message)
	}

	if kafkaConfig == nil {
		kafkaConfig := sarama.NewConfig()
		hostname, err := os.Hostname()
		if err != nil {
			hostname = ""
		}
		kafkaConfig.ClientID = hostname
		kafkaConfig.Producer.Compression = sarama.CompressionSnappy
		kafkaConfig.Producer.Return.Successes = true
		kafkaConfig.Producer.RequiredAcks = sarama.WaitForAll
	}

	kafkaProducer, err := sarama.NewAsyncProducer(kafkaAddresses, kafkaConfig)
	if err != nil {
		return nil, err
	}

	p := &producer{
		notifsChan:    notifsChan,
		kafkaProducer: kafkaProducer,
		encoder:       encoder,
		done:          make(chan struct{}),
		wg:            sync.WaitGroup{},
	}
	return p, nil
}

// Start makes producer to start processing writes.
// This method is non-blocking.
func (p *producer) Start() {
	p.wg.Add(3)
	go p.handleSuccesses()
	go p.handleErrors()
	go p.run()
}

func (p *producer) run() {
	defer p.wg.Done()
	for {
		select {
		case batch, open := <-p.notifsChan:
			if !open {
				return
			}
			err := p.produceNotification(batch)
			if err != nil {
				if _, ok := err.(openconfig.UnhandledSubscribeResponseError); !ok {
					panic(err)
				}
			}
		case <-p.done:
			return
		}
	}
}

func (p *producer) Write(m proto.Message) {
	p.notifsChan <- m
}

func (p *producer) Stop() {
	close(p.done)
	p.kafkaProducer.Close()
	p.wg.Wait()
}

func (p *producer) produceNotification(protoMessage proto.Message) error {
	message, err := p.encoder.Encode(protoMessage)
	if err != nil {
		return err
	}
	select {
	case p.kafkaProducer.Input() <- message:
		glog.V(9).Infof("Message produced to Kafka: %s", message)
		return nil
	case <-p.done:
		return nil
	}
}

// handleSuccesses reads from the producer's successes channel and collects some
// information for monitoring
func (p *producer) handleSuccesses() {
	defer p.wg.Done()
	for msg := range p.kafkaProducer.Successes() {
		p.encoder.HandleSuccess(msg)
	}
}

// handleErrors reads from the producer's errors channel and collects some information
// for monitoring
func (p *producer) handleErrors() {
	defer p.wg.Done()
	for msg := range p.kafkaProducer.Errors() {
		p.encoder.HandleError(msg)

	}
}
