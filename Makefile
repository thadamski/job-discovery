# job-discovery — see CLAUDE.md and the canonical plan referenced therein.

BINARY  := job-discovery
PKG     := ./...
COMPOSE := docker compose

.PHONY: qa qa-build fmt build-image all build test test-integration lint generate run tidy clean

## Docker-based QA (run before every commit)
qa: ## Full CI check suite in Docker
	$(COMPOSE) run --rm dev ./qa.sh

qa-build: ## Rebuild the dev container image
	$(COMPOSE) build dev

fmt: ## Format code with gofumpt
	$(COMPOSE) run --rm dev gofumpt -w .

build-image: ## Build Docker image locally (mirrors CI docker/build-push-action)
	docker build .

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
