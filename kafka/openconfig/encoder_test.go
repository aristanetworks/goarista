// Copyright (C) 2016  Arista Networks, Inc.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the COPYING file.

package openconfig

import (
	"encoding/json"
	"testing"

	"github.com/aristanetworks/goarista/openconfig"
	"github.com/aristanetworks/goarista/test"
)

func TestJsonify(t *testing.T) {
	var tests = []struct {
		notification *openconfig.Notification
		document     map[string]interface{}
	}{{
		notification: &openconfig.Notification{
			Prefix: &openconfig.Path{Element: []string{"Sysdb", "a"}},
			Update: []*openconfig.Update{
				&openconfig.Update{
					Path: &openconfig.Path{Element: []string{"b"}},
					Value: &openconfig.Value{
						Value: []byte{52, 50},
						Type:  openconfig.Type_JSON,
					},
				},
			},
		},
		document: map[string]interface{}{
			"Sysdb": map[string]interface{}{
				"a": map[string]interface{}{
					"b": 42,
				},
			},
		},
	},
	}
	for _, jsonTest := range tests {
		expected, err := json.Marshal(jsonTest.document)
		if err != nil {
			t.Fatal(err)
		}
		actual, err := jsonify(jsonTest.notification)
		if err != nil {
			t.Error(err)
		}
		diff := test.Diff(actual, expected)
		if len(diff) > 0 {
			t.Errorf("Unexpected diff: %s", diff)
		}
	}
}
