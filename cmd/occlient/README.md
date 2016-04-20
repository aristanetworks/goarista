# occlient

Client for the gRPC OpenConfig service for subscribing to, getting and setting the
configuration and state of a network device.

## Sample usage

Subscribe to all updates on the Arista device at `10.0.1.2` and dump results as JSON to stdout:

```
occlient -addrs 10.0.1.2 -json
```

Subscribe to temperature sensors from 2 switches and stream to Kafka:

```
occlient -addrs 10.0.1.2,10.0.1.3 -kafkaaddrs kafka:9092 -subscribe /Sysdb/environment/temperature/status/tempSensor
```

Start in a container:
```
docker run aristanetworks/occlient -addrs 10.0.1.1
```

## ELK integration demo
The following video demoes integration with ELK using [this](https://github.com/aristanetworks/docker-logstash) Logstash instance:

[![video preview](http://img.youtube.com/vi/WsyFmxMwXYQ/0.jpg)](https://youtu.be/WsyFmxMwXYQ)
