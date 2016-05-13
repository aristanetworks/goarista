// Copyright (C) 2016  Arista Networks, Inc.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the COPYING file.

// The occli tool is a simple client to dump in JSON or text format the
// protobufs returned by the OpenConfig gRPC interface.
package main

import (
	"encoding/json"
	"flag"
	"strconv"
	"strings"
	"sync"

	"github.com/aristanetworks/goarista/openconfig"
	"github.com/aristanetworks/goarista/openconfig/client"
	"github.com/golang/protobuf/proto"

	"github.com/aristanetworks/glog"
)

const defaultPort = "6042"

var jsonFlag = flag.Bool("json", false,
	"Print the output in JSON instead of protobuf")

func main() {
	username, password, subscriptions, addrs, opts := client.ParseFlags()

	publish := func(resp *openconfig.SubscribeResponse) {
		if resp.GetHeartbeat() != nil && !glog.V(1) {
			return // Log heartbeats with verbose logging only.
		}
		var respTxt string
		if *jsonFlag {
			respTxt = jsonify(resp)
		} else {
			respTxt = proto.MarshalTextString(resp)
		}
		glog.Info(respTxt)
	}

	wg := new(sync.WaitGroup)
	for _, addr := range addrs {
		wg.Add(1)
		go client.Run(publish, wg, username, password, addr, subscriptions, opts)
	}
	wg.Wait()
}

func joinPath(path *openconfig.Path) string {
	return strings.Join(path.Element, "/")
}

func convertUpdate(update *openconfig.Update) interface{} {
	switch update.Value.Type {
	case openconfig.Type_JSON:
		var value interface{}
		err := json.Unmarshal(update.Value.Value, &value)
		if err != nil {
			glog.Fatalf("Malformed JSON update %q in %s", update.Value.Value, update)
		}
		return value
	case openconfig.Type_BYTES:
		return strconv.Quote(string(update.Value.Value))
	default:
		glog.Fatalf("Unhandled type of value %v in %s", update.Value.Type, update)
		return nil
	}
}

func jsonify(resp *openconfig.SubscribeResponse) string {
	m := make(map[string]interface{}, 1)
	switch resp := resp.Response.(type) {
	case *openconfig.SubscribeResponse_Update:
		notif := resp.Update
		m["timestamp"] = notif.Timestamp
		m["path"] = "/" + joinPath(notif.Prefix)
		if len(notif.Update) != 0 {
			updates := make(map[string]interface{}, len(notif.Update))
			for _, update := range notif.Update {
				updates[joinPath(update.Path)] = convertUpdate(update)
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
	case *openconfig.SubscribeResponse_Heartbeat:
		m["heartbeat"] = resp.Heartbeat.Interval
	case *openconfig.SubscribeResponse_SyncResponse:
		m["syncResponse"] = resp.SyncResponse
	default:
		glog.Fatalf("Unknown type of response: %T: %s", resp, resp)
	}
	js, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		glog.Fatal("json: ", err)
	}
	return string(js)
}
