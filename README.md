# job-discovery

> Pulls job listings from external boards (Greenhouse, Lever, Ashby) into a
> service-owned Postgres and emits `jobhunt.listing.discovered` on NATS
> JetStream whenever a listing is seen for the first time.

`job-discovery` is the entry point for the personal job-hunt platform. Other
services (`job-scoring`, `resume-forge`, `application-tracker`) consume from
here — either over HTTP for read APIs or off the NATS subject for async
fan-out.

The canonical design doc and platform-wide conventions live in the
[`homelab-fleet`](https://github.com/thadamski/homelab-fleet) repo at
[`docs/job-hunt/plan.md`](https://github.com/thadamski/homelab-fleet/blob/main/docs/job-hunt/plan.md).
Treat it as authoritative over general best-practice instincts. The plan's
hard constraints are non-negotiable.

## Status

| | |
|---|---|
| **Latest tag** | `v0.1.1` |
| **Deployed on** | cassette (k3s, namespace `job-hunt`) via Flux GitOps |
| **Image** | `ghcr.io/thadamski/job-discovery` (multi-arch, distroless nonroot, SBOM + cosign signed) |
| **Smoke test** | passed 2026-05-14 (414 listings, NATS persistence + idempotency confirmed) |

## What this service does

1. Holds a list of `companies` (greenhouse/lever/ashby slugs, priority, tags)
   in its own Postgres database.
2. On `POST /api/v1/job-discovery/refresh`, fetches listings from each active
   company's board, inserts new ones, and publishes
   `jobhunt.listing.discovered` on NATS for each newly inserted listing.
3. Exposes a read API over `listings` so consumers can fetch a listing's full
   payload or filter by status, company, etc.
4. Lets a caller transition a listing's `status` (`new → filtered_out → scored
   → shortlisted → applied → closed | passed`) via `PATCH /listings/{id}`.

Greenhouse is the only board implemented today (`internal/discovery/greenhouse.go`).
Lever and Ashby are planned, not built — add a new fetcher under
`internal/discovery/` and wire it into `refresh.go` when needed.

## HTTP API

Versioned under `/api/v1/job-discovery/`. Errors are
[RFC 7807 problem details](https://www.rfc-editor.org/rfc/rfc7807)
(`application/problem+json`).

| Method | Path | Purpose |
|--------|------|---------|
| POST   | `/api/v1/job-discovery/refresh`            | Fetch listings (all or by `company_id` / `board_kind`). |
| GET    | `/api/v1/job-discovery/companies`          | List companies. |
| POST   | `/api/v1/job-discovery/companies`          | Register a company. |
| GET    | `/api/v1/job-discovery/listings`           | List listings (filter by `status`, `company`, `min_score`; paginate via `page` / `page_size`). |
| GET    | `/api/v1/job-discovery/listings/{id}`      | Fetch a single listing. |
| PATCH  | `/api/v1/job-discovery/listings/{id}`      | Update mutable fields (currently `status` only). |

Operational endpoints (`/healthz`, `/readyz`, `/metrics`) live at the root and
are intentionally not part of the versioned API.

The source of truth for the API is [`api/openapi.yaml`](api/openapi.yaml).
Handlers are generated from it via `oapi-codegen`; see
[Code generation](#code-generation) below.

## NATS events

| Subject | When | Payload |
|---------|------|---------|
| `jobhunt.listing.discovered` | A listing is inserted for the first time (re-fetches of the same `(company_id, external_id)` do **not** re-emit). | `{listing_id, company_name, title, url}` |

Stream name: `JOBHUNT` (subject filter `jobhunt.>`). JetStream is required;
the publisher creates/updates the stream config on boot.

## Data model

Two tables, both owned by this service:

```
companies                       listings
─────────                       ────────
id (uuid, pk)                   id (uuid, pk)
name                            company_id (uuid, fk → companies.id)
board_kind  ┐ unique together   external_id ┐ unique together
board_slug  ┘                   title       ┘
priority (smallint, 1-5)        location (nullable)
tags (text[])                   url
notes (nullable)                description
active (bool)                   raw_payload (jsonb)
created_at (timestamptz)        posted_at (nullable)
                                fetched_at
                                status (text, see plan.md)
```

Schema and indexes live in [`migrations/`](migrations/). Migrations are
applied at deploy time by an init container; see
[Deployment](#deployment) below.

## Repo layout

```
.
├── api/openapi.yaml            # source of truth for the HTTP API
├── cmd/server/main.go          # wiring only — no business logic
├── internal/
│   ├── api/                    # oapi-codegen output + handlers (handlers.go is hand-written)
│   ├── config/                 # env-driven config struct (POSTGRES_DSN required)
│   ├── discovery/              # per-board fetchers + refresh.go (orchestrator)
│   ├── events/                 # NATS JetStream publisher
│   ├── obs/                    # slog JSON + OTel + Prometheus
│   └── store/
│       ├── db/                 # sqlc-generated code — DO NOT EDIT
│       ├── queries/            # *.sql files — sqlc input
│       └── store.go            # Store interface + pgx pool wiring
├── migrations/                 # golang-migrate format (NNNN_<slug>.up.sql + .down.sql)
├── deploy/
│   ├── base/                   # Kustomize base (deployment + migrate init + db + netpol + service)
│   └── overlays/cassette/      # cluster-specific overlay (image tag + GHCR pull secret)
├── tools/tools.go              # build-time tool pins (oapi-codegen, sqlc)
├── Dockerfile                  # multi-stage, distroless nonroot
├── Makefile                    # `make lint`, `make test`, `make generate`, `make build`
└── .github/workflows/
    ├── ci.yaml                 # mod verify, generate-clean check, lint, race tests, migration idempotency, docker build
    └── release.yaml            # release-please → multi-arch image + SBOM + cosign sign
```

The repo layout matches `docs/job-hunt/plan.md` exactly. If you find yourself
about to deviate, surface the gap and ask first.

## Local development

```bash
make generate   # oapi-codegen + sqlc → internal/api/gen.go, internal/store/db/
make lint       # strict golangci-lint (see .golangci.yml)
make test       # race + coverage
make build      # binary into bin/job-discovery
make run        # POSTGRES_DSN=... go run ./cmd/server
```

Required environment for `make run`:

| Var | Default | Notes |
|-----|---------|-------|
| `POSTGRES_DSN`              | _required_                                            | e.g. `postgres://app:app@localhost:5432/job_discovery?sslmode=disable` |
| `NATS_URL`                  | `nats://nats.platform.svc.cluster.local:4222`         | Connect string passed to `nats.Connect`. |
| `HTTP_ADDR`                 | `:8080`                                               | |
| `LOG_LEVEL`                 | `info`                                                | `debug` / `info` / `warn` / `error`. |
| `OTEL_EXPORTER_OTLP_ENDPOINT` | _unset_                                             | When unset, the service uses a no-op tracer. |

Migrations: `migrate -path ./migrations -database $POSTGRES_DSN up`.

### Code generation

We're OpenAPI-first and sqlc-first:

1. To change the API: edit `api/openapi.yaml` → `make generate` → adjust
   handlers in `internal/api/handlers.go`.
2. To change the schema: add a numbered migration in `migrations/` → edit/add
   the matching query in `internal/store/queries/` → `make generate` →
   adapt the domain layer.

**Generated code is committed to git.** CI fails if `go generate ./...`
produces a diff.

**No generator-specific types in application code.** sqlc and oapi-codegen
are configured to emit Go-native types (`google/uuid.UUID`, `time.Time`,
pointers for nullable scalars, `[]byte` for jsonb, `[]string` for `text[]`).
If `grep -rE 'pgtype\.|openapi_types\.' internal/ cmd/` returns anything,
the override list is incomplete — fix it before merging.

## Deployment

Flux GitOps from [`homelab-fleet`](https://github.com/thadamski/homelab-fleet)
applies `deploy/overlays/cassette/` to the `job-hunt` namespace on cassette.
On every Pod start, an init container running `migrate/migrate:4` applies any
pending migrations from a ConfigMap mount, then the main container starts.

| Resource | Purpose |
|----------|---------|
| `Deployment job-discovery`     | Distroless nonroot pod, `/healthz` + `/readyz` probes, resource limits, read-only rootfs. |
| `Service job-discovery`        | ClusterIP :80 → pod :8080. |
| `Cluster job-discovery-db`     | CloudNativePG primary, 5Gi PVC, owns DB `job_discovery`. |
| `ConfigMap …-migrations`       | SQL files mounted into the migrate init container. **Keep in sync with `migrations/`.** |
| `NetworkPolicy default-deny` + targeted allows | Hermes ingress only; egress restricted to DB, NATS, external HTTPS (board APIs), DNS. |
| `Secret ghcr-pull` (cassette overlay) | SOPS-encrypted; provides imagePullSecret for `ghcr.io/thadamski/job-discovery`. Shared with sibling services in the same namespace. |

Image tags are bumped through Flux's image-automation controllers — see
`apps/job-hunt/image-automation.yaml` in the fleet repo.

## Release process

Commits to `main` are scanned by `release-please`. When it sees `feat:` /
`fix:` / `BREAKING CHANGE:` (Conventional Commits), it opens a release PR
that updates the changelog and bumps the version. Merging the PR creates a
git tag, which triggers the multi-arch image build, SBOM, and cosign signing.

To cut a release manually you only need to merge the open release-please PR
(or push a `vX.Y.Z` tag if you need to skip release-please entirely).

## Conventions for contributors (humans and agents)

1. **Conventional Commits, always.** Release-please depends on it.
2. **Edit OpenAPI / SQL first.** Don't write Go that touches the API surface
   or the DB before the spec/migration/query exists.
3. **Tests are stdlib `testing` only.** Table-driven. No testify, no assert.
4. **No `pgtype.*` / `openapi_types.*` leakage into app code.**
5. **No global state.** Inject everything via constructors.
6. **`context.Context` is the first parameter everywhere**; never store on a
   struct.
7. **Don't smuggle in things the plan says aren't in v1** (LinkedIn, web UI,
   auto-apply, plugin systems, custom retry on top of JetStream, etc.).
   Surface the gap, ask, defer.

See [CLAUDE.md](./CLAUDE.md) for the agent-facing summary of these rules.

## License

MIT — see [LICENSE](./LICENSE).
