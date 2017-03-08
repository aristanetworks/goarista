// Copyright (C) 2017  Arista Networks, Inc.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the COPYING file.

package main

import (
	"net"

	kcp "github.com/xtaci/kcp-go"
)

type udpClient struct {
	addr string
	conn net.Conn
}

func newUDPClient(addr string) OpenTSDBConn {
	return &udpClient{
		addr: addr,
	}
}

func (c *udpClient) Put(d *DataPoint) error {
	var err error
	if c.conn == nil {
		c.conn, err = kcp.DialWithOptions(c.addr, nil, 10, 3)
		if err != nil {
			return err
		}
	}
	_, err = c.conn.Write([]byte(d.String()))
	if err != nil {
		c.conn.Close()
		c.conn = nil
	}
	return err
}
