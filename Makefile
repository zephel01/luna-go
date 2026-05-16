.PHONY: build test vet lint clean install

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

## help: show this message
help:
	@grep -E '^## ' Makefile | sed 's/## /  /'
