// Copyright (C) 2016  Arista Networks, Inc.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the COPYING file.

// The occlient tool is a client for the gRPC service for getting and setting the
// OpenConfig configuration and state of a network device.
package main

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"strconv"
	"strings"
	"sync"

	"golang.org/x/net/context"

	"github.com/Shopify/sarama"
	"github.com/aristanetworks/goarista/kafka"
	"github.com/aristanetworks/goarista/kafka/openconfig"
	"github.com/aristanetworks/goarista/kafka/producer"

	"github.com/aristanetworks/glog"
	pb "github.com/aristanetworks/goarista/openconfig"
	"github.com/golang/protobuf/proto"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"
)

const (
	defaultPort   = "6042"
	addrsFlagName = "addrs"
)

var addrsFlag = flag.String(addrsFlagName, "localhost:"+defaultPort,
	"Addresses of the OpenConfig servers (comma-separated)")

var certFileFlag = flag.String("certfile", "",
	"Path to client TLS certificate file")

var keyFileFlag = flag.String("keyfile", "",
	"Path to client TLS private key file")

var caFileFlag = flag.String("cafile", "",
	"Path to server TLS certificate file")

var jsonOutputFlag = flag.Bool("json", false,
	"Print the output in JSON instead of protobuf")

var subscribeFlag = flag.String("subscribe", "",
	"Comma-separated list of paths to subscribe to upon connecting to the server")

var usernameFlag = flag.String("username", "",
	"Username to authenticate with")

var passwordFlag = flag.String("password", "",
	"Password to authenticate with")

var kafkaKeysFlag = flag.String("kafkakey", "",
	"Keys for kafka messages (comma-separated, default: the value of -"+
		addrsFlagName)

type client struct {
	addr          string
	client        pb.OpenConfigClient
	ctx           context.Context
	kafkaProducer producer.Producer
}

func newClient(addr string, opts *[]grpc.DialOption, username, password string,
	subscribePaths []string, kafkaAddresses []string, kafkaTopic,
	kafkaKey string) (*client, error) {
	c := &client{
		addr: addr,
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
	if len(kafkaAddresses) == 0 {
		return c, nil
	}
	kafkaConfig := sarama.NewConfig()
	hostname, err := os.Hostname()
	if err != nil {
		hostname = ""
	}
	kafkaConfig.ClientID = hostname
	kafkaConfig.Producer.Compression = sarama.CompressionSnappy
	kafkaConfig.Producer.Return.Successes = true

	kafkaClient, err := sarama.NewClient(kafkaAddresses, kafkaConfig)
	if err != nil {
		glog.Fatalf("Failed to create Kafka client: %s", err)
	}
	kafkaChan := make(chan proto.Message)
	key := sarama.StringEncoder(kafkaKey)
	c.kafkaProducer, err = producer.New(kafkaTopic, kafkaChan, kafkaClient, key,
		openconfig.MessageEncoder)
	if err != nil {
		return nil, fmt.Errorf("Failed to create Kafka producer: %s", err)
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
						&pb.Subscription{
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
		glog.Info(respTxt)
		if c.kafkaProducer != nil {
			c.kafkaProducer.Write(resp)
		}
	}
}

func main() {
	flag.Parse()
	var opts []grpc.DialOption
	if *caFileFlag != "" || *certFileFlag != "" {
		config := &tls.Config{}
		if *caFileFlag != "" {
			b, err := ioutil.ReadFile(*caFileFlag)
			if err != nil {
				glog.Fatal(err)
			}
			cp := x509.NewCertPool()
			if !cp.AppendCertsFromPEM(b) {
				glog.Fatalf("credentials: failed to append certificates")
			}
			config.RootCAs = cp
		} else {
			config.InsecureSkipVerify = true
		}
		if *certFileFlag != "" {
			if *keyFileFlag == "" {
				glog.Fatalf("Please provide both -certfile and -keyfile")
			}
			cert, err := tls.LoadX509KeyPair(*certFileFlag, *keyFileFlag)
			if err != nil {
				glog.Fatal(err)
			}
			config.Certificates = []tls.Certificate{cert}
		}
		opts = append(opts, grpc.WithTransportCredentials(credentials.NewTLS(config)))
	} else {
		opts = append(opts, grpc.WithInsecure())
	}
	if *kafkaKeysFlag == "" {
		*kafkaKeysFlag = *addrsFlag
	}
	addrs := strings.Split(*addrsFlag, ",")
	kafkaKeys := strings.Split(*kafkaKeysFlag, ",")
	if len(addrs) != len(kafkaKeys) {
		glog.Fatal("Please provide the same number of addresses and Kafka keys")
	}
	var subscribePaths []string
	if *subscribeFlag != "" {
		subscribePaths = strings.Split(*subscribeFlag, ",")
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
		c, err := newClient(addr, &opts, *usernameFlag, *passwordFlag, subscribePaths,
			kafkaAddresses, *kafka.Topic, kafkaKey)
		if err != nil {
			glog.Fatal(err)
		}
		wg.Add(1)
		go c.run(wg, subscribePaths)
	}
	wg.Wait()
}

func joinPath(path *pb.Path) string {
	return strings.Join(path.Element, "/")
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
				var value interface{}
				switch update.Value.Type {
				case pb.Type_JSON:
					err := json.Unmarshal(update.Value.Value, &value)
					if err != nil {
						glog.Fatal(err)
					}
				case pb.Type_BYTES:
					value = strconv.Quote(string(update.Value.Value))
				default:
					glog.Fatalf("Unhandled type of value %v", update.Value.Type)
				}
				updates[joinPath(update.Path)] = value
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
