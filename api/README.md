# API

Go/chi service over PostgreSQL via pgx.

- `cmd/server` wires the pool, embedded migrations, optional local seed, auth mode,
  structured request logger, timeouts, and graceful shutdown.
- `internal/authn` verifies OIDC tokens or selects the explicit synthetic demo adapter.
- `internal/httpapi` owns routes, identity/role middleware, status mapping, the JSON
  concurrency limit, and the `openapi.yaml` contract: `openapi_test.go` validates the
  document and `openapi_conformance_test.go` validates each handler's responses (every
  status a route can emit, including 429/500/503) against it.
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
  packages running in parallel never share rows. Every integration test in this module uses
  it, including the store's; the database named by `DATABASE_URL` is only the server they
  create their copies on.
- `cmd/oidcfixture` (image `Dockerfile.oidc`) is the local standards-based OpenID provider
  behind `compose.oidc.yml`: Authorization Code + PKCE, signed tokens, discovery and JWKS,
  two fixed fictional subjects. It is a test dependency, never an internet identity provider.
- `migrations/*.down.sql` document how each migration is reversed. Nothing applies them
  automatically: `embed.go` embeds only the `*.up.sql` files and the runner is forward-only.

Error contract: every error response, including an unknown path (404), a wrong method
(405), and a recovered handler panic (500), is a JSON `{"error": ...}` body. Each response
carries a server-generated `X-Request-Id` that matches the `request_id` field of its
structured log line; an inbound `X-Request-Id` is ignored so clients cannot plant text in
the logs. Rejected bearer tokens are logged with the verifier's reason (never the token)
so an identity-provider outage is distinguishable from bad credentials.

Throttling knobs: `RATE_LIMIT_PER_MINUTE` (default 120; `0` disables the per-client
limit) and `MAX_CONCURRENT_REQUESTS` (default 64; `0` selects the default). Both answer
with a JSON 429 and `Retry-After`.

`GET /v1/members/{id}/eligibility` is deliberately unauthenticated in this reference
implementation: it is the fixed synthetic-member example the `/members` page renders, member
ids are unguessable UUIDs, and it exposes only the derived status and its reason. The
compliance roster, which lists members by name, is admin-only. A real deployment would put
member-level eligibility behind organization-level authorization (see `README.md` at the
repository root, "Security and privacy boundaries").

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
