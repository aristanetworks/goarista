// Copyright (C) 2017  Arista Networks, Inc.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the COPYING file.

// Package dscp provides helper functions to apply DSCP / ECN / CoS flags to sockets.
package dscp

import (
	"net"
	"reflect"
)

// DialTCPWithTOS is similar to net.DialTCP but with the socket configured
// to the use the given ToS (Type of Service), to specify DSCP / ECN / class
// of service flags to use for incoming connections.
func DialTCPWithTOS(laddr, raddr *net.TCPAddr, tos byte) (*net.TCPConn, error) {
	conn, err := net.DialTCP("tcp", laddr, raddr)
	if err != nil {
		return nil, err
	}
	value := reflect.ValueOf(conn)
	if err = setTOS(raddr, value, tos); err != nil {
		conn.Close()
		return nil, err
	}
	return conn, err
}
