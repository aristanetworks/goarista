// Copyright (c) 2019 Arista Networks, Inc.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the COPYING file.

//go:build !linux
// +build !linux

package netns

import (
	"net"

	"github.com/aristanetworks/goarista/logger"
)

var hasMount = func(_ string, _ logger.Logger) bool {
	return true
}

// NewNSListener creates a new net.Listener bound to a network namespace. The listening socket will
// be bound to the specified local address and will have the specified tos.
func NewNSListener(nsName string, addr *net.TCPAddr, tos byte,
	_ logger.Logger) (net.Listener, error) {
	return makeListener(nsName, addr, tos)
}
