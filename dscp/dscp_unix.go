// Copyright (c) 2019 Arista Networks, Inc.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the COPYING file.

//go:build linux || darwin
// +build linux darwin

package dscp

import (
	"context"
	"net"
	"os"
	"strings"
	"syscall"

	"github.com/aristanetworks/goarista/logger"
	"golang.org/x/sys/unix"
)

// ListenTCPWithTOS is similar to net.ListenTCP but with the socket configured
// to the use the given ToS (Type of Service), to specify DSCP / ECN / class
// of service flags to use for incoming connections.
func ListenTCPWithTOS(address *net.TCPAddr, tos byte) (*net.TCPListener, error) {
	return ListenTCPWithTOSLogger(address, tos, logger.Std)
}

// ListenTCPWithTOSLogger is similar to net.ListenTCP but with the
// socket configured to the use the given ToS (Type of Service), to
// specify DSCP / ECN / class of service flags to use for incoming
// connections. Allows passing in a Logger.
func ListenTCPWithTOSLogger(address *net.TCPAddr, tos byte, l logger.Logger) (*net.TCPListener,
	error) {
	cfg := net.ListenConfig{
		Control: func(network, address string, c syscall.RawConn) error {
			return SetTOSLogger(network, c, tos, l)
		},
	}

	lsnr, err := cfg.Listen(context.Background(), "tcp", address.String())
	if err != nil {
		return nil, err
	}

	return lsnr.(*net.TCPListener), err
}

// SetTOS will set the TOS byte on a unix system. It's intended to be
// used in a net.Dialer's Control function.
func SetTOS(network string, c syscall.RawConn, tos byte) error {
	return SetTOSLogger(network, c, tos, logger.Std)
}

// SetTOSLogger will set the TOS byte on a unix system. It's intended
// to be used in a net.Dialer's Control function. Allows passing in a
// Logger.
func SetTOSLogger(network string, c syscall.RawConn, tos byte, l logger.Logger) error {
	return c.Control(func(fd uintptr) {
		// Configure ipv4 TOS for both IPv4 and IPv6 networks because
		// v4 connections can still come over v6 networks.
		err := unix.SetsockoptInt(int(fd), unix.IPPROTO_IP, unix.IP_TOS, int(tos))
		if err != nil {
			l.Errorf("failed to configure IP_TOS: %v", os.NewSyscallError("setsockopt", err))
		}
		if strings.HasSuffix(network, "4") {
			// Skip configuring IPv6 when we know we are using an IPv4
			// network to avoid error.
			return
		}
		err6 := unix.SetsockoptInt(int(fd), unix.IPPROTO_IPV6, unix.IPV6_TCLASS, int(tos))
		if err6 != nil {
			l.Errorf(
				"failed to configure IPV6_TCLASS, traffic may not use the configured DSCP: %v",
				os.NewSyscallError("setsockopt", err6))
		}

	})
}
