# gnmi

`gnmi` is a command-line client for interacting with a
[gNMI service](https://github.com/openconfig/reference/tree/master/rpc/gnmi).

# Installation

Compiling `gnmi` requires Go 1.16 or later. Instructions for
installing Go can be found [here](https://go.dev/doc/install). Once Go
is installed you can run:

```
go install github.com/aristanetworks/goarista/cmd/gnmi@latest
```

This will install the `gnmi` binary in the `$HOME/go/bin` directory by
default. Run `go help install` for more information.

# Usage

```
$ gnmi [OPTIONS] [OPERATION]
```

When running on the switch in a non-default VRF:

```
$ ip netns exec ns-<VRF> gnmi [OPTIONS] [OPERATION]
```

## Options

* `-addr [<VRF-NAME>/]ADDR:PORT`  
Address of the gNMI endpoint (REQUIRED) with VRF name (OPTIONAL)
* `-username USERNAME`  
Username to authenticate with
* `-password PASSWORD`  
Password to authenticate with
* `-tls`  
Enable TLS
* `-cafile PATH`  
Path to server TLS certificate file
* `-certfile PATH`  
Path to client TLS certificate file
* `-keyfile PATH`  
Path to client TLS private key file

## Operations

`gnmi` supports the following operations: `capabilites`, `get`,
`subscribe`, `set`, `update`, `replace`, `delete`, and `union_replace`.

### capabilities

`capabilities` prints the result of calling the
[Capabilities gNMI RPC](https://github.com/openconfig/reference/blob/master/rpc/gnmi/gnmi-specification.md#32-capability-discovery).

Example:

```
$ gnmi [OPTIONS] capabilities
```

### get

`get` requires a path and calls the
[Get gNMI RPC](https://github.com/openconfig/reference/blob/master/rpc/gnmi/gnmi-specification.md#222-paths).

Example:

Get all configuration in the default network instance:
```
$ gnmi [OPTIONS] get '/network-instances/network-instance[name=default]'
```

### subscribe

`subscribe` requires a path and calls the
[Subscribe gNMI RPC](https://github.com/openconfig/reference/blob/master/rpc/gnmi/gnmi-specification.md#35-subscribing-to-telemetry-updates).
This command will continuously print out results until signalled to
exit, for example by typing `Ctrl-C`.

Example:

**Subscribe to interface counters**
```
$ gnmi [OPTIONS] subscribe '/interfaces/interface[name=*]/state/counters'
```

**Subscribe with proto**

The `-proto` option parses the `subscribe` argument as the Protocol Buffer Text Format of a
[SubscribeRequest](https://github.com/openconfig/reference/blob/master/rpc/gnmi/gnmi-specification.md#3511-the-subscriberequest-message).  
The argument is either the path to the proto file or the proto text itself.

**Proto file**

File `path/to/request.proto` contains the following:

```
subscribe: {
  mode: STREAM
  subscription: {
    mode: ON_CHANGE
    path: {
      elem: {
        name: "interfaces"
      }
      elem: {
        name: "interface"
        key: {
          key: "name"
          value: "Management1"
        }
      }
      elem: {
        name: "state"
      }
      elem: {
        name: "counters"
      }
    }
  }
  subscription: {
    mode: SAMPLE
    sample_interval: 2000000000
    path: {
      elem: {
        name: "system"
      }
      elem: {
        name: "state"
      }
      elem: {
        name: "hostname"
      }
    }
  }
}
```

```
$ gnmi [OPTIONS] -proto subscribe path/to/request.proto
```

**Proto text**

```
$ gnmi [OPTIONS] -proto subscribe 'subscribe:{subscription:{mode:SAMPLE sample_interval:2000000000 path:{elem:{name:"system"}elem:{name:"state"}elem:{name:"hostname"}}}}'
```

### set

`set` takes a single argument, the Protocol Buffer Text Format of a
[SetRequest](https://github.com/openconfig/reference/blob/master/rpc/gnmi/gnmi-specification.md#341-the-setrequest-message),
and calls the [Set gNMI RPC](https://github.com/openconfig/reference/blob/master/rpc/gnmi/gnmi-specification.md#34-modifying-state).  
The argument is either the path to the proto file or the proto text itself.

Example:

**Proto file**

File `path/to/config.proto` contains the following:

```
replace {
  path {
    elem {
      name: "system"
    }
    elem {
      name: "config"
    }
    elem {
      name: "hostname"
    }
  }
  val {
    string_val: "foo"
  }
}
```

```
$ gnmi [OPTIONS] set path/to/config.proto
```

**Proto text**

```
$ gnmi [OPTIONS] set 'replace{path{elem{name:"system"}elem{name:"config"}elem{name:"hostname"}}val{string_val:"foo"}}'
```

### update/replace/delete/union_replace

`update`, `replace`, `delete`, and `union_replace` are used to
[modify the configuration of a gNMI endpoint](https://github.com/openconfig/reference/blob/master/rpc/gnmi/gnmi-specification.md#34-modifying-state).
All of these operations take a path that must specify a single node
element. In other words all list members must be fully-specified.

`delete` takes a path and will delete that path.

Example:

Delete BGP configuration in the default network instance:
```
$ gnmi [OPTIONS] delete '/network-instances/network-instance[name=default]/protocols/protocol[name=BGP][identifier=BGP]/'
```

`update`, `replace`, and `union_replace` take a path and a value in JSON
format. The JSON data may be provided in a file. See
[here](https://github.com/openconfig/reference/blob/master/rpc/gnmi/gnmi-specification.md#344-modes-of-update-union-replace-replace-and-update)
for documentation on the differences between `update`, `replace`, and `union_replace`.

Examples:

Disable interface Ethernet3/42:
```
gnmi [OPTIONS] update '/interfaces/interface[name=Ethernet3/42]/config/enabled' 'false'
```

Replace the BGP global configuration:
```
gnmi [OPTIONS] replace '/network-instances/network-instance[name=default]/protocols/protocol[name=BGP][identifier=BGP]/bgp/global' '{"config":{"as": 1234, "router-id": "1.2.3.4"}}'
```

Note: String values need to be quoted if they look like JSON. For example, setting the login banner to `tor[13]`:
```
gnmi [OPTIONS] update '/system/config/login-banner '"tor[13]"'
```

#### JSON in a file

The value argument to `update`, `replace`, and `union_replace` may be a file. The
content of the file is used to make the request.

Example:

File `path/to/subintf100.json` contains the following:

```
{
  "subinterface": [
    {
      "config": {
        "enabled": true,
        "index": 100
      },
      "index": 100
    }
  ]
}
```

Add subinterface 100 to interfaces Ethernet4/1/1 and Ethernet4/2/1 in
one transaction:

```
gnmi [OPTIONS] update '/interfaces/interface[name=Ethernet4/1/1]/subinterfaces' path/to/subintf100.json \
               update '/interfaces/interface[name=Ethernet4/2/1]/subinterfaces' path/to/subintf100.json
```

### CLI requests
`gnmi` offers the ability to send CLI text inside an `update`, `replace`, or
`union_replace` operation. This is achieved by doing an `update`, `replace`, or
`union_replace` and specifying `"origin=cli"` along with an empty path and a set of configure-mode
CLI commands separated by `\n`.

Example:

Configure the idle-timeout on SSH connections
```
gnmi [OPTIONS] update 'origin=cli' "" 'management ssh
idle-timeout 300'
```

### P4 Config
`gnmi` offers the ability to send p4 config files inside a `replace` operation.
This is achieved by doing a `replace` and specifying `"origin=p4_config"`
along with the path of the p4 config file to send.

Example:

Send the config.p4 file
```
gnmi [OPTIONS] replace 'origin=p4_config' 'config.p4'
```

## Paths

Paths in `gnmi` use a simplified xpath style. Path elements are
separated by `/`. Selectors may be used on list to select certain
members. Selectors are of the form `[key-leaf=value]`. All members of a
list may be selected by not specifying any selectors, or by using a
`*` as the value in a selector. The following are equivalent:

* `/interfaces/interface`
* `/interfaces/interface[name=*]`

All characters, including `/` are allowed inside a selector value. The
character `]` must be escaped, for example `[key=[\]]` selects the
element in the list whose `key` leaf is value `[]`.

See more examples of paths in the examples above.

See
[here](https://github.com/openconfig/reference/blob/master/rpc/gnmi/gnmi-specification.md#222-paths)
for more information.
