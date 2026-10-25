#!/usr/bin/env bash
# Run the short test suite for every package, letting go test build and run
# packages in parallel. -gcflags=all=-l disables inlining, which the
# gomonkey-based tests need in order to patch functions.
#
# Packages whose tests bind fixed ports (e.g. the probe server on 7788) collide
# when run at the same time, so they run afterwards, one package at a time.

set -e
serial='/e2etest$|/pkg/probe$|/server/itx$'
pkgs=$(go list ./...)
go test -short -gcflags=all=-l $(grep -Ev "$serial" <<< "$pkgs")
go test -short -gcflags=all=-l -p 1 $(grep -E "$serial" <<< "$pkgs")
