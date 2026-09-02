# Recruiter demo deployment runbook

> **Prepared, not deployed.** This runbook stops before resource creation, billing, DNS,
> secrets, or account changes. Provider terms and prices must be checked again immediately
> before execution. The local OIDC overlay is the reproducible proof available today.

## What the public demo must prove

One URL should let a reviewer understand the fictional-data boundary, sign in through a real
OIDC provider, complete one learner action, sign out, switch to the administrator, and see the
derived compliance roster. It is not intended to model an entire federation platform.

## Reproducible pre-deployment gate

```powershell
docker compose -f compose.yml -f compose.oidc.yml up -d --build
cd web
$env:PLAYWRIGHT_OIDC = "1"
$env:PLAYWRIGHT_BASE_URL = "http://localhost:3000"
npm run test:auth
cd ..
./scripts/reset-demo.ps1
docker compose -f compose.yml -f compose.oidc.yml down -v
```

The OIDC fixture is a local test dependency, not an internet identity provider. It creates an
Authorization Code + PKCE redirect, signs access and ID tokens, validates nonce/state, uses
single-use codes, and offers only two fixed fictional subjects.

## Proposed service boundaries

| Component | Prepared shape | Required configuration |
| --- | --- | --- |
| Next.js web | platform with Node server support; root `web/` | variables in `web/.env.public.example` |
| Go API | long-running container from `api/Dockerfile` | variables in `api/.env.public.example` |
| PostgreSQL | managed PostgreSQL with required TLS | `DATABASE_URL`; synthetic seed only |
| Identity | OIDC regular web application + API audience | callback and logout URLs; client credentials |

No provider is part of the implementation claim until its actual URL passes the checks below.

## Security gate before creation

- Keep `DEPLOYMENT_ENV=public` and `WEB_DEPLOYMENT_ENV=public`; both processes then reject
  demo auth, HTTP origins, missing secrets, and the known local session placeholder.
- Register exactly `https://APP_HOST/api/auth/callback`; do not use wildcard redirects.
- Generate separate database, OIDC client, and session secrets. Put them only in provider
  secret stores. Never put them in a Vercel-prefixed public environment variable.
- Leave `TRUST_PROXY=0` unless the chosen API platform documents a trusted proxy that replaces
  the forwarding header. If enabled, verify client IP behavior from the deployed edge.
- Keep the seed fictional and the public member examples visibly identified as enumerable demo
  fixtures. A real member eligibility endpoint would require authentication and organization scope.
- Configure the smallest service size, spending notification, database retention, and teardown
  owner before publishing the URL. A notification is not a hard cost cap.

## Deployment order (requires later approval)

1. Create the PostgreSQL resource; apply the embedded migrations through a one-off API start.
2. Create the OIDC application/API and exact callback/logout allowlists.
3. Deploy the Go container with no public seed until configuration validation passes.
4. Load only `db/seed/seed.sql`, then run `/resetdemo` once with the explicit confirmation.
5. Deploy the Next.js server with the API and OIDC variables.
6. Run the read-only smoke command, then the complete learner/admin browser journey.
7. Record provider names, resource IDs, region, owner, spend policy, backup behavior, and exact
   deletion steps in a private operations note. Do not put secrets in that note.

```powershell
python scripts/smoke_public_demo.py `
  --app-url https://APP_HOST `
  --api-url https://API_HOST `
  --issuer-host YOUR_TENANT.example.com
```

## Publish and teardown rules

Only add a homepage URL after desktop/mobile, keyboard, authorization-negative, reset, and cold
start checks pass. Remove the link if the service becomes slow, unsafe, over budget, or stale.
Delete the web/API/database/identity resources and revoke their secrets when the demo is retired.
