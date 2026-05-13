# job-discovery — agent context

## Canonical plan

The authoritative design doc for this service and the broader job-hunt platform
lives outside this repo, in:

```
github.com/thadamski/homelab-fleet : docs/job-hunt/plan.md
```

**Read it before making non-trivial changes.** Treat it as authoritative over
general best-practice instincts. Hard constraints there are non-negotiable.

## Hard rules (summary — see plan for full text)

- Go 1.23+, stdlib + `chi`. No Gin/Echo.
- OpenAPI-first: edit `api/openapi.yaml` first, regenerate, then implement.
- `sqlc` for data access. SQL in `internal/store/queries/`. No ORMs.
- **No generator-specific types in app code.** No `pgtype.*`, no `openapi_types.*`. sqlc and oapi-codegen are configured with overrides so UUIDs are `google/uuid.UUID`, timestamps are `time.Time`, nullable scalars are pointers, jsonb is `[]byte`. Verify with `grep -rE 'pgtype\.|openapi_types\.' internal/ cmd/` returning empty. See **Code generation rules** in `plan.md`.
- **sqlc queries use named params only**: `@col_name`, never `$1`/`$2`; nullable filters via `sqlc.narg('name')::<type>`.
- `context.Context` is always the first parameter; never stored on structs.
- Error wrap with `fmt.Errorf("doing X: %w", err)`. No `pkg/errors`.
- `slog` JSON, `service`/`trace_id`/`request_id` always bound.
- Tests: stdlib `testing` only, table-driven. No testify.
- Generated code is committed; CI verifies `go generate ./...` is clean.
- Conventional Commits for all messages (release-please depends on it).

## Local commands

- `make lint` — strict golangci-lint
- `make test` — race + coverage
- `make generate` — oapi-codegen + sqlc
- `make build` — binary into `bin/`

## When in doubt

Surface the gap and ask. Do not smuggle in things the plan says aren't in v1.
