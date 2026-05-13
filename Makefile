# job-discovery — see CLAUDE.md and the canonical plan referenced therein.

BINARY := job-discovery
PKG    := ./...

.PHONY: all build test test-integration lint generate run tidy clean

all: lint test build

build:
	go build -o bin/$(BINARY) ./cmd/server

test:
	go test -race -coverprofile=cover.out $(PKG)

test-integration:
	go test -race -tags integration -coverprofile=cover.out $(PKG)

lint:
	golangci-lint run --timeout=5m

generate:
	go generate $(PKG)

run:
	go run ./cmd/server

tidy:
	go mod tidy

clean:
	rm -rf bin dist cover.out
