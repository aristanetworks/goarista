// Copyright (c) 2019 Arista Networks, Inc.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the COPYING file.

package netns

import (
	"errors"
	"net"
	"sync"
	"testing"

	"github.com/aristanetworks/goarista/glog"
	"github.com/aristanetworks/goarista/logger"
)

type mockListener struct {
	makes      int
	accepts    int
	closes     int
	maxAccepts int
	stop       chan struct{}
	done       chan struct{}
}

func (l *mockListener) Accept() (net.Conn, error) {
	if l.accepts >= l.maxAccepts {
		<-l.stop
		return nil, errors.New("closed")
	}
	l.accepts++
	return nil, nil
}

func (l *mockListener) Close() error {
	l.closes++
	close(l.stop)
	close(l.done)
	return nil
}

func (l *mockListener) Addr() net.Addr {
	return nil
}

var currentMockListener struct {
	mu       sync.RWMutex
	listener *mockListener
}

func makeMockListener(n int) func() (net.Listener, error) {
	return func() (net.Listener, error) {
		currentMockListener.mu.Lock()
		defer currentMockListener.mu.Unlock()
		currentMockListener.listener = &mockListener{
			maxAccepts: n,
			stop:       make(chan struct{}),
			done:       make(chan struct{}),
		}
		currentMockListener.listener.makes++
		return currentMockListener.listener, nil
	}
}

type mockNsWatcher struct {
	netNsOperator NetNsOperator
}

func (w *mockNsWatcher) Start() error {
	return nil
}

func (w *mockNsWatcher) Close() {
	// Do nothing
}

func TestNSListener(t *testing.T) {
	mockListenerCreator := makeMockListener(1)
	var nsWatcher *mockNsWatcher
	newNsWatcher = func(nsName string, logger logger.Logger,
		netNsOperator NetNsOperator) (NsWatcher, error) {
		nsWatcher = &mockNsWatcher{netNsOperator: netNsOperator}
		return nsWatcher, nil
	}

	logger := &glog.Glog{}
	l, err := NewNSListener("", nil, 0, logger, WithCustomListener(mockListenerCreator))
	if err != nil {
		t.Fatalf("Can't create mock listener: %v", err)
	}

	var listener *mockListener
	for i := 1; i <= 3; i++ {
		err := nsWatcher.netNsOperator.NetNsOperation()
		if err != nil {
			t.Fatalf("Couldn't perform namespace operation: %v", err)
		}
		nsWatcher.netNsOperator.NetNsOperationSuccess()
		if _, err = l.Accept(); err != nil {
			t.Fatalf("Unexpected accept error: %v", err)
		}

		currentMockListener.mu.RLock()
		if listener == currentMockListener.listener {
			t.Fatalf("%v: listener hasn't changed", i)
		}
		listener = currentMockListener.listener
		currentMockListener.mu.RUnlock()

		nsWatcher.netNsOperator.NetNsTeardown()
		<-listener.done

		if listener.makes != 1 {
			t.Fatalf("%v: Expected makeListener to be called once, but it was called %v times", i,
				listener.makes)
		}
		if listener.accepts != 1 {
			t.Fatalf("%v: Expected accept to be called once, but it was called %v times", i,
				listener.accepts)
		}
		if listener.closes != 1 {
			t.Fatalf("%v: Expected close to be called once, but it was called %v times", i,
				listener.closes)
		}
	}

	l.Close()
}

func TestNSListenerClose(t *testing.T) {
	mockListenerCreator := makeMockListener(1)

	var nsWatcher *mockNsWatcher
	newNsWatcher = func(nsName string, logger logger.Logger,
		netNsOperator NetNsOperator) (NsWatcher, error) {
		nsWatcher = &mockNsWatcher{netNsOperator: netNsOperator}
		return nsWatcher, nil
	}

	logger := &glog.Glog{}
	l, err := NewNSListener("", nil, 0, logger, WithCustomListener(mockListenerCreator))
	if err != nil {
		t.Fatalf("Can't create mock listener: %v", err)
	}

	err = nsWatcher.netNsOperator.NetNsOperation()
	if err != nil {
		t.Fatalf("Couldn't perform namespace operation: %v", err)
	}
	nsWatcher.netNsOperator.NetNsOperationSuccess()
	defer nsWatcher.netNsOperator.NetNsTeardown()

	done := make(chan struct{})
	go func() {
		_, err := l.Accept()
		if err != nil {
			t.Errorf("Couldn't accept: %v", err)
		}
		close(done)
	}()
	<-done
	l.Close()
}
