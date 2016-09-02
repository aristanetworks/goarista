// Copyright (C) 2016  Arista Networks, Inc.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the COPYING file.

package openconfig

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
)

func joinPath(path *Path) string {
	return strings.Join(path.Element, "/")
}

func convertUpdate(update *Update) (interface{}, error) {
	switch update.Value.Type {
	case Type_JSON:
		var value interface{}
		decoder := json.NewDecoder(bytes.NewReader(update.Value.Value))
		decoder.UseNumber()
		if err := decoder.Decode(&value); err != nil {
			return nil, fmt.Errorf("Malformed JSON update %q in %s",
				update.Value.Value, update)
		}
		return value, nil
	case Type_BYTES:
		return update.Value.Value, nil
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

// EscapeFunc is the escaping method for attribute names
type EscapeFunc func(k string) string

// escapeValue looks for maps in an interface and escapes their keys
func escapeValue(value interface{}, escape EscapeFunc) interface{} {
	valueMap, ok := value.(map[string]interface{})
	if !ok {
		return value
	}
	escapedMap := make(map[string]interface{}, len(valueMap))
	for k, v := range valueMap {
		escapedKey := escape(k)
		escapedMap[escapedKey] = escapeValue(v, escape)
	}
	return escapedMap
}

// addPathToMap creates a map[string]interface{} from a path. It returns the node in
// the map corresponding to the last element in the path
func addPathToMap(parent map[string]interface{}, path []string, escape EscapeFunc) (
	map[string]interface{}, error) {
	for _, element := range path[:len(path)-1] {
		escapedElement := escape(element)
		node, found := parent[escapedElement]
		if !found {
			node = map[string]interface{}{}
			parent[escapedElement] = node
		}
		var ok bool
		parent, ok = node.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf(
				"Node %s is of type %T (expected map[string]interface traversing %q)",
				element, node, path)
		}
	}
	return parent, nil
}

// NotificationToMap maps a Notification into a nested map of entities
func NotificationToMap(notification *Notification,
	escape EscapeFunc) (map[string]interface{}, error) {
	if escape == nil {
		escape = func(name string) string {
			return name
		}
	}
	prefix := notification.GetPrefix()
	root := map[string]interface{}{
		"_timestamp": notification.Timestamp,
	}
	prefixLeaf := root
	if prefix != nil {
		parent := root
		for _, element := range prefix.Element {
			node := map[string]interface{}{}
			parent[escape(element)] = node
			parent = node
		}
		prefixLeaf = parent
	}
	for _, update := range notification.GetUpdate() {
		parent := prefixLeaf
		path := update.GetPath()
		elementLen := len(path.Element)
		if elementLen > 1 {
			parentElements := path.Element[:elementLen-1]
			var err error
			parent, err = addPathToMap(prefixLeaf, parentElements, escape)
			if err != nil {
				return nil, err
			}
		}
		value := update.GetValue()
		var unmarshaledValue interface{}
		switch value.Type {
		case Type_JSON:
			if err := json.Unmarshal(value.Value, &unmarshaledValue); err != nil {
				return nil, err
			}
		case Type_BYTES:
			unmarshaledValue = update.Value.Value
		default:
			return nil, fmt.Errorf("Unexpected value type %s for path %v",
				value.Type, path)
		}
		parent[escape(path.Element[elementLen-1])] = escapeValue(unmarshaledValue,
			escape)
	}
	return root, nil
}

// NotificationToJSONDocument maps a Notification into a single JSON document
func NotificationToJSONDocument(notification *Notification,
	escape EscapeFunc) ([]byte, error) {
	m, err := NotificationToMap(notification, escape)
	if err != nil {
		return nil, err
	}
	return json.Marshal(m)
}
