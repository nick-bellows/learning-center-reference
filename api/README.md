# API

Go/chi service over PostgreSQL via pgx.

- `cmd/server` wires the pool, embedded migrations, optional local seed, auth mode,
  structured request logger, timeouts, and graceful shutdown.
- `internal/authn` verifies OIDC tokens or selects the explicit synthetic demo adapter.
- `internal/httpapi` owns routes, identity/role middleware, status mapping, and the
  semantically validated `openapi.yaml` contract.
- `internal/store` resolves database roles, executes the learning transaction, builds
  projections, and loads safeguarding inputs through parameterized SQL.
- `internal/safeguarding` is a pure derived-eligibility rule with boundary tests.
- `internal/credentials` builds the `learning-center.credentials.v1` response for the
  service-to-service route; eligibility and every validity flag come from `internal/safeguarding`.
- `migrations/0005_progress.up.sql` contains the bounded append-only progress log and
  dashboard projection.
- `internal/projection` and `cmd/reconcileprogress` compare that projection with the event
  log and rebuild drifted rows (`--apply`); the dry run exits 3 when drift exists.
- `internal/testdb` gives integration tests a private, migrated throwaway database so test
  packages running in parallel never share rows.

Run all tests, including real-PostgreSQL integration tests:

```sh
docker compose up -d db
cd api
go vet ./...
DATABASE_URL="postgres://lcr:change-me-locally@localhost:5432/lcr?sslmode=disable" go test ./...
```

Database privilege scopes: `DATABASE_URL` is the owner and is used only for migrations, the
seed, and the operator commands (`/resetdemo`, `/reconcileprogress`). Request handling runs
with the DML-only `lcr_runtime` role from migration `0007` when either knob is set:

- `DB_RUNTIME_ROLE=lcr_runtime` — every pooled connection runs `SET ROLE` after connecting
  (the Compose default; no second password to manage).
- `RUNTIME_DATABASE_URL` — a separate login role created `IN ROLE lcr_runtime`, the shape
  for a hosted database. Both paths are covered by `internal/store/runtime_role_test.go`.

`DEPLOYMENT_ENV=public` refuses to start unless one of them is set.

Protected routes fail closed unless `AUTH_MODE=demo` or `AUTH_MODE=oidc` is selected.
OIDC mode additionally requires `OIDC_ISSUER_URL` and `OIDC_AUDIENCE`; provider discovery
failure stops startup rather than downgrading authentication.
