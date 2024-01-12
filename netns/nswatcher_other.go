// Copyright (c) 2024 Arista Networks, Inc.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the COPYING file.

//go:build !linux
// +build !linux

package netns

import (
	"github.com/aristanetworks/goarista/logger"
)

var hasMount = func(_ string, _ logger.Logger) bool {
	return true
}

// NewNsWatcher constructs an NsWatcher
func NewNsWatcher(nsName string, logger logger.Logger, netNsOperator NetNsOperator) (NsWatcher, error) {
	return newDefaultNsWatcher(netNsOperator)
}
