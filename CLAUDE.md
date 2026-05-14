# job-discovery — agent context

If you're a coding agent landing in this repo, read this whole file before
making changes. Then keep [`docs/job-hunt/plan.md`](https://github.com/thadamski/homelab-fleet/blob/main/docs/job-hunt/plan.md)
open in another tab — it's the canonical design and convention doc for the
broader platform, and it overrides anything you'd otherwise infer from
general best practices.

## What this service is

Pulls job listings from external boards into a service-owned Postgres and
emits `jobhunt.listing.discovered` on NATS JetStream whenever a listing is
seen for the first time. Greenhouse is the only board implemented today;
Lever and Ashby are planned.

The plan describes the broader four-service platform — `job-scoring`,
`resume-forge`, and `application-tracker` are the siblings. They live in
their own repos; cross-service work happens over HTTP and NATS, never
direct DB access.

## Hard rules (non-negotiable)

These are summarised from `plan.md`. If anything below disagrees with that
file, the plan wins.

- **Go 1.23+, stdlib + `chi` only.** No Gin/Echo. The `go.mod` declares the
  exact toolchain.
- **OpenAPI-first.** `api/openapi.yaml` is the source of truth for the HTTP
  surface. Edit the spec, run `make generate`, then adjust handlers.
- **sqlc-first.** `migrations/*.sql` and `internal/store/queries/*.sql` are
  the source of truth for the data layer. Migration files use the
  golang-migrate format (`NNNN_<slug>.up.sql` + `.down.sql`). Edit migration
  + query, then `make generate`.
- **No generator-specific types in application code.** No `pgtype.*`, no
  `openapi_types.*`. sqlc and oapi-codegen are configured with overrides so
  UUIDs are `google/uuid.UUID`, timestamps are `time.Time`, nullable scalars
  are pointers, jsonb is `[]byte`, `text[]` is `[]string`. Verify with
  `grep -rE 'pgtype\.|openapi_types\.' internal/ cmd/` returning empty. If
  it doesn't, the override list is incomplete — fix `sqlc.yaml` /
  `oapi-codegen.yaml` before merging.
- **sqlc queries use named params only.** `@col_name`, never `$1`/`$2`.
  Nullable filters use `sqlc.narg('name')::<type>`.
- **`context.Context` is always the first parameter.** Never stored on a
  struct.
- **Error wrap with `fmt.Errorf("doing X: %w", err)`.** No `pkg/errors`.
- **`slog` JSON, always.** `service`, `trace_id`, `request_id` bound.
- **Tests are stdlib `testing` only**, table-driven. No testify, no assert
  libraries.
- **Generated code is committed.** CI runs `go generate ./...` followed by
  `git diff --exit-code`.
- **Conventional Commits, always.** `release-please` depends on it.
- **No business logic in `cmd/server/main.go`** — wiring only.
- **No global state.** Inject everything via constructors.

## Data layer — source of truth

`migrations/*.sql` and `internal/store/queries/*.sql` are the source of truth.
The flow for any data-layer change:

1. Add a numbered migration (`migrations/NNNN_<slug>.up.sql` + `.down.sql`).
2. Edit/add the matching query in `internal/store/queries/`.
3. `make generate` — regenerates `internal/store/db/`.
4. Adapt the domain layer to the new generated types.
5. **Also update `deploy/base/migrations-configmap.yaml`** — that ConfigMap
   is what the migrate init container actually applies in-cluster. The CI
   `migration-idempotency` job runs migrations twice against a fresh
   Postgres to catch non-idempotent SQL.

Never hand-write code in `internal/store/db/`. Never write Go that touches
the DB before the migration and the `.sql` query both exist.

## Where things live

| You want to change… | Edit… | Then… |
|---|---|---|
| The HTTP API shape         | `api/openapi.yaml`                         | `make generate`, adjust `internal/api/handlers.go` |
| The DB schema              | new file in `migrations/`                  | add query in `internal/store/queries/`, `make generate`, update `deploy/base/migrations-configmap.yaml` |
| Business logic             | `internal/discovery/`                      | add a test in the same package |
| NATS publishing            | `internal/events/publisher.go`             | new subject? new event struct + a new `Publish<Foo>` method |
| HTTP response helpers      | `internal/api/handlers.go` (e.g. `problem`) | add a constant for the status |
| Metrics                    | `internal/obs/obs.go`                      | add the new field to `Obs`, use it in the call site |
| Deployment manifests       | `deploy/base/`                             | overlays only override; keep the base correct for any cluster |

## What's intentionally NOT in v1 (don't smuggle it in)

- Lever / Ashby fetchers — add when needed, but the plan says Greenhouse-first
  to prove the architecture.
- A web UI.
- Auto-apply, LinkedIn integration, PDF/docx export (these belong elsewhere).
- Custom retry logic on top of JetStream — use the built-in semantics.
- Direct DB access from sibling services — they go through this service's
  HTTP API or off the NATS subject.
- An ORM. Ever.

If a request seems to want one of these, surface the gap and ask.

## Common pitfalls (learned the hard way)

These are recorded here so future-you doesn't pay the cost again. Most of
them are also in `plan.md`'s "Infrastructure lessons" section.

- **Generator type leakage is contagious.** If one handler accepts
  `pgtype.Text`, every caller starts importing `pgtype`. Fix the override
  config, never the call site.
- **`sqlc` is picky about parameter ordering.** It assigns `$N` placeholders
  by the order the field appears in the generated struct, not by SQL
  appearance order. If you reorder a SELECT's WHERE clauses and the params
  shift, regenerate and re-test.
- **The migrate init container needs the ConfigMap to be in sync with
  `migrations/`.** A drift here will pass CI but break the deploy. Treat
  the ConfigMap as a deploy-time mirror, not a separate source of truth.
- **CloudNativePG pod labels are `cnpg.io/cluster`**, not
  `postgresql.cnpg.io/cluster`. NetworkPolicy selectors fail silently if
  you get this wrong.
- **k3s evaluates NetworkPolicy after DNAT.** To allow egress to the
  Kubernetes API server, both the ClusterIP (`10.43.0.1:443`) and the node
  endpoint (`10.0.0.51:6443`) must be in the `ipBlock` allow list.
- **NATS monitor HTTP server is off by default.** If you're touching the
  NATS config, make sure `http_port: 8222` stays enabled — the readiness
  probe relies on it.
- **The `release-please` action emits `outputs.pr` as a JSON object, not a
  number.** The auto-merge step in `release.yaml` uses
  `fromJson(steps.release.outputs.pr).number` — don't simplify it.
- **GitHub Actions ship with read-only permissions on new repos.** Run
  `gh api -X PUT repos/<owner>/<repo>/actions/permissions/workflow -F default_workflow_permissions=write -F can_approve_pull_request_reviews=true`
  once after creating a repo, or the first release-please run fails.

## Local commands

```
make generate          # oapi-codegen + sqlc
make lint              # strict golangci-lint (see .golangci.yml)
make test              # race + coverage
make test-integration  # testcontainers-go against a real Postgres
make build             # bin/job-discovery
make run               # POSTGRES_DSN=... go run ./cmd/server
```

## When in doubt

Surface the gap and ask. Do not smuggle in things the plan says aren't in
v1. Do not write a 200-line abstraction to delay one 10-line decision. The
plan and this file are how Tommy thinks; if your instinct fights them,
trust them and ask.
