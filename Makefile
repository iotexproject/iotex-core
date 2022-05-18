########################################################################################################################
# Copyright (c) 2019 IoTeX Foundation
# This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
# warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
# permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
# License 2.0 that can be found in the LICENSE file.
########################################################################################################################

# Go parameters
GOCMD=go
GOLINT=golint
GOBUILD=$(GOCMD) build
GOINSTALL=$(GOCMD) install
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
GOPATH=$(shell go env GOPATH)
BUILD_TARGET_SERVER=server
BUILD_TARGET_ACTINJV2=actioninjectorv2
BUILD_TARGET_ADDRGEN=addrgen
BUILD_TARGET_IOCTL=ioctl
BUILD_TARGET_XCTL=xctl
BUILD_TARGET_MINICLUSTER=minicluster
BUILD_TARGET_RECOVER=recover
BUILD_TARGET_IOMIGRATER=iomigrater
BUILD_TARGET_OS=$(shell go env GOOS)
BUILD_TARGET_ARCH=$(shell go env GOARCH)

# Pkgs
ALL_PKGS := $(shell go list ./... )
PKGS := $(shell go list ./... | grep -v /test/ )
ROOT_PKG := "github.com/iotexproject/iotex-core"

# Docker parameters
DOCKERCMD=docker

# Package Info
PACKAGE_VERSION := $(shell git describe --tags)
PACKAGE_COMMIT_ID := $(shell git rev-parse HEAD)
GIT_STATUS := $(shell git status --porcelain)
ifdef GIT_STATUS
	GIT_STATUS := "dirty"
else
	GIT_STATUS := "clean"
endif
GO_VERSION := $(shell go version)
BUILD_TIME=$(shell date +%F-%Z/%T)
VersionImportPath := github.com/iotexproject/iotex-core/pkg/version
PackageFlags += -X '$(VersionImportPath).PackageVersion=$(PACKAGE_VERSION)'
PackageFlags += -X '$(VersionImportPath).PackageCommitID=$(PACKAGE_COMMIT_ID)'
PackageFlags += -X '$(VersionImportPath).GitStatus=$(GIT_STATUS)'
PackageFlags += -X '$(VersionImportPath).GoVersion=$(GO_VERSION)'
PackageFlags += -X '$(VersionImportPath).BuildTime=$(BUILD_TIME)'
PackageFlags += -s -w

TEST_IGNORE= ".git,vendor"
COV_OUT := profile.coverprofile
COV_REPORT := overalls.coverprofile
COV_HTML := coverage.html

LINT_LOG := lint.log

V ?= 0
ifeq ($(V),0)
	ECHO_V = @
else
	VERBOSITY_FLAG = -v
	DEBUG_FLAG = -debug
endif

default: clean build test
all: clean build-all test

.PHONY: build
build: ioctl
	$(GOBUILD) -ldflags "$(PackageFlags)" -o ./bin/$(BUILD_TARGET_SERVER) -v ./$(BUILD_TARGET_SERVER)

.PHONY: build-all
build-all: build build-actioninjector build-addrgen build-minicluster build-staterecoverer

.PHONY: build-actioninjector
build-actioninjector: 
	$(GOBUILD) -o ./bin/$(BUILD_TARGET_ACTINJV2) -v ./tools/actioninjector.v2

.PHONY: build-addrgen
build-addrgen:
	$(GOBUILD) -o ./bin/$(BUILD_TARGET_ADDRGEN) -v ./tools/addrgen

.PHONY: build-minicluster
build-minicluster:
	$(GOBUILD) -o ./bin/$(BUILD_TARGET_MINICLUSTER) -v ./tools/minicluster

.PHONY: build-staterecoverer
build-staterecoverer:
	$(GOBUILD) -o ./bin/$(BUILD_TARGET_RECOVER) -v ./tools/staterecoverer

.PHONY: fmt
fmt:
	$(GOCMD) fmt ./...

.PHONY: lint
lint:
	go list ./... | xargs $(GOLINT)

.PHONY: lint-rich
lint-rich:
	$(ECHO_V)rm -rf $(LINT_LOG)
	@echo "Running golangcli lint..."
	$(ECHO_V)golangci-lint run $(VERBOSITY_FLAG)--enable-all -D gochecknoglobals -D prealloc -D lll -D interfacer -D scopelint -D maligned -D dupl| tee -a $(LINT_LOG)
