# ockafka

Client for the gRPC OpenConfig service for subscribing to the configuration and
state of a network device and feeding the stream to Kafka.

## Sample usage

Subscribe to all updates on the Arista device at `10.0.1.2` and stream to a local
Kafka instance:

```
ockafka -addr 10.0.1.2
```

Subscribe to temperature sensors and stream to a remote Kafka instance:

```
ockafka -addr 10.0.1.2 -kafkaaddrs kafka:9092 -subscribe /Sysdb/environment/temperature/status/tempSensor
```

Start in a container:
```
docker run aristanetworks/ockafka -addr 10.0.1.2 -kafkaaddrs kafka:9092
```

## Kafka/Elastic integration demo
The following video demoes integration with Kafka and Elastic using [this Logstash instance](https://github.com/aristanetworks/docker-logstash):

[![video preview](http://img.youtube.com/vi/WsyFmxMwXYQ/0.jpg)](https://youtu.be/WsyFmxMwXYQ)
