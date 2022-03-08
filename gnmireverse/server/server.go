// Copyright (c) 2020 Arista Networks, Inc.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the COPYING file.

package server

import (
	"crypto/tls"
	"crypto/x509"
	"flag"
	"fmt"
	"io/ioutil"
	"math"
	"net"
	"path"
	"time"

	"github.com/aristanetworks/glog"
	gnmilib "github.com/aristanetworks/goarista/gnmi"
	"github.com/aristanetworks/goarista/gnmireverse"
	"github.com/openconfig/gnmi/proto/gnmi"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	_ "google.golang.org/grpc/encoding/gzip" // Enable gzip encoding for the server.
	"google.golang.org/protobuf/proto"
)

func newTLSConfig(clientCertAuth bool, certFile, keyFile, clientCAFile string) (*tls.Config,
	error) {
	tlsConfig := tls.Config{}
	if clientCertAuth {
		tlsConfig.ClientAuth = tls.RequireAndVerifyClientCert
		if clientCAFile == "" {
			return nil, fmt.Errorf("client_cert_auth enable, client_cafile must also be set")
		}
		b, err := ioutil.ReadFile(clientCAFile)
		if err != nil {
			return nil, err
		}
		cp := x509.NewCertPool()
		if !cp.AppendCertsFromPEM(b) {
			return nil, fmt.Errorf("credentials: failed to append certificates")
		}
		tlsConfig.ClientCAs = cp
	}
	if certFile != "" {
		if keyFile == "" {
			return nil, fmt.Errorf("please provide both -certfile and -keyfile")
		}
		cert, err := tls.LoadX509KeyPair(certFile, keyFile)
		if err != nil {
			return nil, err
		}
		tlsConfig.Certificates = []tls.Certificate{cert}
	}
	return &tlsConfig, nil
}

// Main initializes the gNMIReverse server.
func Main() {
	addr := flag.String("addr", "127.0.0.1:6035", "address to listen on")
	useTLS := flag.Bool("tls", true, "set to false to disable TLS in collector")
	clientCertAuth := flag.Bool("client_cert_auth", false,
		"require and verify a client certificate. -client_cafile must also be set.")
	certFile := flag.String("certfile", "", "path to TLS certificate file")
	keyFile := flag.String("keyfile", "", "path to TLS key file")
	clientCAFile := flag.String("client_cafile", "",
		"path to TLS CA file to verify client certificate")
	debugMode := flag.Bool("debug", false, "enable debug mode")
	flag.Parse()

	var config *tls.Config
	if *useTLS {
		var err error
		config, err = newTLSConfig(*clientCertAuth, *certFile, *keyFile, *clientCAFile)
		if err != nil {
			glog.Fatal(err)
		}
	}

	var serverOptions []grpc.ServerOption
	if config != nil {
		serverOptions = append(serverOptions, grpc.Creds(credentials.NewTLS(config)))
	}

	serverOptions = append(serverOptions,
		grpc.MaxRecvMsgSize(math.MaxInt32),
	)

	grpcServer := grpc.NewServer(serverOptions...)
	s := &server{
		debugMode: *debugMode,
	}
	gnmireverse.RegisterGNMIReverseServer(grpcServer, s)

	listener, err := net.Listen("tcp", *addr)
	if err != nil {
		glog.Fatal(err)
	}
	if err := grpcServer.Serve(listener); err != nil {
		glog.Fatal(err)
	}
}

type server struct {
	gnmireverse.UnimplementedGNMIReverseServer
	debugMode bool
}

func (s *server) Publish(stream gnmireverse.GNMIReverse_PublishServer) error {
	for {
		resp, err := stream.Recv()
		if err != nil {
			return err
		}
		if err := gnmilib.LogSubscribeResponse(resp); err != nil {
			glog.Error(err)
		}
	}
}

func (s *server) PublishGet(stream gnmireverse.GNMIReverse_PublishGetServer) error {
	debugGet := &debugGet{}
	for {
		resp, err := stream.Recv()
		if err != nil {
			return err
		}
		if s.debugMode {
			debugGet.log(resp)
			continue
		}

		for _, notif := range resp.GetNotification() {
			var notifTarget string
			if target := notif.GetPrefix().GetTarget(); target != "" {
				notifTarget = " (" + target + ")"
			}
			notifTime := time.Unix(0, notif.GetTimestamp()).UTC()
			fmt.Printf("[%s]%s\n", notifTime.Format(time.RFC3339Nano), notifTarget)
			prefix := gnmilib.StrPath(notif.GetPrefix())
			for _, update := range notif.GetUpdate() {
				fmt.Println(path.Join(prefix, gnmilib.StrPath(update.GetPath())))
				fmt.Println(gnmilib.StrUpdateVal(update))
			}
		}
	}
}

type debugGet struct {
	lastReceiveTime time.Time
	lastNotifTime   time.Time
}

func (d *debugGet) log(res *gnmi.GetResponse) {
	// Time in which the GetResponse was received.
	receiveTime := time.Now().UTC()
	var lastReceiveAgo time.Duration
	if !d.lastReceiveTime.IsZero() {
		lastReceiveAgo = receiveTime.Sub(d.lastReceiveTime)
	}
	d.lastReceiveTime = receiveTime

	// Timestamp of the first notification of the GetResponse.
	var timestamp int64
	if len(res.GetNotification()) > 0 {
		timestamp = res.GetNotification()[0].GetTimestamp()
	}
	notifTime := time.Unix(0, timestamp).UTC()
	var lastNotifAgo time.Duration
	if !d.lastNotifTime.IsZero() {
		lastNotifAgo = notifTime.Sub(d.lastNotifTime)
	}
	d.lastNotifTime = notifTime

	// Difference between the GetResponse receive time and notification timestamp.
	latency := receiveTime.Sub(d.lastNotifTime)

	fmt.Printf("rx_time=%s notif_time=%s latency=%s"+
		" last_rx_ago=%s last_notif_ago=%s size_bytes=%d num_notifs=%d\n",
		receiveTime.Format(time.RFC3339Nano),
		notifTime.Format(time.RFC3339Nano),
		latency,
		lastReceiveAgo,
		lastNotifAgo,
		proto.Size(res),
		len(res.GetNotification()))
}
