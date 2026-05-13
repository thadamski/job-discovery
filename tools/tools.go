//go:build tools

// Package tools pins build-time tool dependencies so `go mod tidy` keeps them in go.sum.
// Run `go generate ./...` from the repo root to invoke the pinned versions.
package tools

import (
	_ "github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen"
	_ "github.com/sqlc-dev/sqlc/cmd/sqlc"
)
