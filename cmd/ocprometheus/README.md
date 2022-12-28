# ocprometheus

This is a client for the OpenConfig gRPC interface that pushes telemetry to
Prometheus. Numerical and boolean (converted to 1 for true and 0 for false) are
supported. Non-numerical data isn't supported by Prometheus and is silently
dropped. Arrays (even with numeric values) are not yet supported.

This tool requires a config file to specify how to map the path of the
notificatons coming out of the OpenConfig gRPC interface onto Prometheus
metric names, and how to extract labels from the path.  For example, the
following rule, excerpt from `sampleconfig_above_4.20.yml`:

```yaml
metrics:
        - name: tempSensor
          path: /Sysdb/environment/temperature/status/tempSensor/(?P<sensor>.+)/(?P<type>(?:maxT|t)emperature)/value
          help: Temperature and Maximum Temperature
          # ...
```

Applied to an update for the path
`/Sysdb/environment/temperature/status/tempSensor/TempSensor1/temperature/value`
will lead to the metric name `tempSensor` and labels `sensor=TempSensor1` and `type=temperature`.

Basically, named groups are used to extract (optional) metrics.
Unnamed groups will be given labels names like "unnamedLabelX" (where X is the group's position).
The timestamps from the notifications are not preserved since Prometheus uses a pull model and
doesn't have (yet) support for exporter specified timestamps.
Prometheus 2.0 will probably support timestamps.

Support for `eos_native` origin when using ocprometheus with the Octa agent (enabled with `provider eos-native` under `management api gnmi`) was added as part of #c6473e3ed183a4706d17336671d4e5be1991b7df

The [sample_configs](./sample_configs) folder contains per platform examples using EOS native paths and also examples for OpenConfig paths.

## Usage

See the `-help` output, but here's an example to push all the metrics defined
in the sample config file:
```
ocprometheus -addr <switch-hostname>:6042 -config sampleconfig.yml
```

For more usage examples and a detailed demo please visit:
https://eos.arista.com/streaming-eos-telemetry-states-to-prometheus/

### Dynamic label extraction

This feature can be enabled by passing the `-enable-description-labels` flag. Paths where labels can be extracted from are defined in the configuration file, e.g.

```yaml
description-label-subscriptions:
        - /interfaces/interface/state/description
        - /network-instances/network-instance/protocols/protocol/static-routes/static/state/description
```

These paths are used to dynamically retrieve labels from other nodes. The default regex used is `\[([^=\]]+)(=[^]]+)?]`.
It finds the nearest list node and adds these labels to any nodes under that list node.

For example, with the example subscriptions above, if we got the following description `[baz][foo=bar]` on the path
`interfaces/interface[name=Ethernet4]/state/description`, it will add the labels `baz=1` and `foo=bar` to all metrics
under `interfaces/interface[name=Ethernet4]`, e.g. interfaces/interface[name=Ethernet4]/state/counter/in-pkts will have
these labels attached to it. The description regex can be changed by passing the `-description-regex <regex>` flag.

These labels will update dynamically as the descriptions change on the box.