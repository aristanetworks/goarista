// Copyright (c) 2020 Arista Networks, Inc.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the COPYING file.

package server

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"math"
	"net"
	"os"
	"path"
	"strings"
	"time"

	"github.com/aristanetworks/glog"
	gnmilib "github.com/aristanetworks/goarista/gnmi"
	"github.com/aristanetworks/goarista/gnmireverse"
	"github.com/openconfig/gnmi/proto/gnmi"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	_ "google.golang.org/grpc/encoding/gzip" // Enable gzip encoding for the server.
	"google.golang.org/grpc/peer"
	"google.golang.org/protobuf/proto"
)

// The debug flag is composed of these flag bits.
// These flag bits are bitwise OR'ed to select what debug information is printed.
const (
	debugSilent             = 1 << iota // Do not print anything.
	debugGetResponseSummary             // Print GetResponse summary.
	debugResponseProto                  // Print response protobuf text format.
	debugUpdates                        // Print update receive and notification times.
	debugUpdatesTiming                  // Print debugUpdates and update timing calculations.
	debugUpdatesPaths                   // Print debugUpdates and update paths.
	debugUpdatesValues                  // Print debugUpdates and update values.
	debugUpdatesAll         = debugUpdates | debugUpdatesTiming | debugUpdatesPaths |
		debugUpdatesValues // Print all update information.
)

// Use log.Logger for thread-safe printing.
var logger = log.New(os.Stdout, "", 0)

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

	debugFlagUsage := `Debug flags. Use bitwise OR to select what to print.
  -1  Enable all debug flags.
   1  Silent mode: do not print anything. Any other enabled flag overrides silent mode.
   2  Print GetResponse summary: receive time, last receive time, response size and
      number of notifications.
   4  Print GetResponse/SubscribeResponse protobuf text format.
   8  Print update receive time and notification time.
  16  Print update receive time, notification time and update timing calculations: difference
      between the last receive time of the same update path, difference between the last
      notification time of the same update path and latency (receive time - notification time).
  32  Print update receive time, notification time and update path.
  64  Print update receive time, notification time and update value.
Example: Use 50 (= 2 | 16 | 32) to print the GetResponse summary and the receive time,
         notification time, timing calculations and path of updates.`
	debugFlag := flag.Int("debug", 0, debugFlagUsage)

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
		debugFlag: *debugFlag,
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
	debugFlag int
	gnmireverse.UnimplementedGNMIReverseServer
}

func (s *server) Publish(stream gnmireverse.GNMIReverse_PublishServer) error {
	debugger := newDebugger(stream.Context(), "subscribe", s.debugFlag)
	for {
		resp, err := stream.Recv()
		if err != nil {
			return err
		}
		if s.debugFlag != 0 {
			debugger.logSubscribeResponse(resp)
			continue
		}

		if err := gnmilib.LogSubscribeResponse(resp); err != nil {
			glog.Error(err)
		}
	}
}

func (s *server) PublishGet(stream gnmireverse.GNMIReverse_PublishGetServer) error {
	debugger := newDebugger(stream.Context(), "get", s.debugFlag)
	for {
		resp, err := stream.Recv()
		if err != nil {
			return err
		}
		if s.debugFlag != 0 {
			debugger.logGetResponse(resp)
			continue
		}

		for _, notif := range resp.GetNotification() {
			notifTime := time.Unix(0, notif.GetTimestamp()).UTC().Format(time.RFC3339Nano)
			var notifTarget string
			if target := notif.GetPrefix().GetTarget(); target != "" {
				notifTarget = " (" + target + ")"
			}
			prefix := gnmilib.StrPath(notif.GetPrefix())
			for _, update := range notif.GetUpdate() {
				pathValSeperator := " = "
				// If the value is a subtree, print the subtree on a new line.
				if _, ok := update.GetVal().GetValue().(*gnmi.TypedValue_JsonIetfVal); ok {
					pathValSeperator = "\n"
				}
				pth := path.Join(prefix, gnmilib.StrPath(update.GetPath()))
				val := gnmilib.StrUpdateVal(update)
				logger.Printf("[%s]%s %s%s%s",
					notifTime,
					notifTarget,
					pth,
					pathValSeperator,
					val,
				)
			}
		}
	}
}

// debugger stores debug information related to responses received from a client.
type debugger struct {
	clientAddr       string               // Address of the client sending the response.
	responseName     string               // Name of the response received.
	responsesCounter uint64               // Updates of the same response have the same counter.
	lastResponseTime time.Time            // Time of the last response received.
	lastUpdateTimes  map[string]time.Time // Map of update paths to the last notification time.
	logBuffer        strings.Builder      // Resultant log text to be printed.
	debugFlag        int                  // Debug flag to control what to print.
}

func newDebugger(ctx context.Context, responseName string, debugFlag int) *debugger {
	var clientAddr string
	if pr, ok := peer.FromContext(ctx); ok && pr.Addr != nil {
		clientAddr = pr.Addr.String()
	}
	return &debugger{
		clientAddr:      clientAddr,
		responseName:    responseName,
		lastUpdateTimes: make(map[string]time.Time),
		debugFlag:       debugFlag,
	}
}

// logGetResponse prints GetResponse debug information based on
// the bits of the debug flag.
func (d *debugger) logGetResponse(res *gnmi.GetResponse) {
	if d.debugFlag^debugSilent == 0 { // Any other enabled flag overrides silence.
		return
	}
	receiveTime := time.Now().UTC()
	if d.debugFlag&debugGetResponseSummary != 0 {
		d.logGetResponseSummary(res, receiveTime)
	}
	if d.debugFlag&debugResponseProto != 0 {
		d.logResponseProto(res)
	}
	if d.debugFlag&debugUpdatesAll != 0 {
		d.logGetResponseNotifs(res, receiveTime)
	}
	logger.Print(d.logBuffer.String())
	d.logBuffer.Reset()
	d.lastResponseTime = receiveTime
	d.responsesCounter++
}

func (d *debugger) logGetResponseSummary(res *gnmi.GetResponse, receiveTime time.Time) {
	debugFields := append(
		d.logPrefixFields(receiveTime),
		d.logLastReceiveAgoField(receiveTime),
		fmt.Sprintf("size_bytes=%d", proto.Size(res)),
		fmt.Sprintf("num_notifs=%d", len(res.GetNotification())),
	)
	d.logBuffer.WriteString(strings.Join(debugFields, " "))
	d.logBuffer.WriteByte('\n')
}

func (d *debugger) logResponseProto(res fmt.Stringer) {
	d.logBuffer.WriteString(res.String())
	d.logBuffer.WriteByte('\n')
}

func (d *debugger) logGetResponseNotifs(res *gnmi.GetResponse, receiveTime time.Time) {
	for _, notif := range res.GetNotification() {
		d.logNotif(notif, receiveTime)
	}
}

func (d *debugger) logNotif(notif *gnmi.Notification, receiveTime time.Time) {
	notifTime := time.Unix(0, notif.GetTimestamp()).UTC()
	prefix := gnmilib.StrPath(notif.GetPrefix())
	for _, update := range notif.GetUpdate() {
		pth := path.Join(prefix, gnmilib.StrPath(update.GetPath()))
		val := gnmilib.StrValCompactJSON(update.GetVal())
		d.logUpdate(receiveTime, notifTime, pth, val)
	}
}

func (d *debugger) logUpdate(receiveTime time.Time, notifTime time.Time, path, val string) {
	debugFields := append(
		d.logPrefixFields(receiveTime),
		fmt.Sprintf("notif_time=%s", notifTime.Format(time.RFC3339Nano)),
	)
	if d.debugFlag&debugUpdatesTiming != 0 {
		// Difference between the notification time and last notification time.
		if _, ok := d.lastUpdateTimes[path]; !ok {
			d.lastUpdateTimes[path] = time.Time{}
		}
		lastUpdateTime := d.lastUpdateTimes[path]
		var lastNotifAgo time.Duration
		if !lastUpdateTime.IsZero() {
			lastNotifAgo = notifTime.Sub(lastUpdateTime)
		}
		d.lastUpdateTimes[path] = notifTime

		// Difference between the response receive time and notification time.
		var latency time.Duration
		if !lastUpdateTime.IsZero() {
			latency = receiveTime.Sub(lastUpdateTime)
		}

		debugFields = append(debugFields,
			d.logLastReceiveAgoField(receiveTime),
			fmt.Sprintf("last_notif_ago=%s", lastNotifAgo),
			fmt.Sprintf("latency=%s", latency),
		)
	}
	if d.debugFlag&debugUpdatesPaths != 0 {
		debugFields = append(debugFields, fmt.Sprintf("path=%s", path))
	}
	if d.debugFlag&debugUpdatesValues != 0 {
		debugFields = append(debugFields, fmt.Sprintf("val=%s", val))
	}
	d.logBuffer.WriteString(strings.Join(debugFields, " "))
	d.logBuffer.WriteByte('\n')
}

// logSubscribeResponse prints SubscribeResponse debug information based on
// the bits of the debug flag.
func (d *debugger) logSubscribeResponse(res *gnmi.SubscribeResponse) {
	if d.debugFlag^debugSilent == 0 { // Any other enabled flag overrides silence.
		return
	}
	receiveTime := time.Now().UTC()
	if d.debugFlag&debugResponseProto != 0 {
		d.logResponseProto(res)
	}
	if d.debugFlag&debugUpdatesAll != 0 {
		d.logSubscribeResponseNotif(res, receiveTime)
	}
	logger.Print(d.logBuffer.String())
	d.logBuffer.Reset()
	d.lastResponseTime = receiveTime
	d.responsesCounter++
}

func (d *debugger) logSubscribeResponseNotif(res *gnmi.SubscribeResponse, receiveTime time.Time) {
	if res.GetSyncResponse() {
		debugFields := d.logPrefixFields(receiveTime)
		if d.debugFlag&debugUpdatesTiming != 0 {
			debugFields = append(debugFields, d.logLastReceiveAgoField(receiveTime))
		}
		debugFields = append(debugFields, "sync_response=true")
		d.logBuffer.WriteString(strings.Join(debugFields, " "))
		d.logBuffer.WriteByte('\n')
		return
	}
	d.logNotif(res.GetUpdate(), receiveTime)
}

// logPrefixFields returns the base debug prefix fields.
func (d *debugger) logPrefixFields(receiveTime time.Time) []string {
	return []string{
		fmt.Sprintf("client=%s", d.clientAddr),
		fmt.Sprintf("res=%s", d.responseName),
		fmt.Sprintf("n=%d", d.responsesCounter),
		fmt.Sprintf("rx_time=%s", receiveTime.Format(time.RFC3339Nano)),
	}
}

// logLastReceiveAgoField returns the debug field of difference between the
// the response receive time and last response receive time.
func (d *debugger) logLastReceiveAgoField(receiveTime time.Time) string {
	var lastReceiveAgo time.Duration
	if !d.lastResponseTime.IsZero() {
		lastReceiveAgo = receiveTime.Sub(d.lastResponseTime)
	}
	return fmt.Sprintf("last_rx_ago=%s", lastReceiveAgo)
}
