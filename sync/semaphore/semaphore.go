// Copyright (c) 2024 Arista Networks, Inc.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the COPYING file.

package semaphore

import (
	"context"
	"sync"

	"golang.org/x/sync/semaphore"
)

// Weighted is a wrapper around the semaphore that tracks available weight
type Weighted struct {
	sem           *semaphore.Weighted
	maxWeight     int64
	currentWeight int64
	mu            sync.Mutex
}

// NewWeighted initializes a new weighted semaphore with a given capacity
func NewWeighted(maxWeight int64) *Weighted {
	return &Weighted{
		sem:           semaphore.NewWeighted(maxWeight),
		maxWeight:     maxWeight,
		currentWeight: maxWeight,
	}
}

// Acquire tries to acquire the specified weight
func (w *Weighted) Acquire(ctx context.Context, weight int64) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	err := w.sem.Acquire(ctx, weight)
	if err == nil {
		w.currentWeight -= weight
	}

	return err
}

// Release releases the specified weight back to the semaphore
func (w *Weighted) Release(weight int64) {
	w.mu.Lock()
	defer w.mu.Unlock()

	w.sem.Release(weight)
	w.currentWeight += weight
}

// Available returns the current available weight
func (w *Weighted) Available() int64 {
	w.mu.Lock()
	defer w.mu.Unlock()

	return w.currentWeight
}
