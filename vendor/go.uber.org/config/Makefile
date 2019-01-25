SHELL := /usr/bin/env bash
TEST := go test -race

.PHONY: install
install:
	glide --version || go get github.com/Masterminds/glide
	glide install

.PHONY: test
test:
	$(TEST) ./...

.PHONY: ci
ci:
ifdef COVER
	$(TEST) -cover -coverprofile=coverage.txt ./...
	bash <(curl -s https://codecov.io/bash)
else
	$(TEST) ./...
endif
