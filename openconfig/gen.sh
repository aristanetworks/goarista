#!/bin/sh
# Copyright (C) 2016  Arista Networks, Inc.
# Use of this source code is governed by the Apache License 2.0
# that can be found in the COPYING file.

# Fix up protobuf imports

go get -u github.com/golang/protobuf/protoc-gen-go
protoc --go_out=plugins=grpc,Mgoogle/protobuf/any.proto=github.com/golang/protobuf/ptypes/any:. openconfig.proto

if ! egrep "Copyright \d+ Google" openconfig.pb.go
then
   egrep -A 12 "Copyright \d+ Google" openconfig.proto | cat - openconfig.pb.go > /tmp/pb.go && mv /tmp/pb.go openconfig.pb.go
fi
