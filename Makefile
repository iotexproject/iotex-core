########################################################################################################################
# Copyright (c) 2018 IoTeX
# This is an alpha (internal) release and is not suitable for production. This source code is provided ‘as is’ and no
# warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
# permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
# License 2.0 that can be found in the LICENSE file.
########################################################################################################################

# Go parameters
GOCMD=go
GOLINT=golint
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
BUILD_TARGET_SERVER=server
BUILD_TARGET_TXINJ=txinjector

# Docker parameters
DOCKERCMD=docker

all: build test
.PHONY: build
build:
	$(GOBUILD) -o ./bin/$(BUILD_TARGET_SERVER) -v ./$(BUILD_TARGET_SERVER)
	$(GOBUILD) -o ./bin/$(BUILD_TARGET_TXINJ) -v ./tools/txinjector

.PHONY: fmt
fmt:
	$(GOCMD) fmt ./...

.PHONY: lint
lint:
	go list ./... | grep -v /vendor/ | xargs $(GOLINT)

.PHONY: test
test: fmt
	rm -f ./e2etests/db.test
	$(GOTEST) -short ./...

.PHONY: mockgen
mockgen:
	@./misc/scripts/mockgen.sh

.PHONY: stringer
stringer:
	sh ./misc/scripts/stringer.sh

.PHONY: clean
clean:
	$(GOCLEAN)
	rm -f ./bin/$(BUILD_TARGET_SERVER)
	rm -f ./bin/$(BUILD_TARGET_TXINJ)
	rm -f chain.db
	rm -f block.dat

.PHONY: run
run:
	$(GOBUILD) -o ./bin/$(BUILD_TARGET_SERVER) -v ./$(BUILD_TARGET_SERVER)
	./bin/$(BUILD_TARGET_SERVER) -config=e2etests/config_local_delegate.yaml -debug=true

.PHONY: docker
docker:
	$(DOCKERCMD) build -t iotex-go:1.0 .
