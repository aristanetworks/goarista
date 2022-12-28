// Copyright (c) 2017 Arista Networks, Inc.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the COPYING file.

package main

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"testing"

	"github.com/aristanetworks/goarista/gnmi"
	"github.com/aristanetworks/goarista/test"
	pb "github.com/openconfig/gnmi/proto/gnmi"
	"github.com/prometheus/client_golang/prometheus"
)

func makeMetrics(cfg *Config, expValues map[source]float64, notification *pb.Notification,
	prevMetrics map[source]*labelledMetric) map[source]*labelledMetric {

	expMetrics := map[source]*labelledMetric{}
	if prevMetrics != nil {
		expMetrics = prevMetrics
	}
	for src, v := range expValues {
		metric := cfg.getMetricValues(src)
		if metric == nil || metric.desc == nil || metric.labels == nil {
			panic("cfg.getMetricValues returned nil")
		}
		// Preserve current value of labels
		labels := metric.labels
		if _, ok := expMetrics[src]; ok && expMetrics[src] != nil {
			labels = expMetrics[src].labels
		}

		// Handle string updates
		if notification.Update != nil {
			if update, err := findUpdate(notification, src.path); err == nil {
				val, _, ok := parseValue(update)
				if !ok {
					continue
				}
				if metric.stringMetric {
					strVal, ok := val.(string)
					if !ok {
						strVal = fmt.Sprintf("%.0f", val)
					}
					v = metric.defaultValue
					labels[len(labels)-1] = strVal
				}
			}
		}
		expMetrics[src] = &labelledMetric{
			metric: prometheus.MustNewConstMetric(metric.desc, prometheus.GaugeValue, v,
				labels...),
			labels:       labels,
			defaultValue: metric.defaultValue,
			floatVal:     v,
			stringMetric: metric.stringMetric,
		}
	}
	// Handle deletion
	for key := range expMetrics {
		if _, ok := expValues[key]; !ok {
			delete(expMetrics, key)
		}
	}
	return expMetrics
}

func findUpdate(notif *pb.Notification, path string) (*pb.Update, error) {
	prefix := notif.Prefix
	for _, v := range notif.Update {
		var fullPath string
		if prefix != nil {
			fullPath = gnmi.StrPath(gnmi.JoinPaths(prefix, v.Path))
		} else {
			fullPath = gnmi.StrPath(v.Path)
		}
		if strings.Contains(path, fullPath) || path == fullPath {
			return v, nil
		}
	}
	return nil, fmt.Errorf("Failed to find matching update for path %v", path)
}

func makeResponse(notif *pb.Notification) *pb.SubscribeResponse {
	return &pb.SubscribeResponse{
		Response: &pb.SubscribeResponse_Update{Update: notif},
	}
}

func makePath(pathStr string) *pb.Path {
	splitPath := gnmi.SplitPath(pathStr)
	path, err := gnmi.ParseGNMIElements(splitPath)
	if err != nil {
		return &pb.Path{}
	}
	return path
}

func TestUpdate(t *testing.T) {
	config := []byte(`
devicelabels:
        10.1.1.1:
                lab1: val1
                lab2: val2
        '*':
                lab1: val3
                lab2: val4
subscriptions:
        - /Sysdb/environment/cooling/status
        - /Sysdb/environment/power/status
        - /Sysdb/bridging/igmpsnooping/forwarding/forwarding/status
        - /Sysdb/l2discovery/lldp/status
metrics:
        - name: fanName
          path: /Sysdb/environment/cooling/status/fan/name
          help: Fan Name
          valuelabel: name
          defaultvalue: 2.5
        - name: intfCounter
          path: /Sysdb/(lag|slice/phy/.+)/intfCounterDir/(?P<intf>.+)/intfCounter
          help: Per-Interface Bytes/Errors/Discards Counters
        - name: fanSpeed
          path: /Sysdb/environment/cooling/status/fan/speed/value
          help: Fan Speed
        - name: igmpSnoopingInf
          path: /Sysdb/igmpsnooping/vlanStatus/(?P<vlan>.+)/ethGroup/(?P<mac>.+)/intf/(?P<intf>.+)
          help: IGMP snooping status
        - name: lldpNeighborInfo
          path: /Sysdb/l2discovery/lldp/status/local/(?P<localIndex>.+)/` +
		`portStatus/(?P<intf>.+)/remoteSystem/(?P<remoteSystemIndex>.+)/sysName/value
          help: LLDP metric info
          valuelabel: neighborName
          defaultvalue: 1`)
	cfg, err := parseConfig(config)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	coll := newCollector(cfg, nil)

	notif := &pb.Notification{
		Prefix: makePath("Sysdb"),
		Update: []*pb.Update{
			{
				Path: makePath("lag/intfCounterDir/Ethernet1/intfCounter"),
				Val: &pb.TypedValue{
					Value: &pb.TypedValue_JsonVal{JsonVal: []byte("42")},
				},
			},
			{
				Path: makePath("environment/cooling/status/fan/speed"),
				Val: &pb.TypedValue{
					Value: &pb.TypedValue_JsonVal{JsonVal: []byte("{\"value\": 45}")},
				},
			},
			{
				Path: makePath("igmpsnooping/vlanStatus/2050/ethGroup/01:00:5e:01:01:01/intf/Cpu"),
				Val: &pb.TypedValue{
					Value: &pb.TypedValue_JsonVal{JsonVal: []byte("true")},
				},
			},
			{
				Path: makePath("environment/cooling/status/fan/name"),
				Val: &pb.TypedValue{
					Value: &pb.TypedValue_JsonVal{JsonVal: []byte("\"Fan1.1\"")},
				},
			},
			{
				Path: makePath("l2discovery/lldp/status/local/1/portStatus/" +
					"Ethernet24/remoteSystem/17/sysName/value"),
				Val: &pb.TypedValue{
					Value: &pb.TypedValue_JsonVal{JsonVal: []byte("{\"value\": \"testName\"}")},
				},
			},
		},
	}
	expValues := map[source]float64{
		{
			addr: "10.1.1.1",
			path: "/Sysdb/lag/intfCounterDir/Ethernet1/intfCounter",
		}: 42,
		{
			addr: "10.1.1.1",
			path: "/Sysdb/environment/cooling/status/fan/speed/value",
		}: 45,
		{
			addr: "10.1.1.1",
			path: "/Sysdb/igmpsnooping/vlanStatus/2050/ethGroup/01:00:5e:01:01:01/intf/Cpu",
		}: 1,
		{
			addr: "10.1.1.1",
			path: "/Sysdb/environment/cooling/status/fan/name",
		}: 2.5,
		{
			addr: "10.1.1.1",
			path: "/Sysdb/l2discovery/lldp/status/local/1/portStatus/Ethernet24/" +
				"remoteSystem/17/sysName/value/value",
		}: 1,
	}
	coll.update("10.1.1.1:6042", makeResponse(notif))
	expMetrics := makeMetrics(cfg, expValues, notif, nil)
	if !test.DeepEqual(expMetrics, coll.metrics) {
		t.Errorf("Mismatched metrics: %v", test.Diff(expMetrics, coll.metrics))
	}

	// Update two values, and one path which is not a metric
	notif = &pb.Notification{
		Prefix: makePath("Sysdb"),
		Update: []*pb.Update{
			{
				Path: makePath("lag/intfCounterDir/Ethernet1/intfCounter"),
				Val: &pb.TypedValue{
					Value: &pb.TypedValue_JsonVal{JsonVal: []byte("52")},
				},
			},
			{
				Path: makePath("environment/cooling/status/fan/name"),
				Val: &pb.TypedValue{
					Value: &pb.TypedValue_JsonVal{JsonVal: []byte("\"Fan2.1\"")},
				},
			},
			{
				Path: makePath("environment/doesntexist/status"),
				Val: &pb.TypedValue{
					Value: &pb.TypedValue_JsonVal{JsonVal: []byte("{\"value\": 45}")},
				},
			},
		},
	}
	src := source{
		addr: "10.1.1.1",
		path: "/Sysdb/lag/intfCounterDir/Ethernet1/intfCounter",
	}
	expValues[src] = 52

	coll.update("10.1.1.1:6042", makeResponse(notif))
	expMetrics = makeMetrics(cfg, expValues, notif, expMetrics)
	if !test.DeepEqual(expMetrics, coll.metrics) {
		t.Errorf("Mismatched metrics: %v", test.Diff(expMetrics, coll.metrics))
	}

	// Same path, different device
	notif = &pb.Notification{
		Prefix: makePath("Sysdb"),
		Update: []*pb.Update{
			{
				Path: makePath("lag/intfCounterDir/Ethernet1/intfCounter"),
				Val: &pb.TypedValue{
					Value: &pb.TypedValue_JsonVal{JsonVal: []byte("42")},
				},
			},
		},
	}
	src.addr = "10.1.1.2"
	expValues[src] = 42

	coll.update("10.1.1.2:6042", makeResponse(notif))
	expMetrics = makeMetrics(cfg, expValues, notif, expMetrics)
	if !test.DeepEqual(expMetrics, coll.metrics) {
		t.Errorf("Mismatched metrics: %v", test.Diff(expMetrics, coll.metrics))
	}

	// Delete a path
	notif = &pb.Notification{
		Prefix: makePath("Sysdb"),
		Delete: []*pb.Path{makePath("lag/intfCounterDir/Ethernet1/intfCounter")},
	}
	src.addr = "10.1.1.1"
	delete(expValues, src)

	coll.update("10.1.1.1:6042", makeResponse(notif))
	// Delete a path
	notif = &pb.Notification{
		Prefix: nil,
		Delete: []*pb.Path{makePath("Sysdb/environment/cooling/status/fan/name")},
	}
	src.path = "/Sysdb/environment/cooling/status/fan/name"
	delete(expValues, src)
	coll.update("10.1.1.1:6042", makeResponse(notif))
	expMetrics = makeMetrics(cfg, expValues, notif, expMetrics)
	if !test.DeepEqual(expMetrics, coll.metrics) {
		t.Errorf("Mismatched metrics: %v", test.Diff(expMetrics, coll.metrics))
	}

	// Non-numeric update to path without value label
	notif = &pb.Notification{
		Prefix: makePath("Sysdb"),
		Update: []*pb.Update{
			{
				Path: makePath("lag/intfCounterDir/Ethernet1/intfCounter"),
				Val: &pb.TypedValue{
					Value: &pb.TypedValue_JsonVal{JsonVal: []byte("\"test\"")},
				},
			},
		},
	}

	coll.update("10.1.1.1:6042", makeResponse(notif))
	src.addr = "10.1.1.1"
	src.path = "/Sysdb/lag/intfCounterDir/Ethernet1/intfCounter"
	expValues[src] = 0
	expMetrics = makeMetrics(cfg, expValues, notif, expMetrics)
	// Don't make new metrics as it should have no effect
	if !test.DeepEqual(expMetrics, coll.metrics) {
		t.Errorf("Mismatched metrics: %v", test.Diff(expMetrics, coll.metrics))
	}
	notif = &pb.Notification{
		Prefix: nil,
		Update: []*pb.Update{
			{
				Path: makePath("/Sysdb/lag/intfCounterDir/Ethernet1/intfCounter"),
				Val: &pb.TypedValue{
					Value: &pb.TypedValue_JsonVal{JsonVal: []byte("62")},
				},
			},
		},
	}
	src = source{
		addr: "10.1.1.1",
		path: "/Sysdb/lag/intfCounterDir/Ethernet1/intfCounter",
	}
	expValues[src] = 62
	coll.update("10.1.1.1:6042", makeResponse(notif))
	expMetrics = makeMetrics(cfg, expValues, notif, expMetrics)
	if !test.DeepEqual(expMetrics, coll.metrics) {
		t.Errorf("Mismatched metrics: %v", test.Diff(expMetrics, coll.metrics))
	}
}

func TestCoalescedDelete(t *testing.T) {
	config := []byte(`
devicelabels:
        10.1.1.1:
                lab1: val1
                lab2: val2
        '*':
                lab1: val3
                lab2: val4
subscriptions:
        - /Sysdb/environment/cooling/status
        - /Sysdb/environment/power/status
        - /Sysdb/bridging/igmpsnooping/forwarding/forwarding/status
metrics:
        - name: fanName
          path: /Sysdb/environment/cooling/status/fan/name
          help: Fan Name
          valuelabel: name
          defaultvalue: 2.5
        - name: intfCounter
          path: /Sysdb/(lag|slice/phy/.+)/intfCounterDir/(?P<intf>.+)/intfCounter
          help: Per-Interface Bytes/Errors/Discards Counters
        - name: fanSpeed
          path: /Sysdb/environment/cooling/status/fan/speed/value
          help: Fan Speed
        - name: igmpSnoopingInf
          path: /Sysdb/igmpsnooping/vlanStatus/(?P<vlan>.+)/ethGroup/(?P<mac>.+)/intf/(?P<intf>.+)
          help: IGMP snooping status`)
	cfg, err := parseConfig(config)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	coll := newCollector(cfg, nil)

	notif := &pb.Notification{
		Prefix: makePath("Sysdb"),
		Update: []*pb.Update{
			{
				Path: makePath("igmpsnooping/vlanStatus/2050/ethGroup/01:00:5e:01:01:01/intf/Cpu"),
				Val: &pb.TypedValue{
					Value: &pb.TypedValue_JsonVal{JsonVal: []byte("true")},
				},
			},
			{
				Path: makePath("igmpsnooping/vlanStatus/2050/ethGroup/01:00:5e:01:01:02/intf/Cpu"),
				Val: &pb.TypedValue{
					Value: &pb.TypedValue_JsonVal{JsonVal: []byte("true")},
				},
			},
			{
				Path: makePath("igmpsnooping/vlanStatus/2050/ethGroup/01:00:5e:01:01:03/intf/Cpu"),
				Val: &pb.TypedValue{
					Value: &pb.TypedValue_JsonVal{JsonVal: []byte("true")},
				},
			},
		},
	}
	expValues := map[source]float64{
		{
			addr: "10.1.1.1",
			path: "/Sysdb/igmpsnooping/vlanStatus/2050/ethGroup/01:00:5e:01:01:01/intf/Cpu",
		}: 1,
		{
			addr: "10.1.1.1",
			path: "/Sysdb/igmpsnooping/vlanStatus/2050/ethGroup/01:00:5e:01:01:02/intf/Cpu",
		}: 1,
		{
			addr: "10.1.1.1",
			path: "/Sysdb/igmpsnooping/vlanStatus/2050/ethGroup/01:00:5e:01:01:03/intf/Cpu",
		}: 1,
	}

	coll.update("10.1.1.1:6042", makeResponse(notif))
	expMetrics := makeMetrics(cfg, expValues, notif, nil)
	if !test.DeepEqual(expMetrics, coll.metrics) {
		t.Errorf("Mismatched metrics: %v", test.Diff(expMetrics, coll.metrics))
	}

	// Delete a subtree
	notif = &pb.Notification{
		Prefix: makePath("Sysdb"),
		Delete: []*pb.Path{makePath("igmpsnooping/vlanStatus/2050/ethGroup/01:00:5e:01:01:02")},
	}
	src := source{
		addr: "10.1.1.1",
		path: "/Sysdb/igmpsnooping/vlanStatus/2050/ethGroup/01:00:5e:01:01:02/intf/Cpu",
	}
	delete(expValues, src)

	coll.update("10.1.1.1:6042", makeResponse(notif))
	expMetrics = makeMetrics(cfg, expValues, notif, expMetrics)
	if !test.DeepEqual(expMetrics, coll.metrics) {
		t.Errorf("Mismatched metrics: %v", test.Diff(expMetrics, coll.metrics))
	}

}

func TestDescriptionTags(t *testing.T) {
	config := []byte(`
devicelabels:
        10.1.1.1:
                lab1: val1
                lab2: val2
        '*':
                lab1: val3
                lab2: val4
subscriptions:
        - /Sysdb/environment/cooling/status
        - /Sysdb/environment/power/status
        - /Sysdb/bridging/igmpsnooping/forwarding/forwarding/status
metrics:
        - name: fanName
          path: /Sysdb/environment/cooling/status/fan/name
          help: Fan Name
          valuelabel: name
          defaultvalue: 2.5
        - name: intfCounter
          path: /Sysdb/(lag|slice/phy/.+)/intfCounterDir/(?P<intf>.+)/intfCounter
          help: Per-Interface Bytes/Errors/Discards Counters
        - name: fanSpeed
          path: /Sysdb/environment/cooling/status/fan/speed/value
          help: Fan Speed
        - name: igmpSnoopingInf
          path: /Sysdb/igmpsnooping/vlanStatus/(?P<vlan>.+)/ethGroup/(?P<mac>.+)/intf/(?P<intf>.+)
          help: IGMP snooping status`)
	cfg, err := parseConfig(config)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	r := regexp.MustCompile(defaultDescriptionRegex)

	for _, tc := range []struct {
		name     string
		resp     *pb.SubscribeResponse
		expDescs map[string]map[string]string
	}{{
		name:     "no description leafs present",
		expDescs: make(map[string]map[string]string),
	}, {
		name: "add successful interface description node",
		resp: makeResponse(&pb.Notification{
			Update: []*pb.Update{{
				Path: makePath("interfaces/interface[name=Ethernet1]/state/description"),
				Val: &pb.TypedValue{
					Value: &pb.TypedValue_StringVal{StringVal: "[baz][bar=foo]"},
				},
			}},
		}),
		expDescs: map[string]map[string]string{
			"/interfaces/interface[name=Ethernet1]": {
				"baz": "1",
				"bar": "foo",
			},
		},
	}, {
		name: "leaf found with no data",
		resp: makeResponse(&pb.Notification{
			Update: []*pb.Update{{
				Path: makePath("interfaces/interface[name=Ethernet1]/state/description"),
				Val: &pb.TypedValue{
					Value: &pb.TypedValue_StringVal{StringVal: "hello"},
				},
			}},
		}),
		expDescs: make(map[string]map[string]string),
	}, {
		name: "leaf found with invalid regex value group found",
		resp: makeResponse(&pb.Notification{
			Update: []*pb.Update{{
				Path: makePath("interfaces/interface[name=Ethernet1]/state/description"),
				Val: &pb.TypedValue{
					Value: &pb.TypedValue_StringVal{StringVal: "[foo=bar=baz]"},
				},
			}},
		}),
		expDescs: make(map[string]map[string]string),
	}, {
		name: "leaf found with invalid regex key group found",
		resp: makeResponse(&pb.Notification{
			Update: []*pb.Update{{
				Path: makePath("interfaces/interface[name=Ethernet1]/state/description"),
				Val: &pb.TypedValue{
					Value: &pb.TypedValue_StringVal{StringVal: "[fo!oo]"},
				},
			}},
		}),
		expDescs: make(map[string]map[string]string),
	}, {
		name: "add successful peer group description node",
		resp: makeResponse(&pb.Notification{
			Update: []*pb.Update{{
				Path: makePath("/network-instances/network-instance[name=default]/protocols/prot" +
					"ocol[protocol=bgp]/bgp/peer-groups/peer-group[name=foo]/state/description"),
				Val: &pb.TypedValue{
					Value: &pb.TypedValue_StringVal{StringVal: "[baz][bar=foo]"},
				},
			}},
		}),
		expDescs: map[string]map[string]string{
			"/network-instances/network-instance[name=default]/protocols/protocol" +
				"[protocol=bgp]/bgp/peer-groups/peer-group[name=foo]": {
				"baz": "1",
				"bar": "foo",
			},
		},
	}, {
		name: "path with no list key",
		resp: makeResponse(&pb.Notification{
			Update: []*pb.Update{{
				Path: makePath("/system/config/description"),
				Val: &pb.TypedValue{
					Value: &pb.TypedValue_StringVal{StringVal: "[baz][bar=foo]"},
				},
			}},
		}),
		expDescs: make(map[string]map[string]string),
	}} {
		t.Run(tc.name, func(t *testing.T) {
			coll := newCollector(cfg, r)

			ctx := context.Background()
			respCh := make(chan *pb.SubscribeResponse)
			wg := &sync.WaitGroup{}
			wg.Add(1)

			go coll.handleDescriptionNodes(ctx, respCh, wg)
			if tc.resp != nil {
				respCh <- tc.resp
			}
			respCh <- &pb.SubscribeResponse{
				Response: &pb.SubscribeResponse_SyncResponse{SyncResponse: true},
			}
			wg.Wait()

			if !test.DeepEqual(tc.expDescs, coll.descriptionLabels) {
				t.Fatalf("unexpected description labels, expected %s, got %s",
					tc.expDescs, coll.descriptionLabels)
			}
		})
	}
}
