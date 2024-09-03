// Copyright (c) 2015 Arista Networks, Inc.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the COPYING file.

// Package monitor provides an embedded HTTP server to expose
// metrics for monitoring
package monitor

import (
	"expvar"
	"fmt"
	"net"
	"net/http"
	_ "net/http/pprof" // Go documentation recommended usage

	"github.com/aristanetworks/goarista/monitor/internal/loglevel"
	"github.com/aristanetworks/goarista/netns"

	"github.com/aristanetworks/glog"
)

// Server represents a monitoring server
type Server interface {
	Run(serveMux *http.ServeMux)
	Serve(serveMux *http.ServeMux) error
}

// server contains information for the monitoring server
type server struct {
	vrfName string
	// Server name e.g. host[:port]
	serverName string
	loglevel   http.Handler
}

// NewServer creates a new server struct
func NewServer(address string) Server {
	vrfName, addr, err := netns.ParseAddress(address)
	if err != nil {
		glog.Errorf("Failed to parse address: %s", err)
	}
	return &server{
		vrfName:    vrfName,
		serverName: addr,
		loglevel:   loglevel.Handler(),
	}
}

func debugHandler(w http.ResponseWriter, r *http.Request) {
	indexTmpl := `<html>
	<head>
	<title>/debug</title>
	</head>
	<body>
	<p>/debug</p>
	<div><a href="/debug/vars">vars</a></div>
	<div><a href="/debug/pprof">pprof</a></div>
	<div><a href="/debug/loglevel">loglevel</a></div>
	</body>
	</html>
	`
	fmt.Fprintf(w, indexTmpl)
}

// PrintableHistogram represents a Histogram that can be printed as
// a chart.
type PrintableHistogram interface {
	Print() string
}

// Pretty prints the latency histograms
func histogramHandler(w http.ResponseWriter, r *http.Request) {
	expvar.Do(func(kv expvar.KeyValue) {
		if hist, ok := kv.Value.(PrintableHistogram); ok {
			w.Write([]byte(hist.Print()))
		}
	})
}

// Run calls Serve. On error the program exits.
func (s *server) Run(serveMux *http.ServeMux) {
	if err := s.Serve(serveMux); err != nil {
		glog.Fatal(err)
	}
}

// Serve registers handlers and starts serving.
func (s *server) Serve(serveMux *http.ServeMux) error {
	serveMux.HandleFunc("/debug", debugHandler)
	serveMux.HandleFunc("/debug/histograms", histogramHandler)
	serveMux.Handle("/debug/loglevel", s.loglevel)

	var listener net.Listener
	err := netns.Do(s.vrfName, func() error {
		var err error
		listener, err = net.Listen("tcp", s.serverName)
		return err
	})
	if err != nil {
		return fmt.Errorf("could not start monitor server in VRF %q: %s", s.vrfName, err)
	}
	_, boundPort, err := net.SplitHostPort(listener.Addr().String())
	if err != nil {
		return fmt.Errorf("failed to split monitor addr: %s", err)
	}
	glog.Infof("monitoring served on port: %s", boundPort)

	return http.Serve(listener, serveMux)
}
