# OpenConfig clients

The following commands are clients for the [OpenConfig](http://openconfig.net) gRPC interface.

## occli

Prints the response protobufs in text form or JSON.

## ockafka

Publishes updates to [Kafka](http://kafka.apache.org) in [Elasticsearch](https://www.elastic.co/products/elasticsearch)-friendly form.

## ocredis

Publishes updates to [Redis](http://redis.io) using both [Redis' hashes](http://redis.io/topics/data-types-intro#hashes)
(one per container / entity / collection) and [Redis' Pub/Sub](http://redis.io/topics/pubsub) 
mechanism, so that one can [subscribe](http://redis.io/commands/subscribe) to
incoming updates being applied on the hash maps.

## octsdb

Publishes updates to [OpenTSDB](http://opentsdb.net).


