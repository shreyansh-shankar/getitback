BINARY     = getitback
CMD_DIR    = ./cmd/getitback
VERSION   ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT    ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
DATE      ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)

LD_FLAGS   = -ldflags="-s -w \
	-X main.version=$(VERSION) \
	-X main.commit=$(COMMIT) \
	-X main.date=$(DATE)"

.PHONY: all build clean install test test-coverage lint vet tidy help

all: build

## build: Build the binary for the current platform
build:
	go build $(LD_FLAGS) -o $(BINARY) $(CMD_DIR)

## install: Build and install to /usr/local/bin
install: build
	install -m 755 $(BINARY) /usr/local/bin/$(BINARY)
	@echo "Installed $(BINARY) to /usr/local/bin"

## clean: Remove build artifacts
clean:
	rm -f $(BINARY) coverage.out

## test: Run all unit tests
test:
	go test -v -race -timeout 120s ./...

## test-coverage: Run tests and generate coverage report
test-coverage:
	go test -coverprofile=coverage.out -covermode=atomic ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

## lint: Run go vet
lint: vet

## vet: Run go vet
vet:
	go vet ./...

## tidy: Tidy go modules
tidy:
	go mod tidy

## help: Show this help message
help:
	@echo "Usage: make <target>"
	@echo ""
	@echo "Targets:"
	@grep -E '^## ' Makefile | sed 's/## /  /'
