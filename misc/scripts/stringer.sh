#!/bin/bash -ex
go get golang.org/x/tools/cmd/stringer
export PATH=$PATH:"$GOPATH/bin"
find . -name '*_string.go' -type f -not -path "./vendor/*" -not -path "$GOPATH/vendor/*" -delete
go generate `glide nv | grep -v go-build`
