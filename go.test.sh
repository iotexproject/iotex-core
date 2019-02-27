#!/usr/bin/env bash

set -e
echo "" > coverage.txt

for d in $(go list ./... | grep -v 'vender\|accountpb\|util\|subchainpb\|rewardingpb\|candidatesutil\|chainservice\|cli\|consensus\|triepb\|idl\|p2p\|log\|unit\|protogen\|server\|identityset\|mock\|tools'); do
    go test -short -v -coverprofile=profile.out -covermode=count "$d" 
    if [ -f profile.out ]; then
        cat profile.out 
        cat profile.out | go2xunit >> /tmp/test_report_upload/coverage_`basename "$d"`_`date +"%H_%M_%S"`.xml
        rm profile.out
    fi
done
