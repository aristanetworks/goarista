// Copyright (c) 2016 Arista Networks, Inc.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the COPYING file.

package openconfig

import (
	"encoding/json"
	"testing"

	"github.com/aristanetworks/goarista/test"

	"github.com/openconfig/gnmi/proto/gnmi"
)

func TestNotificationToMap(t *testing.T) {
	value := map[string]interface{}{
		"239.255.255.250_0.0.0.0": map[string]interface{}{
			"creationTime": 4.567969230573434e+06,
		},
	}
	valueJSON, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		notification gnmi.Notification
		json         map[string]interface{}
	}{{
		notification: gnmi.Notification{
			Prefix: &gnmi.Path{
				Element: []string{
					"foo",
				},
			},
			Update: []*gnmi.Update{
				{
					Path: &gnmi.Path{
						Element: []string{
							"route1",
						},
					},
					Value: &gnmi.Value{
						Value: valueJSON,
					},
				}, {
					Path: &gnmi.Path{
						Element: []string{
							"route2",
						},
					},
					Value: &gnmi.Value{
						Value: valueJSON,
					},
				}},
		},
		json: map[string]interface{}{
			"timestamp": int64(0),
			"dataset":   "cairo",
			"update": map[string]interface{}{
				"foo": map[string]interface{}{
					"route1": map[string]interface{}{
						"239.255.255.250_0.0.0.0": map[string]interface{}{
							"creationTime": 4.567969230573434e+06,
						},
					},
					"route2": map[string]interface{}{
						"239.255.255.250_0.0.0.0": map[string]interface{}{
							"creationTime": 4.567969230573434e+06,
						},
					},
				},
			},
		},
	}, {
		notification: gnmi.Notification{
			Prefix: &gnmi.Path{
				Element: []string{
					"foo", "bar",
				},
			},
			Delete: []*gnmi.Path{
				{
					Element: []string{
						"route", "237.255.255.250_0.0.0.0",
					}},
				{
					Element: []string{
						"route", "238.255.255.250_0.0.0.0",
					},
				},
			},
			Update: []*gnmi.Update{{
				Path: &gnmi.Path{
					Element: []string{
						"route",
					},
				},
				Value: &gnmi.Value{
					Value: valueJSON,
				},
			}},
		},
		json: map[string]interface{}{
			"timestamp": int64(0),
			"dataset":   "cairo",
			"delete": map[string]interface{}{
				"foo": map[string]interface{}{
					"bar": map[string]interface{}{
						"route": map[string]interface{}{
							"237.255.255.250_0.0.0.0": map[string]interface{}{},
							"238.255.255.250_0.0.0.0": map[string]interface{}{},
						},
					},
				},
			},
			"update": map[string]interface{}{
				"foo": map[string]interface{}{
					"bar": map[string]interface{}{
						"route": map[string]interface{}{
							"239.255.255.250_0.0.0.0": map[string]interface{}{
								"creationTime": 4.567969230573434e+06,
							},
						},
					},
				},
			},
		},
	}}
	for i := 0; i < len(tests); i++ {
		tcase := &tests[i] // index slice to avoid copying struct with mutex in it
		actual, err := NotificationToMap("cairo", &tcase.notification, nil)
		if err != nil {
			t.Fatal(err)
		}
		diff := test.Diff(tcase.json, actual)
		if len(diff) > 0 {
			expectedJSON, _ := json.Marshal(tcase.json)
			actualJSON, _ := json.Marshal(actual)
			t.Fatalf("Unexpected diff: %s\nExpected:\n%s\nGot:\n%s\n)", diff, expectedJSON,
				actualJSON)
		}
	}
}
