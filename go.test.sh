#!/usr/bin/env bash
# Run the short test suite for every package, letting go test build and run
# packages in parallel. -gcflags=all=-l disables inlining, which the
# gomonkey-based tests need in order to patch functions.

set -e
go test -short -gcflags=all=-l ./...
