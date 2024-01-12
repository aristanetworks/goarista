// Copyright (c) 2024 Arista Networks, Inc.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the COPYING file.

package netns

import (
	"errors"
	"os"

	"github.com/aristanetworks/goarista/logger"
)

var hasMount = func(mountPoint string, logger logger.Logger) bool {
	fd, err := os.Open("/proc/mounts")
	if err != nil {
		logger.Fatal("can't open /proc/mounts")
	}
	defer fd.Close()

	return hasMountInProcMounts(fd, mountPoint)
}

func getNsDir() (string, error) {
	fd, err := os.Open("/proc/mounts")
	if err != nil {
		return "", errors.New("can't open /proc/mounts")
	}
	defer fd.Close()

	return getNsDirFromProcMounts(fd)
}

// NewNsWatcher constructs an NsWatcher
func NewNsWatcher(nsName string, logger logger.Logger, netNsOperator NetNsOperator) (NsWatcher, error) {
	// The default namespace doesn't get recreated and avoid the watcher helps with environments
	// that aren't setup for multiple namespaces (eg inside containers)
	if nsName == "" || nsName == "default" {
		return newDefaultNsWatcher(netNsOperator)
	}
	nsDir, err := getNsDir()
	if err != nil {
		return nil, err
	}
	return newNsWatcherWithDir(nsDir, nsName, logger, netNsOperator)
}
