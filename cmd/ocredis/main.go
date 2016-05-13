// Copyright (C) 2016  Arista Networks, Inc.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the COPYING file.

// The ocredis tool is a client for the OpenConfig gRPC interface that
// subscribes to state and pushes it to Redis, using Redis' support for hash
// maps and for publishing events that can be subscribed to.
package main

import (
	"encoding/json"
	"flag"
	"strconv"
	"strings"
	"sync"

	"github.com/aristanetworks/glog"
	"github.com/aristanetworks/goarista/openconfig"
	"github.com/aristanetworks/goarista/openconfig/client"
	"github.com/garyburd/redigo/redis"
)

var redisFlag = flag.String("redis", "",
	"Address of Redis server where to push updates to")

func main() {
	username, password, subscriptions, addrs, opts := client.ParseFlags()
	if *redisFlag == "" {
		glog.Fatal("Specify the address of the Redis server to write to with -redis")
	}

	r, err := redis.Dial("tcp", *redisFlag)
	if err != nil {
		glog.Fatal("Failed to connect to Redis: ", err)
	}

	publish := func(resp *openconfig.SubscribeResponse) {
		if notif := resp.GetUpdate(); notif != nil {
			publishToRedis(r, notif)
		}
	}

	wg := new(sync.WaitGroup)
	for _, addr := range addrs {
		wg.Add(1)
		go client.Run(publish, wg, username, password, addr, subscriptions, opts)
	}
	wg.Wait()
}

func publishToRedis(r redis.Conn, notif *openconfig.Notification) {
	path := "/" + joinPath(notif.Prefix)

	publish := func(kind string, payload interface{}) {
		js, err := json.Marshal(map[string]interface{}{
			"kind":    kind,
			"payload": payload,
		})
		if err != nil {
			glog.Fatalf("JSON error: %s", err)
		}
		err = r.Send("PUBLISH", path, js)
		if err != nil {
			glog.Fatalf("redis PUBLISH error: %s", err)
		}
	}

	var err error
	if len(notif.Update) != 0 {
		// kvs is going to be: ["/path/to/entity", "key1", "value1", "key2", "value2", ...]
		kvs := make([]interface{}, 1+len(notif.Update)*2)
		// Updates to publish on the pub/sub.
		pub := make(map[string]interface{}, len(notif.Update))
		kvs[0] = path
		for i, update := range notif.Update {
			key := joinPath(update.Path)
			value := convertUpdate(update)
			pub[key] = value
			// The initial "1+" is to skip over the path.
			kvs[1+i*2] = key
			kvs[1+i*2+1], err = json.Marshal(value)
			if err != nil {
				glog.Fatalf("Failed to JSON marshal update %#v", update)
			}
		}
		err = r.Send("HMSET", kvs...)
		if err != nil {
			glog.Fatalf("redis HMSET error: %s", err)
		}
		publish("updates", pub)
	}

	if len(notif.Delete) != 0 {
		// keys is going to be: ["/path/to/entity", "key1", "key2", ...]
		keys := make([]interface{}, 1+len(notif.Delete))
		keys[0] = path
		for i, del := range notif.Delete {
			keys[i] = joinPath(del)
		}
		err = r.Send("HDEL", keys...)
		if err != nil {
			glog.Fatalf("redis HDEL error: %s", err)
		}
		publish("deletes", keys)
	}
	_, err = r.Do("") // Flush
	if err != nil {
		glog.Fatalf("redis error: %s", err)
	}
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
