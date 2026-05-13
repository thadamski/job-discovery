# job-discovery

Pulls job listings from Greenhouse/Lever/Ashby; emits listing.discovered on NATS

Part of the personal job-hunt platform. Canonical design and conventions live
in the homelab-fleet repo at `docs/job-hunt/plan.md`. See [CLAUDE.md](./CLAUDE.md)
for agent context.

## Status

Bootstrap only — no application code yet.

## Quickstart

```bash
make lint
make test
make build
```

## Layout

See the plan for the canonical repo layout. Highlights:

- `api/openapi.yaml` — source of truth for the HTTP API.
- `internal/store/queries/` — sqlc input.
- `migrations/` — golang-migrate SQL migrations.
- `deploy/` — Kustomize manifests (base + overlays).

## License

MIT — see [LICENSE](./LICENSE).
