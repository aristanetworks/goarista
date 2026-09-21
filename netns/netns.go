// Copyright (c) 2016 Arista Networks, Inc.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the COPYING file.

// Package netns provides a utility function that allows a user to
// perform actions in a different network namespace
package netns

import (
	"fmt"
	"os"
	"runtime"

	"github.com/aristanetworks/goarista/logger"
)

const (
	netNsRunDir      = "/var/run/netns/"
	threadSelfNsFile = "/proc/thread-self/ns/net"
)

// Variables allow tests to verify that namespace capture happens only after
// the goroutine is pinned to an OS thread.
var (
	lockOSThread   = runtime.LockOSThread
	unlockOSThread = runtime.UnlockOSThread
)

// getCurrentThreadNs opens the calling thread's network namespace. It must be
// called only after the goroutine is pinned so both paths resolve to that
// stable OS thread.
func getCurrentThreadNs() (handle, string, error) {
	path := threadSelfNsFile
	ns, err := getNs(path)
	if os.IsNotExist(err) {
		// /proc/thread-self was added in Linux 3.17. Older kernels can
		// identify the same locked thread through its explicit TID.
		path = currentThreadNsFallbackFile()
		ns, err = getNs(path)
	}
	return ns, path, err
}

// Callback is a function that gets called in a given network namespace.
// The user needs to check any errors from any calls inside this function.
type Callback func() error

// File descriptor interface so we can mock for testing
type handle interface {
	close() error
	fd() int
}

// The file descriptor associated with a network namespace
type nsHandle int

// setNsByName wraps setNs, allowing specification of the network namespace by name.
// It returns the file descriptor mapped to the given network namespace.
func setNsByName(nsName string) error {
	netPath := netNsRunDir + nsName
	handle, err := getNs(netPath)
	if err != nil {
		return fmt.Errorf("Failed to getNs: %s", err)
	}
	err = setNs(handle)
	handle.close()
	if err != nil {
		return fmt.Errorf("Failed to setNs: %s", err)
	}
	return nil
}

// Do takes a function which it will call in the network namespace specified by nsName.
// The goroutine that calls this will lock itself to its current OS thread, hop
// namespaces, call the given function, hop back to its original namespace, and then
// unlock itself from its current OS thread.
// Do returns an error if an error occurs at any point besides in the invocation of
// the given function, or if the given function itself returns an error.
//
// The callback function is expected to do something simple such as just
// creating a socket / opening a connection, as it's not desirable to start
// complex logic in a goroutine that is pinned to the current OS thread.
// Also any goroutine started from the callback function may or may not
// execute in the desired namespace.
func Do(nsName string, cb Callback) (retErr error) {
	// If destNS is empty, the function is called in the caller's namespace
	if nsName == "" {
		return cb()
	}

	// Namespace membership is an OS-thread property. Pin first, then use
	// thread-self so the saved namespace belongs to the exact thread that
	// setNs below will move.
	lockOSThread()
	safeToUnlock := true
	defer func() {
		if safeToUnlock {
			unlockOSThread()
		}
	}()

	currNsFd, currNsPath, err := getCurrentThreadNs()
	if os.IsNotExist(err) {
		return fmt.Errorf("File descriptor to current namespace does not exist: %s", err)
	} else if err != nil {
		return fmt.Errorf("Failed to open %s: %s", currNsPath, err)
	}
	defer currNsFd.close()

	// Jump to the new network namespace
	if err := setNsByName(nsName); err != nil {
		return fmt.Errorf("Failed to set the namespace to %s: %s", nsName, err)
	}

	// The thread must not return to Go's scheduler between entering the target
	// namespace and successfully restoring its original namespace.
	safeToUnlock = false
	defer func() {
		if err := setNs(currNsFd); err != nil {
			// Keep the goroutine pinned if restoration fails. This quarantines
			// the OS thread from unrelated goroutines; Go terminates the thread
			// when the calling goroutine exits while still locked to it.
			retErr = fmt.Errorf(
				"Failed to return to the original namespace: %w (callback returned %v)",
				err, retErr)
			logger.Std.Errorf("%v; current goroutine remains locked to its OS thread", retErr)
			return
		}
		safeToUnlock = true
	}()

	// Restoration is deferred so it also runs while a callback panic unwinds.
	return cb()
}
