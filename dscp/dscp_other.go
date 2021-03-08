// Copyright (c) 2021 Arista Networks, Inc.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the COPYING file.

//go:build !linux && !darwin
// +build !linux,!darwin

package dscp

import (
	"errors"
	"net"
	"syscall"
)

// ListenTCPWithTOS is similar to net.ListenTCP but with the socket configured
// to the use the given ToS (Type of Service), to specify DSCP / ECN / class
// of service flags to use for incoming connections.
func ListenTCPWithTOS(address *net.TCPAddr, tos byte) (*net.TCPListener, error) {
	if tos != 0 {
		return nil, errors.New("TOS is not supported by this library on this platform")
	}
	return net.ListenTCP("tcp", address)
}

// SetTOS will set the TOS byte on a unix system. It's intended to be
// used in a net.Dialer's Control function.
func SetTOS(network string, c syscall.RawConn, tos byte) error {
	if tos != 0 {
		return errors.New("TOS is not supported by this library on this platform")
	}
	return nil
}
