# api

> Status: **implemented** (safeguarding vertical slice). Go service (chi)
> over PostgreSQL via pgx.

- `cmd/server` — wiring: pool → migrations (`MIGRATE_ON_START=1`) → seed
  (`SEED_FILE`) → hardened `http.Server` with graceful shutdown
- `internal/safeguarding` — the pure eligibility rule (table-driven tests,
  inclusive-expiry boundary cases)
- `internal/httpapi` — router; id validation (400), 404/500 separation,
  DB-aware `/health`
- `internal/store` — pgx queries assembling a member's safeguarding inputs
- `internal/dbsetup` — small hand-rolled migration runner over the embedded
  `migrations/` files (one transaction per migration, `schema_migrations`)

Run the integration test locally:

```sh
docker compose up -d db
DATABASE_URL="postgres://lcr:change-me-locally@localhost:5432/lcr?sslmode=disable" \
  go test ./internal/store -run Integration -v
```

An OpenAPI spec is planned; the single `/v1` endpoint shape is documented in
the root README until then.
