// Copyright (c) 2019 Arista Networks, Inc.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the COPYING file.

//go:build !linux
// +build !linux

package netns

import (
	"net"

	"github.com/aristanetworks/goarista/dscp"
	"github.com/aristanetworks/goarista/logger"
)

var hasMount = func(_ string, _ logger.Logger) bool {
	return true
}

// NewNSListener creates a new net.Listener bound to a network namespace. The listening socket will
// be bound to the specified local address and will have the specified tos.
func NewNSListener(nsName string, addr *net.TCPAddr, tos byte,
	l logger.Logger) (net.Listener, error) {
	return NewNSListenerWithCustomListener(nsName, addr, l,
		func() (net.Listener, error) {
			return dscp.ListenTCPWithTOSLogger(addr, tos, l)
		})
}

// NewNSListenerWithCustomListener creates a new net.Listener bound to a network namespace. The
// listener is created using listenerCreator. listenerCreator should create a listener that
// binds to addr.
func NewNSListenerWithCustomListener(nsName string, addr *net.TCPAddr, logger logger.Logger,
	listenerCreator ListenerCreator) (net.Listener, error) {
	return makeListener(nsName, listenerCreator)
}
