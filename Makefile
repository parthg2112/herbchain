APPNAME := herbchain
BINARY  := $(APPNAME)d
BUILDDIR ?= $(CURDIR)/build

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT  := $(shell git rev-parse HEAD 2>/dev/null)

ldflags = -X github.com/cosmos/cosmos-sdk/version.Name=$(APPNAME) \
	-X github.com/cosmos/cosmos-sdk/version.AppName=$(BINARY) \
	-X github.com/cosmos/cosmos-sdk/version.Version=$(VERSION) \
	-X github.com/cosmos/cosmos-sdk/version.Commit=$(COMMIT)
BUILD_FLAGS := -mod=readonly -ldflags '$(ldflags)'

# Pinned to match .github/workflows/ci.yml; v2 is required for Go 1.26 sources.
GOLANGCI_LINT_VERSION := v2.14.0
GOLANGCI_LINT ?= golangci-lint
BUF ?= go tool github.com/bufbuild/buf/cmd/buf

COVER_PKGS ?= ./x/...

.PHONY: all build install test test-race cover lint lint-go lint-proto lint-install \
	proto-gen serve demo demo-ibc clean

all: build

build:
	go build $(BUILD_FLAGS) -o $(BUILDDIR)/$(BINARY) ./cmd/$(BINARY)

install:
	go install $(BUILD_FLAGS) ./cmd/$(BINARY)

test:
	go test -mod=readonly ./...

test-race:
	go test -mod=readonly -race ./...

cover:
	go test -mod=readonly -coverprofile=coverage.out -covermode=atomic $(COVER_PKGS)
	go tool cover -func=coverage.out | tail -n 1

lint: lint-go lint-proto

lint-go:
	$(GOLANGCI_LINT) run ./... --timeout 15m

lint-proto:
	$(BUF) lint

lint-install:
	go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION)

proto-gen:
	ignite generate proto-go --yes </dev/null

serve:
	ignite chain serve --reset-once --yes

demo:
	./scripts/demo.sh

demo-ibc:
	./scripts/demo-ibc.sh

clean:
	rm -rf $(BUILDDIR) coverage.out
