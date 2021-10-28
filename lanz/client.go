// Copyright (c) 2016 Arista Networks, Inc.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the COPYING file.

// Package lanz implements a LANZ client that will listen to notofications from LANZ streaming
// server and will decode them and send them as a protobuf over a channel to a receiver.
package lanz

import (
	"bufio"
	"encoding/binary"
	"io"
	"net"
	"sync"
	"time"

	pb "github.com/aristanetworks/goarista/lanz/proto"

	"github.com/aristanetworks/glog"
	"google.golang.org/protobuf/proto"
)

const (
	defaultConnectTimeout = 10 * time.Second
	defaultConnectBackoff = 30 * time.Second
)

// Client is the LANZ client interface.
type Client interface {
	// Run is the main loop of the client.
	// It connects to the LANZ server and reads the notifications, decodes them
	// and sends them to the channel.
	// In case of disconnect, it will reconnect automatically.
	Run(ch chan<- *pb.LanzRecord)
	// Stops the client.
	Stop()
}

// ConnectReadCloser extends the io.ReadCloser interface with a Connect method.
type ConnectReadCloser interface {
	io.ReadCloser
	// Connect connects to the address, returning an error if it fails.
	Connect() error
}

type client struct {
	sync.Mutex
	addr      string
	stop      chan struct{}
	connected bool
	timeout   time.Duration
	backoff   time.Duration
	conn      ConnectReadCloser
}

// New creates a new client with default TCP connection to the LANZ server.
func New(opts ...Option) Client {
	c := &client{
		stop:    make(chan struct{}),
		timeout: defaultConnectTimeout,
		backoff: defaultConnectBackoff,
	}

	for _, opt := range opts {
		opt(c)
	}

	if c.conn == nil {
		if c.addr == "" {
			panic("Neither address, nor connector specified")
		}
		c.conn = &netConnector{
			addr:    c.addr,
			timeout: c.timeout,
			backoff: c.backoff,
		}
	}

	return c
}

func (c *client) setConnected(connected bool) {
	c.Lock()
	defer c.Unlock()
	if c.connected && !connected {
		c.conn.Close()
	}
	c.connected = connected
}

func (c *client) Run(ch chan<- *pb.LanzRecord) {
	go func() {
		<-c.stop
		c.setConnected(false)
	}()

	defer func() {
		close(ch)
		// This is to handle a race when the connection is
		// established, but not marked as connected yet and is then
		// preempted with c.stop closing.
		c.setConnected(false)
	}()

	for {
		select {
		case <-c.stop:
			return
		default:
			if err := c.conn.Connect(); err != nil {
				select {
				case <-c.stop:
					return
				default:
					glog.V(1).Infof("Can't connect to LANZ server: %v", err)
					time.Sleep(c.backoff)
					continue
				}
			}
			glog.V(1).Infof("Connected successfully to LANZ server: %v", c.addr)
			c.setConnected(true)
			if err := c.read(bufio.NewReader(c.conn), ch); err != nil {
				select {
				case <-c.stop:
					return
				default:
					if err != io.EOF && err != io.ErrUnexpectedEOF {
						glog.Errorf("Error receiving LANZ events: %v", err)
					}
					c.setConnected(false)
					time.Sleep(c.backoff)
				}
			}
		}
	}

}

func (c *client) read(r *bufio.Reader, ch chan<- *pb.LanzRecord) error {
	for {
		select {
		case <-c.stop:
			return nil
		default:
			len, err := binary.ReadUvarint(r)
			if err != nil {
				return err
			}

			buf := make([]byte, len)
			if _, err = io.ReadFull(r, buf); err != nil {
				return err
			}

			rec := &pb.LanzRecord{}
			if err = proto.Unmarshal(buf, rec); err != nil {
				return err
			}

			ch <- rec
		}
	}
}

func (c *client) Stop() {
	close(c.stop)
}

type netConnector struct {
	net.Conn
	addr    string
	timeout time.Duration
	backoff time.Duration
}

func (c *netConnector) Connect() (err error) {
	c.Conn, err = net.DialTimeout("tcp", c.addr, c.timeout)
	if err != nil {
	}
	return
}
