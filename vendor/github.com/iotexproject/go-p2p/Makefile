# Go parameters
GOCMD=go
GOLINT=golint
GOBUILD=$(GOCMD) build
GOINSTALL=$(GOCMD) install
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
SKIP_DEP=false

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
	$(GOBUILD) -o ./bin/p2p -v ./main

.PHONY: fmt
fmt:
	$(GOCMD) fmt ./...

.PHONY: lint
lint:
	go list ./... | grep -v /vendor/ | grep -v /explorer/idl/ | xargs $(GOLINT)

.PHONY: test
test: fmt
	$(GOTEST) -short ./...

.PHONY: clean
clean:
	@echo "Cleaning..."
	$(ECHO_V)rm -f ./bin/p2p
	$(ECHO_V)rm -f $(COV_REPORT) $(COV_HTML) $(LINT_LOG)
	$(ECHO_V)find . -name $(COV_OUT) -delete
	$(ECHO_V)find . -name $(TESTBED_COV_OUT) -delete
	$(ECHO_V)$(GOCLEAN) -i $(PKGS)

.PHONY: docker
docker:
	$(DOCKERCMD) build -t $(USER)/go-p2p:latest --build-arg SKIP_DEP=$(SKIP_DEP) .
