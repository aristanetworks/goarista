// Copyright (c) 2019 Arista Networks, Inc.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the COPYING file.

package netns

import (
	"errors"
	"net"
	"sync"

	"github.com/aristanetworks/goarista/dscp"
	"github.com/aristanetworks/goarista/logger"
)

type ListenerCreator func() (net.Listener, error)

func defaultListenerCreator(addr *net.TCPAddr, tos byte, logger logger.Logger) ListenerCreator {
	return func() (net.Listener, error) {
		return dscp.ListenTCPWithTOSLogger(addr, tos, logger)
	}
}

type NSListenerOption func(*nsListener)

// WithCustomListener makes it so the ListenerCreator passed in is called to create the internal
// net.Listener. If this isn't used, a dscp.ListenTCPWithTOSLogger is created
func WithCustomListener(lc ListenerCreator) NSListenerOption {
	return func(l *nsListener) {
		l.listenerCreator = lc
	}
}

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

// nsListener is a net.Listener that binds to a specific network namespace when it becomes
// available and in case it gets deleted and recreated it will automatically bind to the newly
// created namespace.
type nsListener struct {
	listener        net.Listener
	listenerMutex   sync.Mutex
	nsWatcher       NsWatcher
	nsName          string
	addr            *net.TCPAddr
	conns           chan net.Conn
	logger          logger.Logger
	listenerCreator ListenerCreator
}

func (l *nsListener) NetNsTeardown() {
	l.listenerMutex.Lock()
	defer l.listenerMutex.Unlock()
	if l.listener != nil {
		l.logger.Info("Destroying listener")
		l.listener.Close()
		l.listener = nil
	}
}

func (l *nsListener) NetNsOperation() error {
	l.listenerMutex.Lock()
	defer l.listenerMutex.Unlock()
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

func newNSListener(nsName string, addr *net.TCPAddr, tos byte, logger logger.Logger,
	options ...NSListenerOption) (net.Listener, error) {

	l := &nsListener{
		nsName: nsName,
		addr:   addr,
		logger: logger,
	}
	for _, opt := range options {
		opt(l)
	}
	if l.listenerCreator == nil {
		l.listenerCreator = defaultListenerCreator(addr, tos, logger)
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

// Close closes the listener. This MUST be called before the nslistener is garbage collected or
// watchers will be leaked
func (l *nsListener) Close() error {
	l.nsWatcher.Close()
	return nil
}

// Addr returns the local address of the listener.
func (l *nsListener) Addr() net.Addr {
	l.listenerMutex.Lock()
	defer l.listenerMutex.Unlock()
	if l.listener != nil {
		return l.listener.Addr()
	} else {
		return l.addr
	}
}

// NewNSListener creates a new net.Listener bound to a network namespace. If the default
// ListenerCreator is used, the listening socket will be bound to the specified local address and
// will have the specified tos.
//
// NewNSListener supports the following configuration options:
//
//	WithCustomListener - function used to create the listener
func NewNSListener(nsName string, addr *net.TCPAddr, tos byte, logger logger.Logger,
	options ...NSListenerOption) (net.Listener, error) {
	return newNSListener(nsName, addr, tos, logger, options...)
}
