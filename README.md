# job-discovery

Pulls job listings from Greenhouse/Lever/Ashby; emits listing.discovered on NATS

Part of the personal job-hunt platform. Canonical design and conventions live
in the homelab-fleet repo at `docs/job-hunt/plan.md`. See [CLAUDE.md](./CLAUDE.md)
for agent context.

## Status

Design pass complete: OpenAPI spec, migrations, sqlc queries. No application
code yet — that arrives in the build pass.

## Quickstart

```bash
make generate   # oapi-codegen + sqlc → internal/api, internal/store/db
make lint
make test
make build
```

Database migrations are applied via `golang-migrate` against the
`DATABASE_URL`; in-cluster, CloudNativePG provisions the DB and the
Deployment runs migrations on startup. See `plan.md` for the full deploy
story.

## Layout

See the plan for the canonical repo layout. Highlights:

- `api/openapi.yaml` — source of truth for the HTTP API.
- `internal/store/queries/` — sqlc input.
- `migrations/` — golang-migrate SQL migrations.
- `deploy/` — Kustomize manifests (base + overlays).

## License

MIT — see [LICENSE](./LICENSE).
