#!/bin/sh

set -e 

go clean
go test -v
go run idl2go/idl2go.go -n -b "github.com/coopernurse/barrister-go/conform/generated/" -d conform/generated conform/conform.json
go build conform/client.go
go build conform/server.go
