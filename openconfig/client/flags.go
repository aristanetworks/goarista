// Copyright (C) 2016  Arista Networks, Inc.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the COPYING file.

package client

import (
	"crypto/tls"
	"crypto/x509"
	"flag"
	"io/ioutil"
	"strings"

	"github.com/aristanetworks/glog"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

// ParseFlags registers some additional common flags,
// parses the flags, and returns the resulting gRPC options,
// and other settings to connect to the gRPC interface.
func ParseFlags() (username string, password string, get string,
	subscriptions, addrs []string,
	opts []grpc.DialOption) {

	var (
		addrsFlag = flag.String("addrs", "localhost:6042",
			"Comma-separated list of addresses of OpenConfig gRPC servers")

		caFileFlag = flag.String("cafile", "",
			"Path to server TLS certificate file")

		certFileFlag = flag.String("certfile", "",
			"Path to client TLS certificate file")

		getFlag = flag.String("get", "",
			"Path to get to upon connecting to the server")

		keyFileFlag = flag.String("keyfile", "",
			"Path to client TLS private key file")

		passwordFlag = flag.String("password", "",
			"Password to authenticate with")

		subscribeFlag = flag.String("subscribe", "",
			"Comma-separated list of paths to subscribe to upon connecting to the server")

		usernameFlag = flag.String("username", "",
			"Username to authenticate with")

		tlsFlag = flag.Bool("tls", false,
			"Enable TLS")
	)

	flag.Parse()
	if *tlsFlag || *caFileFlag != "" || *certFileFlag != "" {
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
	addrs = strings.Split(*addrsFlag, ",")
	if *getFlag != "" {
		if *subscribeFlag != "" {
			glog.Fatal("-get and -subscribe are mutually exclusive")
		} else if len(addrs) > 1 {
			glog.Fatal("Please specify a single address in -addrs with -get")
		}
	}
	if *subscribeFlag != "" {
		subscriptions = strings.Split(*subscribeFlag, ",")
	}
	return *usernameFlag, *passwordFlag, *getFlag, subscriptions, addrs, opts
}
