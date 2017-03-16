// Copyright (C) 2017  Arista Networks, Inc.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the COPYING file.

package kafka

import (
	"github.com/Shopify/sarama"
	"github.com/golang/protobuf/proto"
)

// MessageEncoder is an encoder interface
// which handles encoding proto.Message to sarama.ProducerMessage
type MessageEncoder interface {
	Encode(proto.Message) (*sarama.ProducerMessage, error)
	HandleSuccess(*sarama.ProducerMessage)
	HandleError(*sarama.ProducerError)
}
