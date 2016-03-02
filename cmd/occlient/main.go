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
	"io"
	"io/ioutil"
	"strconv"
	"strings"

	"golang.org/x/net/context"

	"github.com/aristanetworks/glog"
	"github.com/aristanetworks/goarista/openconfig"
	"github.com/golang/protobuf/proto"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"
)

var addr = flag.String("addr", "localhost:6042",
	"Address of the server")

var certFile = flag.String("certfile", "",
	"Path to client TLS certificate file")

var keyFile = flag.String("keyfile", "",
	"Path to client TLS private key file")

var caFile = flag.String("cafile", "",
	"Path to server TLS certificate file")

var jsonOutput = flag.Bool("json", false,
	"Print the output in JSON instead of protobuf")

var subscribe = flag.String("subscribe", "",
	"Comma-separated list of paths to subscribe to upon connecting to the server")

var username = flag.String("username", "",
	"Username to authenticate with")

var password = flag.String("password", "",
	"Password to authenticate with")

func main() {
	flag.Parse()
	var opts []grpc.DialOption
	if *caFile != "" || *certFile != "" {
		config := &tls.Config{}
		if *caFile != "" {
			b, err := ioutil.ReadFile(*caFile)
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
		if *certFile != "" {
			if *keyFile == "" {
				glog.Fatalf("Please provide both -certfile and -keyfile")
			}
			cert, err := tls.LoadX509KeyPair(*certFile, *keyFile)
			if err != nil {
				glog.Fatal(err)
			}
			config.Certificates = []tls.Certificate{cert}
		}
		opts = append(opts, grpc.WithTransportCredentials(credentials.NewTLS(config)))
	} else {
		opts = append(opts, grpc.WithInsecure())
	}
	conn, err := grpc.Dial(*addr, opts...)
	if err != nil {
		glog.Fatalf("fail to dial: %s", err)
	}
	defer conn.Close()
	client := openconfig.NewOpenConfigClient(conn)

	ctx := context.Background()
	if *username != "" {
		ctx = metadata.NewContext(ctx, metadata.Pairs(
			"username", *username,
			"password", *password))
	}

	stream, err := client.Subscribe(ctx)
	if err != nil {
		glog.Fatalf("Subscribe failed: %s", err)
	}
	defer stream.CloseSend()

	for _, path := range strings.Split(*subscribe, ",") {
		sub := &openconfig.SubscribeRequest{
			Request: &openconfig.SubscribeRequest_Subscribe{
				Subscribe: &openconfig.SubscriptionList{
					Subscription: []*openconfig.Subscription{
						&openconfig.Subscription{
							Path: &openconfig.Path{Element: strings.Split(path, "/")},
						},
					},
				},
			},
		}

		err = stream.Send(sub)
		if err != nil {
			glog.Fatalf("Failed to subscribe: %s", err)
		}
	}

	for {
		resp, err := stream.Recv()
		if err != nil {
			if err != io.EOF {
				glog.Fatalf("Error received from the server: %s", err)
			}
			return
		}
		var respTxt string
		if *jsonOutput {
			respTxt = jsonify(resp)
		} else {
			respTxt = proto.MarshalTextString(resp)
		}
		glog.Info(respTxt)
	}
}

func joinPath(path *openconfig.Path) string {
	return strings.Join(path.Element, "/")
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
				var value interface{}
				switch update.Value.Type {
				case openconfig.Type_JSON:
					err := json.Unmarshal(update.Value.Value, &value)
					if err != nil {
						glog.Fatal(err)
					}
				case openconfig.Type_BYTES:
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
