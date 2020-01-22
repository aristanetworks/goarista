# Copyright (c) 2016 Arista Networks, Inc.
# Use of this source code is governed by the Apache License 2.0
# that can be found in the COPYING file.

FROM golang:1.13.6

RUN mkdir -p /go/src/github.com/aristanetworks/goarista/cmd
WORKDIR /go/src/github.com/aristanetworks/goarista
COPY ./ .
RUN go get -d ./cmd/ockafka/... \
  && go install ./cmd/ockafka

ENTRYPOINT ["/go/bin/ockafka"]
