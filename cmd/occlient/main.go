// Copyright (C) 2016  Arista Networks, Inc.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the COPYING file.

// The occlient tool is a client for the gRPC service for getting and setting the
// OpenConfig configuration and state of a network device.
package main

import (
	"encoding/json"
	"flag"
	"io"
	"strconv"
	"strings"
	"sync"

	"golang.org/x/net/context"

	"github.com/aristanetworks/goarista/kafka"
	"github.com/aristanetworks/goarista/kafka/producer"
	cli "github.com/aristanetworks/goarista/openconfig/client"

	"github.com/aristanetworks/glog"
	pb "github.com/aristanetworks/goarista/openconfig"
	"github.com/golang/protobuf/proto"

	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

const defaultPort = "6042"

var jsonOutputFlag = flag.Bool("json", false,
	"Print the output in JSON instead of protobuf")

var kafkaKeysFlag = flag.String("kafkakey", "",
	"Keys for kafka messages (comma-separated, default: the value of -addrs")

type client struct {
	addr          string
	client        pb.OpenConfigClient
	ctx           context.Context
	kafkaProducer producer.Producer
}

// newClient creates a new gRPC client and pipes it into the given producer.
// The producer is typically something responsible for pushing updates to a
// backend system like Kafka or Redis or HBase.
func newClient(addr string, opts *[]grpc.DialOption, username, password string,
	p producer.Producer) (*client, error) {
	c := &client{
		addr:          addr,
		kafkaProducer: p,
	}
	conn, err := grpc.Dial(addr, *opts...)
	if err != nil {
		return nil, err
	}
	glog.Infof("Connected to %s", addr)
	c.client = pb.NewOpenConfigClient(conn)
	c.ctx = context.Background()
	if username != "" {
		c.ctx = metadata.NewContext(c.ctx, metadata.Pairs(
			"username", username,
			"password", password))
	}
	return c, nil
}

func (c *client) run(wg sync.WaitGroup, subscribePaths []string) error {
	defer wg.Done()
	stream, err := c.client.Subscribe(c.ctx)
	if err != nil {
		glog.Fatalf("Subscribe failed: %s", err)
	}
	for _, path := range subscribePaths {
		sub := &pb.SubscribeRequest{
			Request: &pb.SubscribeRequest_Subscribe{
				Subscribe: &pb.SubscriptionList{
					Subscription: []*pb.Subscription{
						{
							Path: &pb.Path{Element: strings.Split(path, "/")},
						},
					},
				},
			},
		}

		err = stream.Send(sub)
		if err != nil {
			return err
		}
	}
	if c.kafkaProducer != nil {
		go c.kafkaProducer.Run()
	}
	for {
		resp, err := stream.Recv()
		if err != nil {
			if err != io.EOF {
				glog.Fatalf("Error received from the %s: %s", c.addr, err)
			}
		}
		var respTxt string
		if *jsonOutputFlag {
			respTxt = jsonify(resp)
		} else {
			respTxt = proto.MarshalTextString(resp)
		}
		log := glog.Info
		if resp.GetHeartbeat() != nil {
			log = glog.V(1).Info // Log heartbeats with verbose logging only.
		}
		if c.kafkaProducer != nil {
			c.kafkaProducer.Write(resp)
		} else {
			log(respTxt)
		}
	}
}

func main() {
	username, password, subscriptions, addrs, opts := cli.ParseFlags()

	if *kafkaKeysFlag == "" {
		*kafkaKeysFlag = strings.Join(addrs, ",")
	}
	kafkaKeys := strings.Split(*kafkaKeysFlag, ",")
	if len(addrs) != len(kafkaKeys) {
		glog.Fatal("Please provide the same number of addresses and Kafka keys")
	}
	var kafkaAddresses []string
	if *kafka.Addresses != "" {
		kafkaAddresses = strings.Split(*kafka.Addresses, ",")
	}
	var wg sync.WaitGroup
	for i := 0; i < len(addrs); i++ {
		addr := addrs[i]
		kafkaKey := kafkaKeys[i]
		if !strings.ContainsRune(addr, ':') {
			addr += ":" + defaultPort
		}
		p, err := newKafkaProducer(kafkaAddresses, *kafka.Topic, kafkaKey)
		if err != nil {
			glog.Fatal("Failed to initialize producer: ", err)
		}
		c, err := newClient(addr, &opts, username, password, p)
		if err != nil {
			glog.Fatal("Failed to initialize client: ", err)
		}
		wg.Add(1)
		go c.run(wg, subscriptions)
	}
	wg.Wait()
}

func joinPath(path *pb.Path) string {
	return strings.Join(path.Element, "/")
}

func convertUpdate(update *pb.Update) interface{} {
	switch update.Value.Type {
	case pb.Type_JSON:
		var value interface{}
		err := json.Unmarshal(update.Value.Value, &value)
		if err != nil {
			glog.Fatalf("Malformed JSON update %q in %s", update.Value.Value, update)
		}
		return value
	case pb.Type_BYTES:
		return strconv.Quote(string(update.Value.Value))
	default:
		glog.Fatalf("Unhandled type of value %v in %s", update.Value.Type, update)
		return nil
	}
}

func jsonify(resp *pb.SubscribeResponse) string {
	m := make(map[string]interface{}, 1)
	switch resp := resp.Response.(type) {
	case *pb.SubscribeResponse_Update:
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
	case *pb.SubscribeResponse_Heartbeat:
		m["heartbeat"] = resp.Heartbeat.Interval
	case *pb.SubscribeResponse_SyncResponse:
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
