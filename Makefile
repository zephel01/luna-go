.PHONY: build test vet lint clean install sandbox-build test-sandbox

BINARY  := luna
VERSION := $(shell grep 'const version' cmd/root.go | grep -o '"[^"]*"' | tr -d '"')
LDFLAGS := -ldflags "-s -w"

## build: compile binary to ./luna
build:
	go build $(LDFLAGS) -o $(BINARY) .

## install: install binary to $GOPATH/bin (or ~/go/bin)
install:
	go install $(LDFLAGS) .

## test: run all tests
test:
	go test ./...

## vet: run go vet
vet:
	go vet ./...

## lint: run govulncheck (install: go install golang.org/x/vuln/cmd/govulncheck@latest)
lint: vet
	govulncheck ./...

## clean: remove built binary
clean:
	rm -f $(BINARY)

## version: print current version
version:
	@echo "luna-go v$(VERSION)"

## sandbox-build: build the luna-sandbox Docker image
sandbox-build:
	docker build -t luna-sandbox:latest -f scripts/sandbox/Dockerfile scripts/sandbox/

## test-sandbox: run sandbox integration tests (requires Docker)
test-sandbox:
	bash scripts/test-sandbox.sh

## test-sandbox-quick: run sandbox tests, skip Docker build and luna LLM test
test-sandbox-quick:
	SKIP_BUILD=1 SKIP_LUNA_TEST=1 bash scripts/test-sandbox.sh

## help: show this message
help:
	@grep -E '^## ' Makefile | sed 's/## /  /'
