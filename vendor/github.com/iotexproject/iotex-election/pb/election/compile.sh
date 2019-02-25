#!/usr/bin/env sh

# Install proto3 from homebrew
#  brew install protobuf

# Update protoc Go bindings via
#  go get -u github.com/golang/protobuf/{proto,protoc-gen-go}
#
# See also
#  https://github.com/grpc/grpc-go/tree/master/examples

protoc ./election.proto --go_out=plugins=grpc:.
