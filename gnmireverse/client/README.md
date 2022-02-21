# gNMIReverse client

The gNMIReverse client is a process that issues gNMI RPCs to a gNMI target
(typically running on the same host as this process) and forwards the
responses to a gNMIReverse server. This can be used to reverse the
dial direction of a gNMI Subscribe or gNMI Get.


## Installation

* Build the gNMIReverse client and copy the executable to the target device.

Compiling the client requires Go 1.16 or later. Instructions for installing Go can be
found [here](https://go.dev/doc/install). Once Go is installed you can run:

```
GOOS=linux go install github.com/aristanetworks/goarista/gnmireverse/client@latest
```

This will install the gNMIReverse `client` binary in the `$HOME/go/bin` or
`$HOME/go/bin/linux_amd64` directory by default. Run `go help install` for more information.


## Options

Run the program with the flag `--help` or `-h` to see the full list of options and documentation.

 Option                    | Description
:--------------------------|:-------------------------------------------------------------------------
`username`                 | Username to authenticate with the target (gNMI server).
`password`                 | Password to authenticate with the target (gNMI server).
`target_addr`              | Address of the gNMI server running on the device.<br/>- Form: `[vrf/]address:port`<br/>- Example: `default/127.0.0.1:6030`, `mgmt/localhost:9339`
`target_value`             | Target name to include in the prefix of all responses to identify the device.
`target_tls_insecure`      | Use TLS connection with the target and do not verify the target certificate. Used if the gNMI server is configured with a TLS certificate and mutual TLS authentication is not enforced.<br/>By default, a plaintext connection is used with the target.
`collector_addr`           | Address of the gNMIReverse server collecting the data.<br/>- Form: `[vrf/]host:port`<br/>- Example: `1.2.3.4:6000`, `mgmt/collector1:10000`
`source_addr`              | Address to use as source in connection to the collector. An IPv6 address must be enclosed in square brackets when specified with a port.<br/>- Form: `ip[:port]` or `:port`<br/>- Example: `10.2.3.4`, `[::1]:1234`, `:1234`
`collector_tls`            | Use TLS connection with the gNMIReverse server.<br/>- Default: `true`
`collector_tls_skipverify` | Do not verify the collector TLS certificate. Used if mutual TLS authentication is not enforced.
`collector_compression`    | Compression method used when streaming to the gNMIReverse server.<br/>- Default: `none`<br/>- Options: `gzip`
`origin`                   | Path origin. Applies to all specified Subscribe/Get paths.
`subscribe`                | Path to subscribe to with `TARGET_DEFINED` mode with an optional heartbeat interval.<br/>Can be repeated multiple times to specify multiple paths.<br/>- Form: `path[@heatbeat_interval]`<br/>- Example: `/system/processes`,`/components/component/state@1m`
`sample`                   | Path to subscribe to with `SAMPLE` mode.<br/>Can be repeated multiple times to specify multiple paths.<br/>- Form: `path@sample_interval`<br/>- Example: `/interfaces/interface/state/counters@30s`
`get`                      | Path to retrieve using a periodic gNMI Get.<br/>Can be repeated multiple times to specify multiple paths.<br/>Arista EOS native origin paths can be specified with the prefix `eos_native:`. This allows for specifying both OpenConfig and EOS native origin paths.<br/>- Example: `/system/memory`, `eos_native:/Sysdb/hardware`
`get_file`                 | File containing a list of paths separated by newlines to retrieve periodically using Get.
`get_sample_interval`      | Interval between periodic Get requests.<br/>- Example: `400ms`, `2.5s`, `1m`
`v`                        | Log level verbosity. Enables gRPC logging.


## gNMI Subscribe dial-out

The client requires a gNMI target address to connect to and a
gNMIReverse server to send the subscription results.

For example, on an Arista EOS device the client can be configured in the CLI:

```
daemon gnmireverse
   exec /mnt/flash/gnmireverse_client
      -username USER
      -password PASS
      -target_addr=mgmt/127.0.0.1:6030
      -collector_addr=mgmt/1.2.3.4:6000
      -target_value=device1
      -sample interfaces/interface/state/counters@30s
      -subscribe network-instances
   no shutdown
```

* The username and password authenticates with the gNMI server.
* The client is connecting to the gNMI server locally on `127.0.0.1:6030` in the `mgmt` VRF.
* The client is connecting through the `mgmt` VRF to the gNMIReverse server listening on `1.2.3.4:6000`.
* Interface counters sampled every 30 seconds are streamed to the collector.
* Changes as they happen to network-instances config and state are streamed to the collector.


## gNMI Get dial-out

An example CLI configuration on an Arista EOS device to stream responses using gNMI Get:

```
daemon gnmireverse
   exec /mnt/flash/gnmireverse_client
      -username admin
      -password pass
      -target_addr=mgmt/127.0.0.1:6030
      -collector_addr=mgmt/1.2.3.4:6000
      -target_value=device1
      -get_sample_interval 10s
      -get /interfaces/interface/state/counters
      -get /system/memory
      -get eos_native:/Sysdb/hardware
   no shutdown
```

* Every 10 seconds, a gNMI Get is issued to the target to retrieve interface counters and memory state from the OpenConfig paths and Sysdb hardware state from the Arista EOS native path.
* The `GetResponse` from the target is forwarded to the collector via a stream.
* The Get paths can also be specified using a `-get_file` containing:
```
/interfaces/interface/state/counters
/system/memory
eos_native:/Sysdb/hardware
```


## Collector

A collector implementing a gNMIReverse server can be installed with:

```
go install github.com/aristanetworks/goarista/gnmireverse/server@latest
```
Run the program with the flag `--help` or `-h` to see the full list of options.


## gRPC compression

By default, the gNMIReverse client sends uncompressed gRPC messages to the gNMIReverse server.
For Get RPCs, the message size is typically larger than individual Subscribe messages so usually,
the message is more compressible. As a result, it may be preferable to enable gzip gRPC compression
with `-collector_compression gzip` to lower bandwidth. Ensure that the gNMIReverse server supports
the gzip gRPC compression method. Note that this may cause an increase in CPU load on the target
device due to compression overhead.


## Debugging

Use `-v 1` or above to enable gRPC logging. This is useful for checking that a connection has
been established with the target and collector.
