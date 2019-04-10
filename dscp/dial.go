// Copyright (c) 2017 Arista Networks, Inc.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the COPYING file.

// Package dscp provides helper functions to apply DSCP / ECN / CoS flags to sockets.
package dscp

import (
	"net"
	"syscall"
	"time"
)

// DialTCPWithTOS is similar to net.DialTCP but with the socket configured
// to the use the given ToS (Type of Service), to specify DSCP / ECN / class
// of service flags to use for incoming connections.
func DialTCPWithTOS(laddr, raddr *net.TCPAddr, tos byte) (*net.TCPConn, error) {
	d := net.Dialer{
		LocalAddr: laddr,
		Control: func(network, address string, c syscall.RawConn) error {
			return setTOS(network, c, tos)
		},
	}
	conn, err := d.Dial("tcp", raddr.String())
	if err != nil {
		return nil, err
	}
	return conn.(*net.TCPConn), err
}

// DialTimeoutWithTOS is similar to net.DialTimeout but with the socket configured
// to the use the given ToS (Type of Service), to specify DSCP / ECN / class
// of service flags to use for incoming connections.
func DialTimeoutWithTOS(network, address string, timeout time.Duration, tos byte) (net.Conn,
	error) {
	d := net.Dialer{
		Timeout: timeout,
		Control: func(network, address string, c syscall.RawConn) error {
			return setTOS(network, c, tos)
		},
	}
	conn, err := d.Dial(network, address)
	if err != nil {
		return nil, err
	}
	return conn, err
}

// DialTCPTimeoutWithTOS is same as DialTimeoutWithTOS except for enforcing "tcp" and
// providing an option to specify local address (source)
func DialTCPTimeoutWithTOS(laddr, raddr *net.TCPAddr, tos byte, timeout time.Duration) (net.Conn,
	error) {
	d := net.Dialer{
		Timeout:   timeout,
		LocalAddr: laddr,
		Control: func(network, address string, c syscall.RawConn) error {
			return setTOS(network, c, tos)
		},
	}
	conn, err := d.Dial("tcp", raddr.String())
	if err != nil {
		return nil, err
	}

	return conn, err
}
