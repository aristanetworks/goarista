// Copyright (c) 2019 Arista Networks, Inc.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the COPYING file.

package netns

import (
	"errors"
	"fmt"
	"net"

	"github.com/aristanetworks/goarista/dscp"
	"github.com/aristanetworks/goarista/logger"
)

type ListenerCreator func() (net.Listener, error)

func accept(listener net.Listener, conns chan<- net.Conn, logger logger.Logger) {
	for {
		c, err := listener.Accept()
		if err != nil {
			logger.Infof("Accept error: %v", err)
			close(conns)
			return
		}
		conns <- c
	}
}

// nsListener is a net.Listener that binds to a specific network namespace when it becomes available
// and in case it gets deleted and recreated it will automatically bind to the newly created
// namespace.
type nsListener struct {
	listener        net.Listener
	nsWatcher       NsWatcher
	nsName          string
	addr            *net.TCPAddr
	conns           chan net.Conn
	logger          logger.Logger
	listenerCreator ListenerCreator
}

func (l *nsListener) NetNsTeardown() {
	if l.listener != nil {
		l.logger.Info("Destroying listener")
		l.listener.Close()
		l.listener = nil
	}
}

func (l *nsListener) NetNsOperation() error {
	listener, err := l.listenerCreator()
	l.listener = listener
	return err
}

func (l *nsListener) NetNsOperationSuccess() {
	l.conns = make(chan net.Conn)
	go accept(l.listener, l.conns, l.logger)
}

var newNsWatcher = func(nsName string, logger logger.Logger,
	netNsOperator NetNsOperator) (NsWatcher, error) {
	return NewNsWatcher(nsName, logger, netNsOperator)
}

func newNSListener(nsName string, addr *net.TCPAddr, logger logger.Logger,
	listenerCreator ListenerCreator) (net.Listener, error) {
	if listenerCreator == nil {
		return nil, fmt.Errorf("newNSListener received nil listenerCreator")
	}

	l := &nsListener{
		nsName:          nsName,
		addr:            addr,
		logger:          logger,
		listenerCreator: listenerCreator,
	}

	nsWatcher, err := newNsWatcher(nsName, logger, l)
	if err != nil {
		return nil, err
	}
	l.nsWatcher = nsWatcher

	return l, nil
}

// Accept accepts a connection on the listener socket.
func (l *nsListener) Accept() (net.Conn, error) {
	if c, ok := <-l.conns; ok {
		return c, nil
	}
	return nil, errors.New("listener closed")
}

// Close closes the listener.
func (l *nsListener) Close() error {
	l.nsWatcher.Close()
	return nil
}

// Addr returns the local address of the listener.
func (l *nsListener) Addr() net.Addr {
	if l.listener != nil {
		return l.listener.Addr()
	} else {
		return l.addr
	}
}

// NewNSListener creates a new net.Listener bound to a network namespace. The listening socket will
// be bound to the specified local address and will have the specified tos.
func NewNSListener(nsName string, addr *net.TCPAddr, tos byte, logger logger.Logger) (net.Listener,
	error) {
	return NewNSListenerWithCustomListener(nsName, addr, logger,
		func() (net.Listener, error) {
			return dscp.ListenTCPWithTOSLogger(addr, tos, logger)
		})
}

// NewNSListenerWithCustomListener creates a new net.Listener bound to a network namespace. The
// listener is created using listenerCreator. listenerCreator should create a listener that
// binds to addr. listenerCreator may be called multiple times if the vrf is deleted and recreated.
func NewNSListenerWithCustomListener(nsName string, addr *net.TCPAddr, logger logger.Logger,
	listenerCreator ListenerCreator) (net.Listener, error) {
	return newNSListener(nsName, addr, logger, listenerCreator)
}
