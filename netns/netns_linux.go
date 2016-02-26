// Copyright (C) 2016  Arista Networks, Inc.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the COPYING file.

package netns

import (
	"fmt"
	"os"
	"runtime"

	"golang.org/x/sys/unix"
)

const (
	defaultNs   = "default"
	netNsRunDir = "/var/run/netns/"
	selfNsFile  = "/proc/self/ns/net"
)

// close closes the file descriptor mapped to a network namespace
func (h nsHandle) close() error {
	return unix.Close(int(h))
}

// fd returns the handle as a uintptr
func (h nsHandle) fd() int {
	return int(h)
}

// getNs returns a file descriptor mapping to the given network namespace
var getNs = func(nsName string) (handle, error) {
	fd, err := unix.Open(nsName, unix.O_RDONLY, 0)
	return nsHandle(fd), err
}

// setNs sets the process's network namespace
var setNs = func(h handle) error {
	return unix.Setns(h.fd(), unix.CLONE_NEWNET)
}

// setNsByName wraps setNs, allowing specification of the network namespace by name.
// It returns the file descriptor mapped to the given network namespace.
func setNsByName(nsName string) error {
	netPath := netNsRunDir + nsName
	netNsHandle, err := getNs(netPath)
	defer netNsHandle.close()
	if err != nil {
		return fmt.Errorf("Failed to getNs: %s", err)
	}
	if err := setNs(netNsHandle); err != nil {
		return fmt.Errorf("Failed to setNs: %s", err)
	}
	return nil
}

// Do takes a function which it will call in the network namespace specified by destNs.
// The goroutine that calls this will lock itself to its current OS thread, hop
// namespaces, call the given function, hop back to its original namespace, and then
// unlock itself from its current OS thread.
// Do returns an error if an error occurs at any point besides in the invocation of
// the given function.
// The caller should check both the error of Do and any errors from the given function call.
func Do(destNs string, cb Callback) error {
	// If destNS is empty or defaultNS, the function is called in the caller's namespace
	if destNs == "" || destNs == defaultNs {
		cb()
		return nil
	}

	// Get the file descriptor to the current namespace
	currNsFd, err := getNs(selfNsFile)
	if os.IsNotExist(err) {
		return fmt.Errorf("File descriptor to current namespace does not exist: %s", err)
	} else if err != nil {
		return fmt.Errorf("Failed to open %s: %s", selfNsFile, err)
	}
	defer currNsFd.close()

	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	// Jump to the new network namespace
	if err := setNsByName(destNs); err != nil {
		return fmt.Errorf("Failed to set the namespace to %s: %s", destNs, err)
	}

	// Call the given function
	cb()

	// Come back to the original namespace
	if err = setNs(currNsFd); err != nil {
		return fmt.Errorf("Failed to return to the original namespace: %s", err)
	}

	return nil
}
