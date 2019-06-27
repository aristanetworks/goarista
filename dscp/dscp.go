// Copyright (c) 2019 Arista Networks, Inc.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the COPYING file.

package dscp

import (
	"context"
	"net"
	"os"
	"strings"
	"syscall"

	"github.com/aristanetworks/glog"
	"golang.org/x/sys/unix"
)

// ListenTCPWithTOS is similar to net.ListenTCP but with the socket configured
// to the use the given ToS (Type of Service), to specify DSCP / ECN / class
// of service flags to use for incoming connections.
func ListenTCPWithTOS(address *net.TCPAddr, tos byte) (*net.TCPListener, error) {
	cfg := net.ListenConfig{
		Control: func(network, address string, c syscall.RawConn) error {
			return setTOS(network, c, tos)
		},
	}

	lsnr, err := cfg.Listen(context.Background(), "tcp", address.String())
	if err != nil {
		return nil, err
	}

	return lsnr.(*net.TCPListener), err
}

func setTOS(network string, c syscall.RawConn, tos byte) error {
	return c.Control(func(fd uintptr) {
		// Configure ipv4 TOS for both IPv4 and IPv6 networks because
		// v4 connections can still come over v6 networks.
		err := unix.SetsockoptInt(int(fd), unix.IPPROTO_IP, unix.IP_TOS, int(tos))
		if err != nil {
			glog.Errorf("failed to configure IP_TOS: %v", os.NewSyscallError("setsockopt", err))
		}
		if strings.HasSuffix(network, "4") {
			// Skip configuring IPv6 when we know we are using an IPv4
			// network to avoid error.
			return
		}
		err6 := unix.SetsockoptInt(int(fd), unix.IPPROTO_IPV6, unix.IPV6_TCLASS, int(tos))
		if err6 != nil {
			glog.Errorf(
				"failed to configure IPV6_TCLASS, traffic may not use the configured DSCP: %v",
				os.NewSyscallError("setsockopt", err6))
		}

	})
}
