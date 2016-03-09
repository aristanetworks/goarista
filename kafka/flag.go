// Copyright (C) 2016  Arista Networks, Inc.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the COPYING file.

package kafka

import (
	"flag"
)

// Addresses is the flag for kafka's comma-separated addresses
var Addresses = flag.String("kafka", "localhost:9092", "kafka's comma-separated addresses")
