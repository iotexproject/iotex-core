#!/usr/bin/env bash

set -e
echo "" > coverage.txt
go test -v $(go list ./... |grep -v vendor | circleci tests split)
# for d in $(go list ./... | grep -v vendor); do
#     go test -short -v -coverprofile=profile.out -covermode=count "$d"
#     if [ -f profile.out ]; then
#         cat profile.out >> coverage.txt
#         rm profile.out
#     fi
# done
