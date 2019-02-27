#!/usr/bin/env bash

set -e
echo "" > coverage.txt

for d in $(go list ./... | grep -v vendor); do
    go test -short -v -coverprofile=profile.out -covermode=count "$d" | go2xunit >> /tmp/test_report_upload/coverage.xml
    if [ -f profile.out ]; then
        cat profile.out >> coverage.txt
        rm profile.out
    fi
done
