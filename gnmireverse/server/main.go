// Copyright (c) 2020 Arista Networks, Inc.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the COPYING file.

package main

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

	gnmilib "github.com/aristanetworks/goarista/gnmi"
	"github.com/aristanetworks/goarista/gnmireverse"

	"github.com/aristanetworks/glog"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	_ "google.golang.org/grpc/encoding/gzip" // Enable gzip encoding for the server.
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

func main() {
	addr := flag.String("addr", "127.0.0.1:6035", "address to listen on")
	useTLS := flag.Bool("tls", true, "set to false to disable TLS in collector")
	clientCertAuth := flag.Bool("client_cert_auth", false,
		"require and verify a client certificate. -client_cafile must also be set.")
	certFile := flag.String("certfile", "", "path to TLS certificate file")
	keyFile := flag.String("keyfile", "", "path to TLS key file")
	clientCAFile := flag.String("client_cafile", "",
		"path to TLS CA file to verify client certificate")
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
	s := &server{}
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
	for {
		resp, err := stream.Recv()
		if err != nil {
			return err
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
