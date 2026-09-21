// Copyright (c) 2016 Arista Networks, Inc.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the COPYING file.

package netns

import (
	"bytes"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

type mockHandle int

func (mh mockHandle) close() error {
	return nil
}

func (mh mockHandle) fd() int {
	return 0
}

func TestNetNs(t *testing.T) {
	setNsCallCount := 0

	// Mock getNs
	oldGetNs := getNs
	getNs = func(nsName string) (handle, error) {
		return mockHandle(1), nil
	}
	defer func() {
		getNs = oldGetNs
	}()

	// Mock setNs
	oldSetNs := setNs
	setNs = func(fd handle) error {
		setNsCallCount++
		return nil
	}
	defer func() {
		setNs = oldSetNs
	}()

	// Create a tempfile so we can use its name for the network namespace
	tmpfile, err := ioutil.TempFile("", "")
	if err != nil {
		t.Fatalf("Failed to create a temp file: %s", err)
	}
	defer os.Remove(tmpfile.Name())
	nsName := filepath.Base(tmpfile.Name())

	// Map of network namespace name to the number of times it should call setNs
	cases := map[string]int{"": 0, "default": 2, nsName: 2}
	for name, callCount := range cases {
		var cbResult string
		err = Do(name, func() error {
			cbResult = "Hello" + name
			return nil
		})
		if err != nil {
			t.Fatalf("Error calling function in different network namespace: %s", err)
		}
		if cbResult != "Hello"+name {
			t.Fatalf("Failed to call the callback function")
		}
		if setNsCallCount != callCount {
			t.Fatalf("setNs should have been called %d times for %s, but was called %d times",
				callCount, name, setNsCallCount)
		}
		setNsCallCount = 0
	}
}

type testHandle string

func (h testHandle) close() error { return nil }
func (h testHandle) fd() int      { return 0 }

// TestDoPinsBeforeCapturingCurrentNamespace verifies that Do pins its
// goroutine before inspecting the current namespace and does not unpin it
// until after restoring that namespace.
func TestDoPinsBeforeCapturingCurrentNamespace(t *testing.T) {
	oldGetNs, oldSetNs := getNs, setNs
	oldLock, oldUnlock := lockOSThread, unlockOSThread
	t.Cleanup(func() {
		getNs, setNs = oldGetNs, oldSetNs
		lockOSThread, unlockOSThread = oldLock, oldUnlock
	})

	var events []string
	lockOSThread = func() {
		events = append(events, "lock")
	}
	unlockOSThread = func() {
		events = append(events, "unlock")
	}
	getNs = func(path string) (handle, error) {
		switch path {
		case threadSelfNsFile:
			events = append(events, "capture-current")
			return testHandle("current"), nil
		case netNsRunDir + "target":
			events = append(events, "open-target")
			return testHandle("target"), nil
		default:
			return nil, fmt.Errorf("unexpected namespace path %q", path)
		}
	}
	setNs = func(h handle) error {
		events = append(events, "set-"+string(h.(testHandle)))
		return nil
	}

	if err := Do("target", func() error {
		events = append(events, "callback")
		return nil
	}); err != nil {
		t.Fatalf("Do returned an error: %v", err)
	}

	want := []string{
		"lock",
		"capture-current",
		"open-target",
		"set-target",
		"callback",
		"set-current",
		"unlock",
	}
	if !reflect.DeepEqual(events, want) {
		t.Fatalf("event order = %v, want %v", events, want)
	}
}

// TestDoFallsBackToCurrentTIDNamespace verifies that kernels without
// /proc/thread-self use the calling thread's explicit TID, obtained only
// after the goroutine is pinned.
func TestDoFallsBackToCurrentTIDNamespace(t *testing.T) {
	oldGetNs, oldSetNs := getNs, setNs
	oldLock, oldUnlock := lockOSThread, unlockOSThread
	oldFallback := currentThreadNsFallbackFile
	t.Cleanup(func() {
		getNs, setNs = oldGetNs, oldSetNs
		lockOSThread, unlockOSThread = oldLock, oldUnlock
		currentThreadNsFallbackFile = oldFallback
	})

	locked := false
	lockOSThread = func() {
		locked = true
	}
	unlockOSThread = func() {
		locked = false
	}
	fallbackPath := "/proc/self/task/123/ns/net"
	currentThreadNsFallbackFile = func() string {
		if !locked {
			t.Fatal("fallback TID was obtained before LockOSThread")
		}
		return fallbackPath
	}

	var paths []string
	getNs = func(path string) (handle, error) {
		paths = append(paths, path)
		switch path {
		case threadSelfNsFile:
			return nil, os.ErrNotExist
		case fallbackPath:
			return testHandle("current"), nil
		case netNsRunDir + "target":
			return testHandle("target"), nil
		default:
			return nil, fmt.Errorf("unexpected namespace path %q", path)
		}
	}
	setNs = func(handle) error {
		return nil
	}

	if err := Do("target", func() error { return nil }); err != nil {
		t.Fatalf("Do returned an error: %v", err)
	}
	want := []string{
		threadSelfNsFile,
		fallbackPath,
		netNsRunDir + "target",
	}
	if !reflect.DeepEqual(paths, want) {
		t.Fatalf("namespace paths = %v, want %v", paths, want)
	}
	if locked {
		t.Fatal("goroutine remained locked after successful restoration")
	}
}

// TestDoRestoresCallingThreadNamespace models a concurrent namespace
// interleaving. The process leader is temporarily in the target namespace
// while a worker calls Do from the default namespace. Linux resolves
// /proc/self/ns/net through the leader, whereas /proc/thread-self/ns/net
// identifies the worker that Do will move.
func TestDoRestoresCallingThreadNamespace(t *testing.T) {
	const (
		defaultNS = testHandle("worker-default")
		leaderNS  = testHandle("leader-target")
		targetNS  = testHandle("target")
	)

	oldGetNs, oldSetNs := getNs, setNs
	t.Cleanup(func() {
		getNs, setNs = oldGetNs, oldSetNs
	})

	workerNS := defaultNS
	getNs = func(path string) (handle, error) {
		switch path {
		case "/proc/thread-self/ns/net":
			return workerNS, nil
		case "/proc/self/ns/net":
			return leaderNS, nil
		case netNsRunDir + "target":
			return targetNS, nil
		default:
			return nil, fmt.Errorf("unexpected namespace path %q", path)
		}
	}
	setNs = func(h handle) error {
		workerNS = h.(testHandle)
		return nil
	}

	if err := Do("target", func() error {
		if workerNS != targetNS {
			t.Fatalf("callback namespace = %q, want %q", workerNS, targetNS)
		}
		return nil
	}); err != nil {
		t.Fatalf("Do returned an error: %v", err)
	}
	if workerNS != defaultNS {
		t.Fatalf("worker restored to %q, want its original namespace %q",
			workerNS, defaultNS)
	}
}

// TestDoRestoresAfterCallbackPanic verifies that a recovered callback panic
// cannot leave its OS thread in the target namespace.
func TestDoRestoresAfterCallbackPanic(t *testing.T) {
	const (
		defaultNS = testHandle("default")
		targetNS  = testHandle("target")
	)

	oldGetNs, oldSetNs := getNs, setNs
	oldLock, oldUnlock := lockOSThread, unlockOSThread
	t.Cleanup(func() {
		getNs, setNs = oldGetNs, oldSetNs
		lockOSThread, unlockOSThread = oldLock, oldUnlock
	})

	lockCalls, unlockCalls := 0, 0
	lockOSThread = func() {
		lockCalls++
	}
	unlockOSThread = func() {
		unlockCalls++
	}
	currentNS := defaultNS
	getNs = func(path string) (handle, error) {
		if path == netNsRunDir+"target" {
			return targetNS, nil
		}
		return currentNS, nil
	}
	setNs = func(h handle) error {
		currentNS = h.(testHandle)
		return nil
	}

	panicValue := "callback panic"
	func() {
		defer func() {
			if got := recover(); got != panicValue {
				t.Fatalf("recovered %v, want %q", got, panicValue)
			}
		}()
		_ = Do("target", func() error {
			panic(panicValue)
		})
	}()

	if currentNS != defaultNS {
		t.Fatalf("worker namespace after panic = %q, want %q", currentNS, defaultNS)
	}
	if lockCalls != 1 {
		t.Fatalf("LockOSThread calls = %d, want 1", lockCalls)
	}
	if unlockCalls != 1 {
		t.Fatalf("UnlockOSThread calls = %d, want 1", unlockCalls)
	}
}

// TestDoQuarantinesThreadAfterRestoreFailure verifies that Do returns the
// restoration error without releasing the contaminated OS thread back to
// the Go scheduler.
func TestDoQuarantinesThreadAfterRestoreFailure(t *testing.T) {
	restoreErr := errors.New("restore failed")
	callbackErr := errors.New("callback failed")
	var logOutput bytes.Buffer
	oldLogOutput := log.Writer()
	log.SetOutput(&logOutput)
	t.Cleanup(func() {
		log.SetOutput(oldLogOutput)
	})

	oldGetNs, oldSetNs := getNs, setNs
	oldLock, oldUnlock := lockOSThread, unlockOSThread
	t.Cleanup(func() {
		getNs, setNs = oldGetNs, oldSetNs
		lockOSThread, unlockOSThread = oldLock, oldUnlock
	})

	lockCalls, unlockCalls := 0, 0
	lockOSThread = func() {
		lockCalls++
	}
	unlockOSThread = func() {
		unlockCalls++
	}
	getNs = func(path string) (handle, error) {
		switch path {
		case threadSelfNsFile:
			return testHandle("current"), nil
		case netNsRunDir + "target":
			return testHandle("target"), nil
		default:
			return nil, fmt.Errorf("unexpected namespace path %q", path)
		}
	}
	setCalls := 0
	setNs = func(h handle) error {
		setCalls++
		if setCalls == 2 {
			return restoreErr
		}
		return nil
	}

	err := Do("target", func() error {
		return callbackErr
	})
	if !errors.Is(err, restoreErr) {
		t.Fatalf("Do error = %v, want wrapped restoration error %v", err, restoreErr)
	}
	if !strings.Contains(err.Error(), callbackErr.Error()) {
		t.Fatalf("Do error = %v, want callback error context %v", err, callbackErr)
	}
	if !strings.Contains(logOutput.String(), err.Error()) {
		t.Fatalf("log output = %q, want restoration error %q", logOutput.String(), err)
	}
	if lockCalls != 1 {
		t.Fatalf("LockOSThread calls = %d, want 1", lockCalls)
	}
	if unlockCalls != 0 {
		t.Fatalf("UnlockOSThread calls = %d, want 0 after restoration failure", unlockCalls)
	}
}
