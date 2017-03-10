// Copyright (C) 2017  Arista Networks, Inc.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the COPYING file.

package main

import (
	"net"

	"github.com/aristanetworks/glog"
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

type udpServer struct {
	lis    *kcp.Listener
	telnet *telnetClient
}

func newUDPServer(udpAddr, tsdbAddr string) (*udpServer, error) {
	lis, err := kcp.ListenWithOptions(udpAddr, nil, 10, 3)
	if err != nil {
		return nil, err
	}
	return &udpServer{
		lis:    lis,
		telnet: newTelnetClient(tsdbAddr).(*telnetClient),
	}, nil
}

func (c *udpServer) Run() error {
	for {
		conn, err := c.lis.AcceptKCP()
		if err != nil {
			return err
		}

		go func() {
			defer conn.Close()
			var buf [4096]byte
			for {
				n, err := conn.Read(buf[:])
				if err != nil {
					if n != 0 { // Not EOF
						glog.Error(err)
					}
					return
				}
				err = c.telnet.PutBytes(buf[:n])
				if err != nil {
					glog.Error(err)
					return
				}
			}
		}()
	}
}
