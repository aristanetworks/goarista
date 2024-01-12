// Copyright (c) 2024 Arista Networks, Inc.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the COPYING file.

package netns

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/aristanetworks/fsnotify"
	"github.com/aristanetworks/goarista/logger"
)

func (w *nsWatcher) waitForMount() bool {
	for !hasMount(w.nsFile, w.logger) {
		time.Sleep(time.Second)
		if _, err := os.Stat(w.nsFile); err != nil {
			w.logger.Infof("error stating %s: %v", w.nsFile, err)
			return false
		}
	}
	return true
}

type NetNsOperator interface {
	NetNsOperation() error
	NetNsOperationSuccess()
	NetNsTeardown()
}

type NsWatcher interface {
	Close()
}

// nsWatcher can be used to perform an operation (e.g. opening a socket) in a network
// namespace.
type nsWatcher struct {
	watcher       *fsnotify.Watcher
	nsName        string
	nsFile        string
	done          chan struct{}
	logger        logger.Logger
	netNsOperator NetNsOperator
}

func (w *nsWatcher) setUp() bool {
	w.logger.Infof("Performing operation in namespace %v", w.nsName)
	if err := w.watcher.Add(w.nsFile); err != nil {
		w.logger.Infof("Can't watch the file (will try again): %v", err)
		return false
	}
	err := Do(w.nsName, func() error {
		err := w.netNsOperator.NetNsOperation()
		return err
	})
	if err != nil {
		w.logger.Infof("Namespace operation failed (will try again): %v", err)
		return false
	}
	w.netNsOperator.NetNsOperationSuccess()
	return true
}

func (w *nsWatcher) watch() {
	var mounted bool
	if hasMount(w.nsFile, w.logger) {
		mounted = w.setUp()
	}

	for {
		select {
		case <-w.done:
			w.netNsOperator.NetNsTeardown()
			go func() {
				// Drain the events, otherwise closing the watcher will get stuck
				for range w.watcher.Events {
				}
			}()
			w.watcher.Close()
			return
		case ev := <-w.watcher.Events:
			if ev.Name != w.nsFile {
				continue
			}
			if ev.Op&fsnotify.Create == fsnotify.Create {
				if mounted || !w.waitForMount() {
					continue
				}
				mounted = w.setUp()
			}
			if ev.Op&fsnotify.Remove == fsnotify.Remove {
				if !mounted {
					continue
				}
				w.netNsOperator.NetNsTeardown()
				mounted = false
			}
		}
	}
}

func (w *nsWatcher) setupWatch() error {
	fsWatcher, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}
	if err = fsWatcher.Add(filepath.Dir(w.nsFile)); err != nil {
		return err
	}

	w.watcher = fsWatcher
	go w.watch()
	return nil
}

func newNsWatcherWithDir(nsDir, nsName string, logger logger.Logger,
	netNsOperator NetNsOperator) (*nsWatcher, error) {
	if netNsOperator == nil {
		return nil, fmt.Errorf("Received nil netNsOperator")
	}
	w := &nsWatcher{
		nsName:        nsName,
		nsFile:        filepath.Join(nsDir, nsName),
		done:          make(chan struct{}),
		logger:        logger,
		netNsOperator: netNsOperator,
	}
	if err := w.setupWatch(); err != nil {
		return nil, err
	}

	return w, nil
}

func (w *nsWatcher) Close() {
	close(w.done)
}

func hasMountInProcMounts(r io.Reader, mountPoint string) bool {
	// Kernels up to 3.18 export the namespace via procfs and later ones via nsfs
	fsTypes := map[string]bool{"proc": true, "nsfs": true}

	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		l := scanner.Text()
		comps := strings.SplitN(l, " ", 3)
		if len(comps) != 3 || !fsTypes[comps[0]] {
			continue
		}
		if comps[1] == mountPoint {
			return true
		}
	}

	return false
}

func getNsDirFromProcMounts(r io.Reader) (string, error) {
	// Newer EOS versions mount netns under /run
	dirs := map[string]bool{"/var/run/netns": true, "/run/netns": true}

	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		l := scanner.Text()
		comps := strings.SplitN(l, " ", 3)
		if len(comps) != 3 || !dirs[comps[1]] {
			continue
		}
		return comps[1], nil
	}

	return "", errors.New("can't find the netns mount dir")
}

// defaultNsWatcher is used in cases where we don't require a netns, or on systems
// where netns support is not present. In these cases we don't watch anything - we
// just immediately invoke the callback functions.
type defaultNsWatcher struct {
	netNsOperator NetNsOperator
}

func (w *defaultNsWatcher) setupWatch() error {
	if err := w.netNsOperator.NetNsOperation(); err != nil {
		return err
	}
	w.netNsOperator.NetNsOperationSuccess()
	return nil
}

func (w *defaultNsWatcher) Close() {
	w.netNsOperator.NetNsTeardown()
}

func newDefaultNsWatcher(netNsOperator NetNsOperator) (NsWatcher, error) {
	if netNsOperator == nil {
		return nil, fmt.Errorf("Received nil netNsOperator")
	}
	w := &defaultNsWatcher{netNsOperator: netNsOperator}
	if err := w.setupWatch(); err != nil {
		return nil, err
	}
	return w, nil
}
