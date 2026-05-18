#!/bin/sh
set -e

echo "==> fmt"
BAD=$(gofumpt -l .)
[ -z "$BAD" ] || { printf "Not gofumpt'd:\n%s\nRun: make fmt\n" "$BAD"; exit 1; }

echo "==> mod verify"
go mod verify

echo "==> generate"
go generate ./...
git diff --exit-code || { echo "Generated files out of date. Run: make generate"; exit 1; }

echo "==> lint"
golangci-lint run --timeout=5m

echo "==> test"
go test -race -coverprofile=cover.out ./...
go tool cover -func=cover.out | tail -1

echo "==> build"
go build -o /dev/null ./cmd/server

echo "==> migrations (idempotency)"
migrate -path ./migrations -database "$POSTGRES_DSN" up
migrate -path ./migrations -database "$POSTGRES_DSN" up

echo ""
echo "All checks passed."
