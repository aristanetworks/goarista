// Copyright (c) 2016 Arista Networks, Inc.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the COPYING file.

package gnmi

import (
	"testing"

	"github.com/aristanetworks/goarista/elasticsearch"
	"github.com/aristanetworks/goarista/test"

	"github.com/openconfig/gnmi/proto/gnmi"
)

func ToStringPtr(str string) *string {
	return &str
}

func ToFloatPtr(flt float64) *float64 {
	return &flt
}

func TestJsonify(t *testing.T) {
	var tests = []struct {
		notification *gnmi.Notification
		document     map[string]interface{}
	}{{
		notification: &gnmi.Notification{
			Prefix: &gnmi.Path{Elem: []*gnmi.PathElem{
				&gnmi.PathElem{Name: "Sysdb"}, &gnmi.PathElem{Name: "a"}}},
			Update: []*gnmi.Update{
				{
					Path: &gnmi.Path{Elem: []*gnmi.PathElem{&gnmi.PathElem{Name: "b"}}},
					Val: &gnmi.TypedValue{
						Value: &gnmi.TypedValue_JsonVal{
							JsonVal: []byte{52, 50},
						},
					},
				},
			},
		},
		document: map[string]interface{}{
			"Timestamp":   uint64(0),
			"DatasetID":   "foo",
			"Path":        "/Sysdb/a/b",
			"Key":         []byte("/b"),
			"KeyString":   ToStringPtr("/b"),
			"ValueDouble": ToFloatPtr(float64(42))},
	},
	}
	for _, jsonTest := range tests {
		actual, err := elasticsearch.NotificationToMaps("foo", jsonTest.notification)
		if err != nil {
			t.Error(err)
		}
		diff := test.Diff(jsonTest.document, actual[0])
		if len(diff) > 0 {
			t.Errorf("Unexpected diff: %s", diff)
		}
	}
}
