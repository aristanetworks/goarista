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
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/PuerkitoBio/redisc"
	"github.com/aristanetworks/glog"
	"github.com/aristanetworks/goarista/openconfig"
	"github.com/aristanetworks/goarista/openconfig/client"
	"github.com/garyburd/redigo/redis"
)

var clusterMode = flag.Bool("cluster", false, "Whether the redis server is a cluster")

var redisFlag = flag.String("redis", "",
	"Comma separated list of Redis servers to push updates to")

var redisPassword = flag.String("redispass", "", "Password of redis server/cluster")

// dialRedis connects to redis and tries to authenticate if necessary.
func dialRedis(server string) (redis.Conn, error) {
	c, err := redis.Dial("tcp", server)
	if err != nil {
		return nil, fmt.Errorf("Failed to connect to Redis server %s: %s", server, err)
	}
	if *redisPassword != "" {
		if _, err := c.Do("AUTH", *redisPassword); err != nil {
			c.Close()
			glog.Fatal("Error authenticating: ", err)
		}
	}
	return c, nil
}

// newPool creates a new redis Pool. This is based on the example in the redigo library
func newPool(server string, options ...redis.DialOption) (*redis.Pool, error) {
	return &redis.Pool{
		MaxIdle:     3,
		IdleTimeout: 300 * time.Second,
		Dial: func() (redis.Conn, error) {
			return dialRedis(server)
		},
		TestOnBorrow: func(c redis.Conn, t time.Time) error {
			_, err := c.Do("PING")
			return err
		},
	}, nil
}

func main() {
	username, password, subscriptions, addrs, opts := client.ParseFlags()
	if *redisFlag == "" {
		glog.Fatal("Specify the address of the Redis server to write to with -redis")
	}

	// The addr is used to uniquely identify different devices
	publish := func(addr string, conn redis.Conn) func(*openconfig.SubscribeResponse) {
		return func(resp *openconfig.SubscribeResponse) {
			if notif := resp.GetUpdate(); notif != nil {
				publishToRedis(addr, conn, notif)
			}
		}
	}

	// If we are in clusterMode, we need to initialise the cluster
	var cluster *redisc.Cluster
	if *clusterMode {
		// Create a cluster, using the pool to authenticate
		cluster = &redisc.Cluster{
			StartupNodes: strings.Split(*redisFlag, ","),
			CreatePool:   newPool,
		}
		defer cluster.Close()

		// From the doc: refreshes the cluster's internal mapping of hash slots to nodes
		if err := cluster.Refresh(); err != nil {
			glog.Fatal("Failed to refresh cluster: ", err)
		}
	}

	wg := new(sync.WaitGroup)
	// Create a connection to Redis per device we are connected to. Otherwise, we will
	// have concurrency errors for using the same connection.
	for _, addr := range addrs {
		var conn redis.Conn
		var err error
		if *clusterMode {
			// If we are in cluster mode, we need to get our connections from the cluster
			r := cluster.Get()
			defer r.Close()
			// Set up the RetryConn
			conn, err = redisc.RetryConn(r, 3, 100*time.Millisecond)
			if err != nil {
				glog.Fatal("Failed to create RetryConn: ", err)
			}
		} else {
			// Otherwise, when we are not in cluster mode, so we can just directly Dial Redis
			conn, err = dialRedis(*redisFlag)
			if err != nil {
				glog.Fatalf("Failed to connect to redis at %s: %s", *redisFlag, err)
			}
		}

		wg.Add(1)
		go client.Run(publish(addr, conn), wg, username, password, addr, subscriptions, opts)
	}
	wg.Wait()
}

func publishToRedis(addr string, r redis.Conn, notif *openconfig.Notification) {
	path := addr + "/" + joinPath(notif.Prefix)

	publish := func(kind string, payload interface{}) {
		js, err := json.Marshal(map[string]interface{}{
			"kind":    kind,
			"payload": payload,
		})
		if err != nil {
			glog.Fatalf("JSON error: %s", err)
		}
		if _, err = r.Do("PUBLISH", path, js); err != nil {
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
		if _, err = r.Do("HMSET", kvs...); err != nil {
			if redirErr := redisc.ParseRedir(err); redirErr == nil {
				// ParseRedir returns nil if err is not a MOVED or ASK err, which
				// means this is some other error
				glog.Fatalf("redis HMSET error: %s", err)
			}
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
		if _, err = r.Do("HDEL", keys...); err != nil {
			if redirErr := redisc.ParseRedir(err); redirErr == nil {
				// ParseRedir returns nil if err is not a MOVED or ASK err, which
				// means this is some other error
				glog.Fatalf("redis HDEL error: %s", err)
			}
		}
		publish("deletes", keys)
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
