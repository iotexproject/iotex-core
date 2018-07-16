#!/usr/bin/env bash

export LD_LIBRARY_PATH=$LD_LIBRARY_PATH:$GOPATH/src/github.com/iotexproject/iotex-core/crypto/lib
pwd
echo $GOPATH
echo $LD_LIBRARY_PATH

set -e
echo "" > coverage.txt

for d in $(go list ./... | grep -v vendor); do
    go test -short -coverprofile=profile.out -covermode=count "$d"
    if [ -f profile.out ]; then
        cat profile.out >> coverage.txt
        rm profile.out
    fi
done
