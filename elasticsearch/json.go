// Copyright (c) 2017 Arista Networks, Inc.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the COPYING file.

package elasticsearch

import (
	"github.com/aristanetworks/goarista/gnmi"
	pb "github.com/openconfig/gnmi/proto/gnmi"
)

// NotificationToMaps converts a gNMI Notification into a map[string][interface] that adheres
// to the Data schema defined in schema.go
func NotificationToMaps(datasetID string,
	notification *pb.Notification) ([]map[string]interface{}, error) {
	var requests []map[string]interface{}
	var trueVar = true

	timeStampNano := uint64(notification.Timestamp)

	prefix := notification.Prefix
	if prefix == nil {
		prefix = &pb.Path{}
	}

	for _, delete := range notification.Delete {
		path := gnmi.JoinPaths(prefix, delete)
		doc := map[string]interface{}{
			"Timestamp": timeStampNano,
			"DatasetID": datasetID,
			"Path":      gnmi.StrPath(path),
			"Del":       &trueVar,
		}

		keyStr := gnmi.StrPath(delete)
		doc["Key"] = []byte(keyStr) // use strigified delete.Path for key
		if err := SetKey(doc, keyStr); err != nil {
			return nil, err
		}

		requests = append(requests, doc)
	}
	for _, update := range notification.Update {
		key := update.Path
		path := gnmi.JoinPaths(prefix, key)
		doc := map[string]interface{}{
			"Timestamp": timeStampNano,
			"DatasetID": datasetID,
			"Path":      gnmi.StrPath(path),
		}
		keyStr := gnmi.StrPath(key)
		doc["Key"] = []byte(keyStr) // use strigified update.Path for key
		if err := SetKey(doc, keyStr); err != nil {
			return nil, err
		}
		if err := SetValue(doc, update.Val.Value); err != nil {
			return nil, err
		}
		requests = append(requests, doc)
	}

	return requests, nil
}
