// Copyright (C) 2016  Arista Networks, Inc.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the COPYING file.

package openconfig

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

func joinPath(path *Path) string {
	return strings.Join(path.Element, "/")
}

func convertUpdate(update *Update) (interface{}, error) {
	switch update.Value.Type {
	case Type_JSON:
		var value interface{}
		err := json.Unmarshal(update.Value.Value, &value)
		if err != nil {
			return nil, fmt.Errorf("Malformed JSON update %q in %s",
				update.Value.Value, update)
		}
		return value, nil
	case Type_BYTES:
		return strconv.Quote(string(update.Value.Value)), nil
	default:
		return nil,
			fmt.Errorf("Unhandled type of value %v in %s", update.Value.Type, update)
	}
}

// SubscribeResponseToJSON converts a SubscribeResponse into a JSON string
func SubscribeResponseToJSON(resp *SubscribeResponse) (string, error) {
	m := make(map[string]interface{}, 1)
	var err error
	switch resp := resp.Response.(type) {
	case *SubscribeResponse_Update:
		notif := resp.Update
		m["timestamp"] = notif.Timestamp
		m["path"] = "/" + joinPath(notif.Prefix)
		if len(notif.Update) != 0 {
			updates := make(map[string]interface{}, len(notif.Update))
			for _, update := range notif.Update {
				updates[joinPath(update.Path)], err = convertUpdate(update)
				if err != nil {
					return "", err
				}
			}
			m["updates"] = updates
		}
		if len(notif.Delete) != 0 {
			deletes := make([]string, len(notif.Delete))
			for i, del := range notif.Delete {
				deletes[i] = joinPath(del)
			}
			m["deletes"] = deletes
		}
		m = map[string]interface{}{"notification": m}
	case *SubscribeResponse_Heartbeat:
		m["heartbeat"] = resp.Heartbeat.Interval
	case *SubscribeResponse_SyncResponse:
		m["syncResponse"] = resp.SyncResponse
	default:
		return "", fmt.Errorf("Unknown type of response: %T: %s", resp, resp)
	}
	js, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return "", err
	}
	return string(js), nil
}
