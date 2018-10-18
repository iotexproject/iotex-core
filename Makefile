########################################################################################################################
# Copyright (c) 2018 IoTeX
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
BUILD_TARGET_SERVER=server
BUILD_TARGET_ACTINJ=actioninjector
BUILD_TARGET_ADDRGEN=addrgen
BUILD_TARGET_IOTC=iotc
BUILD_TARGET_MINICLUSTER=minicluster
SKIP_DEP=false

# Pkgs
ALL_PKGS := $(shell go list ./... )
PKGS := $(shell go list ./... | grep -v /test/ )
ROOT_PKG := "github.com/iotexproject/iotex-core"

# Docker parameters
DOCKERCMD=docker

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

all: clean build test
.PHONY: build
build:
	$(GOBUILD) -o ./bin/$(BUILD_TARGET_SERVER) -v ./$(BUILD_TARGET_SERVER)
	$(GOBUILD) -o ./bin/$(BUILD_TARGET_ACTINJ) -v ./tools/actioninjector
	$(GOBUILD) -o ./bin/$(BUILD_TARGET_ADDRGEN) -v ./tools/addrgen
	$(GOBUILD) -o ./bin/$(BUILD_TARGET_IOTC) -v ./cli/iotc
	$(GOBUILD) -o ./bin/$(BUILD_TARGET_MINICLUSTER) -v ./tools/minicluster

.PHONY: fmt
fmt:
	$(GOCMD) fmt ./...

.PHONY: lint
lint:
	go list ./... | grep -v /vendor/ | grep -v /explorer/idl/ | xargs $(GOLINT)

.PHONY: lint-rich
lint-rich:
	$(ECHO_V)rm -rf $(LINT_LOG)
	@echo "Running golangcli lint..."
	$(ECHO_V)golangci-lint run $(VERBOSITY_FLAG)--enable-all | tee -a $(LINT_LOG)

.PHONY: test
test: fmt
	$(GOTEST) -short ./...

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
	$(ECHO_V)rm -f ./bin/$(BUILD_TARGET_SERVER)
	$(ECHO_V)rm -f ./bin/$(BUILD_TARGET_ACTINJ)
	$(ECHO_V)rm -f ./bin/$(BUILD_TARGET_ADDRGEN)
	$(ECHO_V)rm -f ./bin/$(BUILD_TARGET_IOTC)
	$(ECHO_V)rm -f ./e2etest/chain*.db
	$(ECHO_V)rm -f chain.db
	$(ECHO_V)rm -f trie.db
	$(ECHO_V)rm -f $(COV_REPORT) $(COV_HTML) $(LINT_LOG)
	$(ECHO_V)find . -name $(COV_OUT) -delete
	$(ECHO_V)find . -name $(TESTBED_COV_OUT) -delete
	$(ECHO_V)$(GOCLEAN) -i $(PKGS)

.PHONY: reboot
reboot:
	$(ECHO_V)rm -f chain.db
	$(ECHO_V)rm -f trie.db
	$(ECHO_V)rm -f ./e2etest/chain*.db
	$(GOBUILD) -o ./bin/$(BUILD_TARGET_SERVER) -v ./$(BUILD_TARGET_SERVER)
	export LD_LIBRARY_PATH=$(LD_LIBRARY_PATH):$(PWD)/crypto/lib:$(PWD)/crypto/lib/blslib
	./bin/$(BUILD_TARGET_SERVER) -config-path=e2etest/config_local_delegate.yaml -log-colorful=true

.PHONY: run
run:
	$(ECHO_V)rm -f ./e2etest/chain*.db
	$(GOBUILD) -o ./bin/$(BUILD_TARGET_SERVER) -v ./$(BUILD_TARGET_SERVER)
	export LD_LIBRARY_PATH=$(LD_LIBRARY_PATH):$(PWD)/crypto/lib:$(PWD)/crypto/lib/blslib
	./bin/$(BUILD_TARGET_SERVER) -config-path=e2etest/config_local_delegate.yaml -log-colorful=true

.PHONY: docker
docker:
	$(DOCKERCMD) build -t $(USER)/iotex-core:latest --build-arg SKIP_DEP=$(SKIP_DEP) .

.PHONY: minicluster
minicluster:
	$(ECHO_V)rm -f chain*.db
	$(ECHO_V)rm -f trie*.db
	$(GOBUILD) -o ./bin/$(BUILD_TARGET_MINICLUSTER) -v ./tools/minicluster
	export LD_LIBRARY_PATH=$(LD_LIBRARY_PATH):$(PWD)/crypto/lib:$(PWD)/crypto/lib/blslib
	./bin/$(BUILD_TARGET_MINICLUSTER) -log-colorful=true

.PHONY: nightlybuild
nightlybuild:
	$(ECHO_V)rm -f chain*.db
	$(ECHO_V)rm -f trie*.db
	$(GOBUILD) -o ./bin/$(BUILD_TARGET_MINICLUSTER) -v ./tools/minicluster
	export LD_LIBRARY_PATH=$(LD_LIBRARY_PATH):$(PWD)/crypto/lib:$(PWD)/crypto/lib/blslib
	./bin/$(BUILD_TARGET_MINICLUSTER) -log-colorful=true -timeout=14400