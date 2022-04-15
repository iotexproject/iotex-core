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
BUILD_TARGET_NEWIOCTL=newioctl
BUILD_TARGET_XCTL=xctl
BUILD_TARGET_NEWXCTL=newxctl
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

.PHONY: test
test: fmt
	$(GOTEST) -short -race ./...

.PHONY: test-rich
test-rich:
	@echo "Running test cases..."
	$(ECHO_V)rm -f $(COV_REPORT)
	$(ECHO_V)touch $(COV_OUT)
	$(ECHO_V)RICHGO_FORCE_COLOR=1 overalls \
		-project=$(ROOT_PKG) \
		-go-binary=richgo \
		-ignore $(TEST_IGNORE) \
		$(DEBUG_FLAG) -- \
		$(VERBOSITY_FLAG) -short | \
		grep -v -e "Test args" -e "Processing"

.PHONY: test-html
test-html: test-rich
	@echo "Generating test report html..."
	$(ECHO_V)gocov convert $(COV_REPORT) | gocov-html > $(COV_HTML)
	$(ECHO_V)open $(COV_HTML)

.PHONY: mockgen
mockgen:
	@./misc/scripts/mockgen.sh

.PHONY: stringer
stringer:
	sh ./misc/scripts/stringer.sh

.PHONY: license
license:
	@./misc/scripts/licenseheader.sh

.PHONY: dev-deps
dev-deps:
	@echo "Installing dev dependencies..."
	$(ECHO_V)go get -u github.com/golangci/golangci-lint/cmd/golangci-lint
	$(ECHO_V)go get -u github.com/kyoh86/richgo
	$(ECHO_V)go get -u github.com/axw/gocov/gocov
	$(ECHO_V)go get -u gopkg.in/matm/v1/gocov-html
	$(ECHO_V)go get -u github.com/go-playground/overalls

.PHONY: clean
clean:
	@echo "Cleaning..."
	$(ECHO_V)rm -rf ./bin/$(BUILD_TARGET_SERVER)
	$(ECHO_V)rm -rf ./bin/$(BUILD_TARGET_ADDRGEN)
	$(ECHO_V)rm -rf ./bin/$(BUILD_TARGET_IOTC)
	$(ECHO_V)rm -rf ./e2etest/*chain*.db
	$(ECHO_V)rm -rf *chain*.db
	$(ECHO_V)rm -rf *trie*.db
	$(ECHO_V)rm -rf *index*.db
	$(ECHO_V)rm -rf *systemlog*.db
	$(ECHO_V)rm -rf *candidate.index*.db
	$(ECHO_V)rm -rf $(COV_REPORT) $(COV_HTML) $(LINT_LOG)
	$(ECHO_V)find . -name $(COV_OUT) -delete
	$(ECHO_V)find . -name $(TESTBED_COV_OUT) -delete
	$(ECHO_V)$(GOCLEAN) -i $(PKGS)

.PHONY: reboot
reboot:
	$(ECHO_V)rm -rf *chain*.db
	$(ECHO_V)rm -rf *trie*.db
	$(ECHO_V)rm -rf *index*.db
	$(ECHO_V)rm -rf *candidate.index*.db
	$(ECHO_V)rm -rf ./e2etest/*chain*.db
	$(GOBUILD) -ldflags "$(PackageFlags)" -o ./bin/$(BUILD_TARGET_SERVER) -v ./$(BUILD_TARGET_SERVER)
	./bin/$(BUILD_TARGET_SERVER) -plugin=gateway -config-path=./config/standalone-config.yaml -genesis-path=./config/standalone-genesis.yaml

.PHONY: run
run:
	$(ECHO_V)rm -rf ./e2etest/*chain*.db
	$(GOBUILD) -ldflags "$(PackageFlags)" -o ./bin/$(BUILD_TARGET_SERVER) -v ./$(BUILD_TARGET_SERVER)
	 sudo mkdir -p /var/data /var/log
	 sudo chown ${USER} /var/data /var/log
	./bin/$(BUILD_TARGET_SERVER) -plugin=gateway -config-path=./config/standalone-config.yaml -genesis-path=./config/standalone-genesis.yaml

.PHONY: docker
docker:
	DOCKER_BUILDKIT=1 $(DOCKERCMD) build -t $(USER)/iotex-core:latest .

.PHONY: minicluster
minicluster:
	$(ECHO_V)rm -rf *chain*.db
	$(ECHO_V)rm -rf *trie*.db
	$(ECHO_V)rm -rf *index*.db
	$(ECHO_V)rm -rf *candidate.index*.db
	$(GOBUILD) -ldflags "$(PackageFlags)" -o ./bin/$(BUILD_TARGET_MINICLUSTER) -v ./tools/minicluster
	./bin/$(BUILD_TARGET_MINICLUSTER)

.PHONY: nightlybuild
nightlybuild:
	$(ECHO_V)rm -rf *chain*.db
	$(ECHO_V)rm -rf *trie*.db
	$(ECHO_V)rm -rf *index*.db
	$(ECHO_V)rm -rf *candidate.index*.db
	$(GOBUILD) -ldflags "$(PackageFlags)" -o ./bin/$(BUILD_TARGET_MINICLUSTER) -v ./tools/minicluster
	./bin/$(BUILD_TARGET_MINICLUSTER) -timeout=14400 -fp-token=true

.PHONY: recover
recover:
	$(ECHO_V)rm -rf ./e2etest/*chain*.db
	$(GOBUILD) -o ./bin/$(BUILD_TARGET_RECOVER) -v ./tools/staterecoverer
	./bin/$(BUILD_TARGET_RECOVER) -plugin=gateway

.PHONY: ioctl
ioctl:
	$(GOBUILD) -ldflags "$(PackageFlags)" -o ./bin/$(BUILD_TARGET_IOCTL) -v ./tools/ioctl

.PHONY: newioctl
newioctl:
	$(GOBUILD) -ldflags "$(PackageFlags)" -o ./bin/$(BUILD_TARGET_NEWIOCTL) -v ./tools/ioctl/newmain
	
.PHONY: ioctl-cross
ioctl-cross:
	$(DOCKERCMD) pull techknowlogick/xgo:latest
	$(GOCMD) get src.techknowlogick.com/xgo
	mkdir -p $(GOPATH)/src
	sudo cp ./tools/ioctl/ioctl.go $(GOPATH)/src
	cd $(GOPATH)/src && sudo rm -f go.mod && $(GOCMD) mod init main && $(GOCMD) mod tidy
	cd $(GOPATH)/src && $(GOPATH)/bin/xgo --targets=$(BUILD_TARGET_OS)/$(BUILD_TARGET_ARCH) .
	mkdir -p ./bin/$(BUILD_TARGET_IOCTL) && sudo mv $(GOPATH)/src/main-$(BUILD_TARGET_OS)-$(BUILD_TARGET_ARCH) ./bin/$(BUILD_TARGET_IOCTL)/ioctl-$(BUILD_TARGET_OS)-$(BUILD_TARGET_ARCH)

.PHONY: xctl
xctl:
	$(GOBUILD) -ldflags "$(PackageFlags)" -o ./bin/$(BUILD_TARGET_XCTL) -v ./tools/xctl

.PHONY: newxctl
newxctl:
	$(GOBUILD) -ldflags "$(PackageFlags)" -o ./bin/$(BUILD_TARGET_NEWXCTL) -v ./tools/xctl/newmain

.PHONY: iomigrater
iomigrater:
	$(GOBUILD) -ldflags "$(PackageFlags)" -o ./bin/$(BUILD_TARGET_IOMIGRATER) -v ./tools/iomigrater
