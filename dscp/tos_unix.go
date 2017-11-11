// Copyright (c) 2017 Arista Networks, Inc.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the COPYING file.

package dscp

import (
	"net"
	"os"
	"reflect"

	"golang.org/x/sys/unix"
)

// This works for the UNIX implementation of netFD, i.e. not on Windows and Plan9.
// This kludge is needed until https://github.com/golang/go/issues/9661 is fixed.
// value can be the reflection of a connection or a dialer.
func setTOS(ip net.IP, value reflect.Value, tos byte) error {
	netFD := value.Elem().FieldByName("fd").Elem()
	fd := int(netFD.FieldByName("pfd").FieldByName("Sysfd").Int())
	var proto, optname int
	if ip.To4() != nil {
		proto = unix.IPPROTO_IP
		optname = unix.IP_TOS
	} else {
		proto = unix.IPPROTO_IPV6
		optname = unix.IPV6_TCLASS
	}
	if err := unix.SetsockoptInt(fd, proto, optname, int(tos)); err != nil {
		return os.NewSyscallError("setsockopt", err)
	}
	return nil
}
