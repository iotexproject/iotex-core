#!/usr/bin/env bash

set -e
cat /dev/null > coverage.txt

for dir in $(go list -f '{{.ImportPath}}={{join .TestGoFiles ","}}' ./...);do
    array=(${dir//=/ })
    if [ -n "${array[1]}" ]; then
        go test -short -v -coverprofile=coverage.out -covermode=count "${array[0]}"
        courtney -l=coverage.out -o=profile.out
        if [ -f profile.out ]; then
            cat profile.out >> coverage.txt
            rm profile.out coverage.out
        fi
    fi    
done
