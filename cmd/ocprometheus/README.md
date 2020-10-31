# ocprometheus

This is a client for the OpenConfig gRPC interface that pushes telemetry to
Prometheus. Numerical and boolean (converted to 1 for true and 0 for false) are
supported. Non-numerical data isn't supported by Prometheus and is silently
dropped. Arrays (even with numeric values) are not yet supported.

This tool requires a config file to specify how to map the path of the
notificatons coming out of the OpenConfig gRPC interface onto Prometheus
metric names, and how to extract labels from the path.  For example, the
following rule, excerpt from `sampleconfig.yml`:

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

## Usage

See the `-help` output, but here's an example to push all the metrics defined
in the sample config file:
```
ocprometheus -addr <switch-hostname>:6042 -config sampleconfig.json
```

## Deployment

This can be packaged as a swix package to ease the deployment on the devices.
For this a sample ocprometheus.spec file is proposed.

Here are some instructions, that were tested on OpenSUSE, which you should
adapt for your environment and the version you wish to use.

Build the rpm:

```
rpmbuild --undefine=_disable_source_fetch -ba ocprometheus.spec
rpmbuild --target i686 -ba ocprometheus.spec
```

Package it as a swix package using a cEOS-lab Docker image:

```
docker run --rm -ti -v $HOME/rpmbuild:/vol --entrypoint=swix ceosimage:4.24.3 \
  create /vol/ocprometheus-0.0.2-1.i686.swix /vol/RPMS/i686/ocprometheus-0.0.2-1.i686.rpm --force
```

You can now install the swix as a normal extension.


