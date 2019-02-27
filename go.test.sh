#!/usr/bin/env bash

set -e
echo "" > coverage.txt

for d in $(go list ./... | grep -v 'vender\|accountpb\|util\|subchainpb\|rewardingpb\|candidatesutil\|chainservice\|cli\|consensus\|triepb\|idl\|p2p\|log\|unit\|protogen\|server\|identityset\|mock\|tools'); do
    go test -short -v -coverprofile=profile.out -covermode=count "$d" 
    if [ -f profile.out ]; then
        cat profile.out >> coverage.txt
        rm profile.out
    fi
done
