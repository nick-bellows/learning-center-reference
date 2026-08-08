# api

> Status: **planned.** Go service (chi + OpenAPI) over PostgreSQL.

Structure lands in Step 2, after the domain model is settled:

```text
cmd/server/       entrypoint (wiring only)
api/openapi.yaml  the /v1 contract (source of truth)
migrations/       golang-migrate .up/.down SQL
internal/
  config/         env parsing, fail-fast on missing vars
  platform/       shared errors, IDs, pagination, clock
  http/           chi router, middleware, thin handlers
  auth/           OIDC token verification (Auth0 JWKS)
  store/          pgx pool + queries
  learning/       courses / lessons / assessments
  coaching/       licenses, prerequisites, renewal
  refereeing/     recertification windows
  safeguarding/   background check + training -> derived status
  progress/       event-sourced: append log + projection
```

Domain logic lives in the context packages (unit-testable without HTTP); handlers stay thin.
