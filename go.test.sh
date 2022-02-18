#!/usr/bin/env bash

set -e

for d in $(go list ./... | grep -v vendor); do
    courtney -short -v -o=coverage.txt "$d"
done
# for d in $(go list ./... | grep -v vendor); do
#     go test -short -v -coverprofile=profile.out -covermode=count "$d"
#     if [ -f profile.out ]; then
#         cat profile.out >> coverage.txt
#         rm profile.out
#     fi
# done
