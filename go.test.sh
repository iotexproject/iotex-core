#!/usr/bin/env bash

set -e
echo "" > coverage.txt

for d in $(go list ./... | grep -v vendor); do
    go test -short -v -coverprofile=profile.out -covermode=count "$d" 
    if [ -f profile.out ]; then
        cat profile.out >> coverage.txt
        cat profile.out | go2xunit >> /tmp/test_report_upload/coverage_"$d".xml
        rm profile.out
    fi
done
