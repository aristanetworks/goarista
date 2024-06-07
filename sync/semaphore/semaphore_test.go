// Copyright (c) 2024 Arista Networks, Inc.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the COPYING file.

package semaphore_test

import (
	"context"
	"sync"
	"testing"

	"github.com/aristanetworks/goarista/sync/semaphore"
)

func acquire(t *testing.T, w *semaphore.Weighted, weight int64) {
	if err := w.Acquire(context.Background(), weight); err != nil {
		t.Fatalf("Failed to acquire semaphore: %v", err)
	}
}

func TestAvailable(t *testing.T) {
	available := int64(10)
	ws := semaphore.NewWeighted(available)
	acquire(t, ws, 1)
	available -= 1
	if ws.Available() != available {
		t.Fatalf("expected %d available but got %d", available, ws.Available())
	}
	wg := sync.WaitGroup{}
	wg.Add(2)
	for i := 0; i < 2; i++ {
		go func() {
			acquire(t, ws, 4)
			wg.Done()
		}()
	}
	wg.Wait()
	available -= 4 * 2
	if ws.Available() != available {
		t.Fatalf("expected %d available but got %d", available, ws.Available())
	}
}
