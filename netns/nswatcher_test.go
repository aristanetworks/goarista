// Copyright (c) 2024 Arista Networks, Inc.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the COPYING file.

package netns

import (
	"errors"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aristanetworks/goarista/glog"
	"github.com/aristanetworks/goarista/logger"
)

type mockOperator struct {
	operations         int
	operation          chan int
	operationCompletes int
	operationComplete  chan int
	teardowns          int
	failOperation      bool
	teardown           chan int
}

func (o *mockOperator) NetNsOperation() error {
	o.operations += 1
	defer func() {
		o.operation <- o.operations
	}()
	if o.failOperation {
		return errors.New("Failed operation")
	}
	return nil
}

func (o *mockOperator) NetNsOperationSuccess() {
	o.operationCompletes += 1
	o.operationComplete <- o.operationCompletes
}

func (o *mockOperator) NetNsTeardown() {
	o.teardowns += 1
	o.teardown <- o.teardowns
}

func (o *mockOperator) reset() {
	o.operations = 0
	o.operationCompletes = 0
	o.teardowns = 0
}

func TestNSWatcher(t *testing.T) {
	hasMount = func(_ string, _ logger.Logger) bool {
		return true
	}
	logger := &glog.Glog{}

	nsDir, err := ioutil.TempDir("", "netns")
	if err != nil {
		t.Fatalf("Can't create temp file: %v", err)
	}
	defer os.RemoveAll(nsDir)

	operator := &mockOperator{
		operation:         make(chan int),
		operationComplete: make(chan int),
		teardown:          make(chan int),
	}
	nsWatcher, err := newNsWatcherWithDir(nsDir, "ns-yolo", logger, operator)
	if err != nil {
		t.Fatalf("Can't create watcher: %v", err)
	}

	nsFile := filepath.Join(nsDir, "ns-yolo")
	for i := 1; i <= 3; i++ {
		operator.reset()
		if err = ioutil.WriteFile(nsFile, []byte{}, os.FileMode(0777)); err != nil {
			t.Fatalf("Can't create ns file: %v", err)
		}

		<-operator.operation
		<-operator.operationComplete
		os.Remove(nsFile)
		<-operator.teardown

		if operator.operations != 1 {
			t.Fatalf("%v: Expected makeListener to be called once, but it was called %v times", i,
				operator.operations)
		}
		if operator.operationCompletes != 1 {
			t.Fatalf("%v: Expected accept to be called once, but it was called %v times", i,
				operator.operationCompletes)
		}
		if operator.teardowns != 1 {
			t.Fatalf("%v: Expected close to be called once, but it was called %v times", i,
				operator.teardowns)
		}
	}

	// Verify that if the operation fails that NetNsOperationSuccess is not called
	operator.reset()
	operator.failOperation = true
	if err = ioutil.WriteFile(nsFile, []byte{}, os.FileMode(0777)); err != nil {
		t.Fatalf("Can't create ns file: %v", err)
	}
	<-operator.operation
	nsWatcher.Close()
	<-operator.teardown
	if operator.operationCompletes != 0 {
		t.Fatalf("Expected operationComplete to not be called, was called %v times",
			operator.operationCompletes)
	}
}

func TestDefaultNSWatcher(t *testing.T) {
	operator := &mockOperator{
		operation:         make(chan int),
		operationComplete: make(chan int),
		teardown:          make(chan int),
	}
	go func() {
		<-operator.operation
		<-operator.operationComplete
	}()
	nsWatcher, err := newDefaultNsWatcher(operator)
	if err != nil {
		t.Fatalf("Couldn't create watcher, %v", err)
	}
	go nsWatcher.Close()
	<-operator.teardown
	if operator.operations != 1 {
		t.Fatalf("Expected operation to be called once, was called %v times",
			operator.operation)
	}
	if operator.operationCompletes != 1 {
		t.Fatalf("Expected operationComplete to be called once, was called %v times",
			operator.operationCompletes)
	}

	// Verify that if the operation fails that NetNsOperationSuccess is not called
	operator.reset()
	operator.failOperation = true
	go func() {
		<-operator.operation
	}()
	nsWatcher, err = newDefaultNsWatcher(operator)
	if err == nil {
		t.Fatalf("defaultNsWatcher construction should have failed")
	}
	if operator.operations != 1 {
		t.Fatalf("Expected operation to be called once, was called %v times",
			operator.operationCompletes)
	}
	if operator.operationCompletes != 0 {
		t.Fatalf("Expected operationComplete to not be called, was called %v times",
			operator.operationCompletes)
	}
	if operator.teardowns != 0 {
		t.Fatalf("Expected teardown to not be called, was called %v times",
			operator.teardowns)
	}
}

func TestHasMount(t *testing.T) {
	testCases := []struct {
		desc     string
		input    string
		expected bool
	}{
		{
			desc: "Mounted as nsfs",
			input: `
none / aufs rw,relatime,si=7aaed56e5ecd215c 0 0
none /.overlay tmpfs rw,relatime,size=593256k,mode=755,idr=enabled 0 0
proc /proc proc rw,nosuid,nodev,noexec,relatime 0 0
sysfs /sys sysfs rw,nosuid,nodev,noexec,relatime 0 0
devtmpfs /dev devtmpfs rw,size=8192k,nr_inodes=485215,mode=755 0 0
securityfs /sys/kernel/security securityfs rw,nosuid,nodev,noexec,relatime 0 0
tmpfs /dev/shm tmpfs rw,nosuid,nodev 0 0
devpts /dev/pts devpts rw,nosuid,noexec,relatime,gid=5,mode=620,ptmxmode=000 0 0
tmpfs /run tmpfs rw,nosuid,nodev,mode=755 0 0
tmpfs /sys/fs/cgroup tmpfs rw,nosuid,nodev,noexec,mode=755 0 0
cgroup /sys/fs/cgroup/systemd cgroup rw,nosuid,nodev,noexec,relatime,name=systemd 0 0
cgroup /sys/fs/cgroup/cpuset cgroup rw,nosuid,nodev,noexec,relatime,cpuset 0 0
cgroup /sys/fs/cgroup/cpu,cpuacct cgroup rw,nosuid,nodev,noexec,relatime,cpu,cpuacct 0 0
cgroup /sys/fs/cgroup/blkio cgroup rw,nosuid,nodev,noexec,relatime,blkio 0 0
cgroup /sys/fs/cgroup/memory cgroup rw,nosuid,nodev,noexec,relatime,memory 0 0
cgroup /sys/fs/cgroup/devices cgroup rw,nosuid,nodev,noexec,relatime,devices 0 0
cgroup /sys/fs/cgroup/freezer cgroup rw,nosuid,nodev,noexec,relatime,freezer 0 0
cgroup /sys/fs/cgroup/net_cls cgroup rw,nosuid,nodev,noexec,relatime,net_cls 0 0
configfs /sys/kernel/config configfs rw,relatime 0 0
debugfs /sys/kernel/debug debugfs rw,relatime 0 0
tmpfs /tmp tmpfs rw,size=593256k 0 0
hugetlbfs /dev/hugepages hugetlbfs rw,relatime 0 0
mqueue /dev/mqueue mqueue rw,relatime 0 0
tmpfs /.deltas tmpfs rw,relatime,size=65536k 0 0
tmpfs /var/run tmpfs rw,relatime,size=65536k 0 0
tmpfs /var/run/netns tmpfs rw,relatime 0 0
tmpfs /.deltas/var/run/netns tmpfs rw,relatime 0 0
tmpfs /var/tmp tmpfs rw,relatime,size=65536k 0 0
tmpfs /var/core tmpfs rw,relatime,size=395504k 0 0
tmpfs /var/log tmpfs rw,relatime,size=395504k 0 0
tmpfs /var/shmem tmpfs rw,relatime,size=988756k 0 0
/monitor /monitor debugfs rw,relatime 0 0
/dev/sda1 /mnt/flash vfat rw,dirsync,noatime,gid=88,fmask=0007,dmask=0007,allow_utime=0020 0 0
nsfs /var/run/netns/default nsfs rw 0 0
nsfs /.deltas/var/run/netns/default nsfs rw 0 0
nsfs /var/run/netns/ns-OOB-Management nsfs rw 0 0
nsfs /.deltas/var/run/netns/ns-OOB-Management nsfs rw 0 0
`,
			expected: true,
		},
		{
			desc: "Mounted as proc",
			input: `
none / aufs rw,relatime,si=7aaed56e5ecd215c 0 0
none /.overlay tmpfs rw,relatime,size=593256k,mode=755,idr=enabled 0 0
proc /proc proc rw,nosuid,nodev,noexec,relatime 0 0
sysfs /sys sysfs rw,nosuid,nodev,noexec,relatime 0 0
devtmpfs /dev devtmpfs rw,size=8192k,nr_inodes=485215,mode=755 0 0
securityfs /sys/kernel/security securityfs rw,nosuid,nodev,noexec,relatime 0 0
tmpfs /dev/shm tmpfs rw,nosuid,nodev 0 0
devpts /dev/pts devpts rw,nosuid,noexec,relatime,gid=5,mode=620,ptmxmode=000 0 0
tmpfs /run tmpfs rw,nosuid,nodev,mode=755 0 0
tmpfs /sys/fs/cgroup tmpfs rw,nosuid,nodev,noexec,mode=755 0 0
cgroup /sys/fs/cgroup/systemd cgroup rw,nosuid,nodev,noexec,relatime,name=systemd 0 0
cgroup /sys/fs/cgroup/cpuset cgroup rw,nosuid,nodev,noexec,relatime,cpuset 0 0
cgroup /sys/fs/cgroup/cpu,cpuacct cgroup rw,nosuid,nodev,noexec,relatime,cpu,cpuacct 0 0
cgroup /sys/fs/cgroup/blkio cgroup rw,nosuid,nodev,noexec,relatime,blkio 0 0
cgroup /sys/fs/cgroup/memory cgroup rw,nosuid,nodev,noexec,relatime,memory 0 0
cgroup /sys/fs/cgroup/devices cgroup rw,nosuid,nodev,noexec,relatime,devices 0 0
cgroup /sys/fs/cgroup/freezer cgroup rw,nosuid,nodev,noexec,relatime,freezer 0 0
cgroup /sys/fs/cgroup/net_cls cgroup rw,nosuid,nodev,noexec,relatime,net_cls 0 0
configfs /sys/kernel/config configfs rw,relatime 0 0
debugfs /sys/kernel/debug debugfs rw,relatime 0 0
tmpfs /tmp tmpfs rw,size=593256k 0 0
hugetlbfs /dev/hugepages hugetlbfs rw,relatime 0 0
mqueue /dev/mqueue mqueue rw,relatime 0 0
tmpfs /.deltas tmpfs rw,relatime,size=65536k 0 0
tmpfs /var/run tmpfs rw,relatime,size=65536k 0 0
tmpfs /var/run/netns tmpfs rw,relatime 0 0
tmpfs /.deltas/var/run/netns tmpfs rw,relatime 0 0
tmpfs /var/tmp tmpfs rw,relatime,size=65536k 0 0
tmpfs /var/core tmpfs rw,relatime,size=395504k 0 0
tmpfs /var/log tmpfs rw,relatime,size=395504k 0 0
tmpfs /var/shmem tmpfs rw,relatime,size=988756k 0 0
/monitor /monitor debugfs rw,relatime 0 0
/dev/sda1 /mnt/flash vfat rw,dirsync,noatime,gid=88,fmask=0007,dmask=0007,allow_utime=0020 0 0
proc /var/run/netns/default proc rw 0 0
proc /.deltas/var/run/netns/default proc rw 0 0
proc /var/run/netns/ns-OOB-Management proc rw 0 0
proc /.deltas/var/run/netns/ns-OOB-Management proc rw 0 0
`,
			expected: true,
		},
		{
			desc: "Not mounted",
			input: `
none / aufs rw,relatime,si=7aaed56e5ecd215c 0 0
none /.overlay tmpfs rw,relatime,size=593256k,mode=755,idr=enabled 0 0
proc /proc proc rw,nosuid,nodev,noexec,relatime 0 0
sysfs /sys sysfs rw,nosuid,nodev,noexec,relatime 0 0
devtmpfs /dev devtmpfs rw,size=8192k,nr_inodes=485215,mode=755 0 0
securityfs /sys/kernel/security securityfs rw,nosuid,nodev,noexec,relatime 0 0
tmpfs /dev/shm tmpfs rw,nosuid,nodev 0 0
devpts /dev/pts devpts rw,nosuid,noexec,relatime,gid=5,mode=620,ptmxmode=000 0 0
tmpfs /run tmpfs rw,nosuid,nodev,mode=755 0 0
tmpfs /sys/fs/cgroup tmpfs rw,nosuid,nodev,noexec,mode=755 0 0
cgroup /sys/fs/cgroup/systemd cgroup rw,nosuid,nodev,noexec,relatime,name=systemd 0 0
cgroup /sys/fs/cgroup/cpuset cgroup rw,nosuid,nodev,noexec,relatime,cpuset 0 0
cgroup /sys/fs/cgroup/cpu,cpuacct cgroup rw,nosuid,nodev,noexec,relatime,cpu,cpuacct 0 0
cgroup /sys/fs/cgroup/blkio cgroup rw,nosuid,nodev,noexec,relatime,blkio 0 0
cgroup /sys/fs/cgroup/memory cgroup rw,nosuid,nodev,noexec,relatime,memory 0 0
cgroup /sys/fs/cgroup/devices cgroup rw,nosuid,nodev,noexec,relatime,devices 0 0
cgroup /sys/fs/cgroup/freezer cgroup rw,nosuid,nodev,noexec,relatime,freezer 0 0
cgroup /sys/fs/cgroup/net_cls cgroup rw,nosuid,nodev,noexec,relatime,net_cls 0 0
configfs /sys/kernel/config configfs rw,relatime 0 0
debugfs /sys/kernel/debug debugfs rw,relatime 0 0
tmpfs /tmp tmpfs rw,size=593256k 0 0
hugetlbfs /dev/hugepages hugetlbfs rw,relatime 0 0
mqueue /dev/mqueue mqueue rw,relatime 0 0
tmpfs /.deltas tmpfs rw,relatime,size=65536k 0 0
tmpfs /var/run tmpfs rw,relatime,size=65536k 0 0
tmpfs /var/run/netns tmpfs rw,relatime 0 0
tmpfs /.deltas/var/run/netns tmpfs rw,relatime 0 0
tmpfs /var/tmp tmpfs rw,relatime,size=65536k 0 0
tmpfs /var/core tmpfs rw,relatime,size=395504k 0 0
tmpfs /var/log tmpfs rw,relatime,size=395504k 0 0
tmpfs /var/shmem tmpfs rw,relatime,size=988756k 0 0
/monitor /monitor debugfs rw,relatime 0 0
/dev/sda1 /mnt/flash vfat rw,dirsync,noatime,gid=88,fmask=0007,dmask=0007,allow_utime=0020 0 0
proc /var/run/netns/default proc rw 0 0
proc /.deltas/var/run/netns/default proc rw 0 0
`,
		},
	}

	for _, tc := range testCases {
		rdr := strings.NewReader(tc.input)
		if r := hasMountInProcMounts(rdr, "/var/run/netns/ns-OOB-Management"); r != tc.expected {
			t.Errorf("%v: unexpected result %v, expected %v", tc.desc, r, tc.expected)
		}
	}
}

func TestGetNsDir(t *testing.T) {
	testCases := []struct {
		desc     string
		input    string
		expected string
		err      string
	}{
		{
			desc: "Mounted in /var/run/netns",
			input: `
none / aufs rw,relatime,si=7aaed56e5ecd215c 0 0
none /.overlay tmpfs rw,relatime,size=593256k,mode=755,idr=enabled 0 0
proc /proc proc rw,nosuid,nodev,noexec,relatime 0 0
sysfs /sys sysfs rw,nosuid,nodev,noexec,relatime 0 0
devtmpfs /dev devtmpfs rw,size=8192k,nr_inodes=485215,mode=755 0 0
securityfs /sys/kernel/security securityfs rw,nosuid,nodev,noexec,relatime 0 0
tmpfs /dev/shm tmpfs rw,nosuid,nodev 0 0
devpts /dev/pts devpts rw,nosuid,noexec,relatime,gid=5,mode=620,ptmxmode=000 0 0
tmpfs /run tmpfs rw,nosuid,nodev,mode=755 0 0
tmpfs /sys/fs/cgroup tmpfs rw,nosuid,nodev,noexec,mode=755 0 0
cgroup /sys/fs/cgroup/systemd cgroup rw,nosuid,nodev,noexec,relatime,name=systemd 0 0
cgroup /sys/fs/cgroup/cpuset cgroup rw,nosuid,nodev,noexec,relatime,cpuset 0 0
cgroup /sys/fs/cgroup/cpu,cpuacct cgroup rw,nosuid,nodev,noexec,relatime,cpu,cpuacct 0 0
cgroup /sys/fs/cgroup/blkio cgroup rw,nosuid,nodev,noexec,relatime,blkio 0 0
cgroup /sys/fs/cgroup/memory cgroup rw,nosuid,nodev,noexec,relatime,memory 0 0
cgroup /sys/fs/cgroup/devices cgroup rw,nosuid,nodev,noexec,relatime,devices 0 0
cgroup /sys/fs/cgroup/freezer cgroup rw,nosuid,nodev,noexec,relatime,freezer 0 0
cgroup /sys/fs/cgroup/net_cls cgroup rw,nosuid,nodev,noexec,relatime,net_cls 0 0
configfs /sys/kernel/config configfs rw,relatime 0 0
debugfs /sys/kernel/debug debugfs rw,relatime 0 0
tmpfs /tmp tmpfs rw,size=593256k 0 0
hugetlbfs /dev/hugepages hugetlbfs rw,relatime 0 0
mqueue /dev/mqueue mqueue rw,relatime 0 0
tmpfs /.deltas tmpfs rw,relatime,size=65536k 0 0
tmpfs /var/run tmpfs rw,relatime,size=65536k 0 0
tmpfs /var/run/netns tmpfs rw,relatime 0 0
tmpfs /.deltas/var/run/netns tmpfs rw,relatime 0 0
tmpfs /var/tmp tmpfs rw,relatime,size=65536k 0 0
tmpfs /var/core tmpfs rw,relatime,size=395504k 0 0
tmpfs /var/log tmpfs rw,relatime,size=395504k 0 0
tmpfs /var/shmem tmpfs rw,relatime,size=988756k 0 0
/monitor /monitor debugfs rw,relatime 0 0
/dev/sda1 /mnt/flash vfat rw,dirsync,noatime,gid=88,fmask=0007,dmask=0007,allow_utime=0020 0 0
nsfs /var/run/netns/default nsfs rw 0 0
nsfs /.deltas/var/run/netns/default nsfs rw 0 0
nsfs /var/run/netns/ns-OOB-Management nsfs rw 0 0
nsfs /.deltas/var/run/netns/ns-OOB-Management nsfs rw 0 0
`,
			expected: "/var/run/netns",
		},
		{
			desc: "Mounted in /run/netns",
			input: `
none / aufs rw,relatime,si=7aaed56e5ecd215c 0 0
none /.overlay tmpfs rw,relatime,size=593256k,mode=755,idr=enabled 0 0
proc /proc proc rw,nosuid,nodev,noexec,relatime 0 0
sysfs /sys sysfs rw,nosuid,nodev,noexec,relatime 0 0
devtmpfs /dev devtmpfs rw,size=8192k,nr_inodes=485215,mode=755 0 0
securityfs /sys/kernel/security securityfs rw,nosuid,nodev,noexec,relatime 0 0
tmpfs /dev/shm tmpfs rw,nosuid,nodev 0 0
devpts /dev/pts devpts rw,nosuid,noexec,relatime,gid=5,mode=620,ptmxmode=000 0 0
tmpfs /run tmpfs rw,nosuid,nodev,mode=755 0 0
tmpfs /sys/fs/cgroup tmpfs rw,nosuid,nodev,noexec,mode=755 0 0
cgroup /sys/fs/cgroup/systemd cgroup rw,nosuid,nodev,noexec,relatime,name=systemd 0 0
cgroup /sys/fs/cgroup/cpuset cgroup rw,nosuid,nodev,noexec,relatime,cpuset 0 0
cgroup /sys/fs/cgroup/cpu,cpuacct cgroup rw,nosuid,nodev,noexec,relatime,cpu,cpuacct 0 0
cgroup /sys/fs/cgroup/blkio cgroup rw,nosuid,nodev,noexec,relatime,blkio 0 0
cgroup /sys/fs/cgroup/memory cgroup rw,nosuid,nodev,noexec,relatime,memory 0 0
cgroup /sys/fs/cgroup/devices cgroup rw,nosuid,nodev,noexec,relatime,devices 0 0
cgroup /sys/fs/cgroup/freezer cgroup rw,nosuid,nodev,noexec,relatime,freezer 0 0
cgroup /sys/fs/cgroup/net_cls cgroup rw,nosuid,nodev,noexec,relatime,net_cls 0 0
configfs /sys/kernel/config configfs rw,relatime 0 0
debugfs /sys/kernel/debug debugfs rw,relatime 0 0
tmpfs /tmp tmpfs rw,size=593256k 0 0
hugetlbfs /dev/hugepages hugetlbfs rw,relatime 0 0
mqueue /dev/mqueue mqueue rw,relatime 0 0
tmpfs /.deltas tmpfs rw,relatime,size=65536k 0 0
tmpfs /var/run tmpfs rw,relatime,size=65536k 0 0
tmpfs /run/netns tmpfs rw,relatime 0 0
tmpfs /.deltas/run/netns tmpfs rw,relatime 0 0
tmpfs /var/tmp tmpfs rw,relatime,size=65536k 0 0
tmpfs /var/core tmpfs rw,relatime,size=395504k 0 0
tmpfs /var/log tmpfs rw,relatime,size=395504k 0 0
tmpfs /var/shmem tmpfs rw,relatime,size=988756k 0 0
/monitor /monitor debugfs rw,relatime 0 0
/dev/sda1 /mnt/flash vfat rw,dirsync,noatime,gid=88,fmask=0007,dmask=0007,allow_utime=0020 0 0
nsfs /run/netns/default nsfs rw 0 0
nsfs /.deltas/run/netns/default nsfs rw 0 0
nsfs /run/netns/ns-OOB-Management nsfs rw 0 0
nsfs /.deltas/run/netns/ns-OOB-Management nsfs rw 0 0
`,
			expected: "/run/netns",
		},
		{
			desc: "Not mounted",
			input: `
none / aufs rw,relatime,si=7aaed56e5ecd215c 0 0
none /.overlay tmpfs rw,relatime,size=593256k,mode=755,idr=enabled 0 0
proc /proc proc rw,nosuid,nodev,noexec,relatime 0 0
sysfs /sys sysfs rw,nosuid,nodev,noexec,relatime 0 0
devtmpfs /dev devtmpfs rw,size=8192k,nr_inodes=485215,mode=755 0 0
securityfs /sys/kernel/security securityfs rw,nosuid,nodev,noexec,relatime 0 0
tmpfs /dev/shm tmpfs rw,nosuid,nodev 0 0
devpts /dev/pts devpts rw,nosuid,noexec,relatime,gid=5,mode=620,ptmxmode=000 0 0
tmpfs /run tmpfs rw,nosuid,nodev,mode=755 0 0
tmpfs /sys/fs/cgroup tmpfs rw,nosuid,nodev,noexec,mode=755 0 0
cgroup /sys/fs/cgroup/systemd cgroup rw,nosuid,nodev,noexec,relatime,name=systemd 0 0
cgroup /sys/fs/cgroup/cpuset cgroup rw,nosuid,nodev,noexec,relatime,cpuset 0 0
cgroup /sys/fs/cgroup/cpu,cpuacct cgroup rw,nosuid,nodev,noexec,relatime,cpu,cpuacct 0 0
cgroup /sys/fs/cgroup/blkio cgroup rw,nosuid,nodev,noexec,relatime,blkio 0 0
cgroup /sys/fs/cgroup/memory cgroup rw,nosuid,nodev,noexec,relatime,memory 0 0
cgroup /sys/fs/cgroup/devices cgroup rw,nosuid,nodev,noexec,relatime,devices 0 0
cgroup /sys/fs/cgroup/freezer cgroup rw,nosuid,nodev,noexec,relatime,freezer 0 0
cgroup /sys/fs/cgroup/net_cls cgroup rw,nosuid,nodev,noexec,relatime,net_cls 0 0
configfs /sys/kernel/config configfs rw,relatime 0 0
debugfs /sys/kernel/debug debugfs rw,relatime 0 0
tmpfs /tmp tmpfs rw,size=593256k 0 0
hugetlbfs /dev/hugepages hugetlbfs rw,relatime 0 0
mqueue /dev/mqueue mqueue rw,relatime 0 0
tmpfs /.deltas tmpfs rw,relatime,size=65536k 0 0
tmpfs /var/run tmpfs rw,relatime,size=65536k 0 0
tmpfs /.deltas/run/netns tmpfs rw,relatime 0 0
tmpfs /var/tmp tmpfs rw,relatime,size=65536k 0 0
tmpfs /var/core tmpfs rw,relatime,size=395504k 0 0
tmpfs /var/log tmpfs rw,relatime,size=395504k 0 0
tmpfs /var/shmem tmpfs rw,relatime,size=988756k 0 0
/monitor /monitor debugfs rw,relatime 0 0
/dev/sda1 /mnt/flash vfat rw,dirsync,noatime,gid=88,fmask=0007,dmask=0007,allow_utime=0020 0 0
`,
			err: "can't find the netns mount",
		},
	}

	for _, tc := range testCases {
		r, err := getNsDirFromProcMounts(strings.NewReader(tc.input))
		if err != nil {
			if tc.err == "" || !strings.Contains(err.Error(), tc.err) {
				t.Errorf("%v: unexpected error %v", tc.desc, err)
				continue
			}
		}
		if r != tc.expected {
			t.Errorf("%v: expected %v, got %v", tc.desc, tc.expected, r)
		}
	}
}
