#!/usr/bin/env bash

set -e
cat /dev/null > coverage.txt

for d in $(go list ./... | grep -v vendor); do
    go test -short -v -coverprofile=coverage.out -covermode=count "$d"
    courtney -l=coverage.out -o=profile.out
    if [ -f profile.out ]; then
        cat profile.out >> coverage.txt
        rm profile.out coverage.out
    fi
done
