# Copyright (C) 2016  Arista Networks, Inc.
# Use of this source code is governed by the Apache License 2.0
# that can be found in the COPYING file.

FROM golang:1.6

RUN mkdir -p /go/src/github.com/aristanetworks/goarista/cmd
WORKDIR /go/src/github.com/aristanetworks/goarista
COPY ./ .
RUN go get -d ./cmd/occlient/... \
  && go install ./cmd/occlient

ENTRYPOINT ["/go/bin/occlient"]
