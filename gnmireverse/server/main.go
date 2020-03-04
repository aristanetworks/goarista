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
	"net"

	gnmilib "github.com/aristanetworks/goarista/gnmi"
	"github.com/aristanetworks/goarista/gnmireverse"

	"github.com/aristanetworks/glog"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
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
