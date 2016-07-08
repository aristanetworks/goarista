// Copyright (C) 2016  Arista Networks, Inc.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the COPYING file.

package openconfig

import (
	"encoding/json"
	"testing"

	"github.com/aristanetworks/goarista/test"
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
	notification := Notification{
		Prefix: &Path{
			Element: []string{
				"Smash", "routing", "pim", "sparsemode", "status", "default",
			},
		},
		Update: []*Update{{
			Path: &Path{
				Element: []string{
					"route",
				},
			},
			Value: &Value{
				Value: valueJSON,
			},
		}},
	}
	expected := map[string]interface{}{
		"Smash": map[string]interface{}{
			"routing": map[string]interface{}{
				"pim": map[string]interface{}{
					"sparsemode": map[string]interface{}{
						"status": map[string]interface{}{
							"default": map[string]interface{}{
								"route": map[string]interface{}{
									"239.255.255.250_0.0.0.0": map[string]interface{}{
										"creationTime": 4.567969230573434e+06,
									},
								},
							},
						},
					},
				},
			},
		},
	}
	actual, err := NotificationToMap(&notification, nil)
	if err != nil {
		t.Fatal(err)
	}
	delete(actual, "_timestamp")
	diff := test.Diff(expected, actual)
	if len(diff) > 0 {
		expectedJSON, _ := json.Marshal(expected)
		actualJSON, _ := json.Marshal(actual)
		t.Fatalf("Unexpected diff: %s (expected: %s, got: %s)", diff, expectedJSON,
			actualJSON)
	}
}
