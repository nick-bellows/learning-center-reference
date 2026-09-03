# Learning Center Reference

> **Status: working portfolio vertical slice.** A synthetic learner can authenticate,
> resolve a database role, browse a course, enroll, complete ordered lessons, and see
> persisted progress. A synthetic administrator can inspect eligibility derived live
> from safeguarding and credential records. A local OIDC fixture now proves the browser
> redirect/session/logout path. External-provider login, CMS content, certificates, and
> cloud deployment remain explicitly unclaimed.

> **Independent portfolio project — not affiliated with, endorsed by, or containing
> data from U.S. Soccer or any member organization. Every name and record is fictional.**

This reference implementation models one hard product problem rather than a broad mock:
education progress and participation eligibility have to remain traceable as roles,
credentials, expirations, and holds change.

![Learner dashboard showing persisted course progress](docs/assets/learner-dashboard.png)

## The implemented workflow

```text
verified identity → PostgreSQL role → published course → idempotent enrollment
                  → ordered lesson completion events → dashboard projection

admin identity → PostgreSQL admin role → current credential facts
               → derived eligibility → compliance roster
```

- The API validates OIDC signature, issuer, audience, and expiry in `AUTH_MODE=oidc`.
- Application roles come from PostgreSQL—not token claims or request parameters.
- The default local stack uses an explicit demo verifier; the optional OIDC overlay exercises
  Authorization Code + PKCE, signed tokens, callback/session handling, and explicit role switching
  with two fixed fictional identities. Neither local mode is suitable for internet exposure.
- Enrollment and lesson-completion retries are idempotent.
- Sequential courses reject an out-of-order completion with `409 Conflict`.
- An append-only `progress_event` record and its `enrollment_progress` projection update
  in one transaction. Event sourcing is deliberately confined to learner progress.
- Participation eligibility is never stored. It is recalculated from background-check,
  SafeSport, role-credential, and disciplinary-hold facts on each request.

![Administrator compliance view with fictional members](docs/assets/admin-compliance.png)

## Architecture

```mermaid
flowchart LR
    Browser[Browser] --> Web[Next.js / TypeScript\nServer Components + Actions]
    Browser -->|Authorization Code + PKCE| IdP[OIDC provider]
    IdP -->|Callback + signed tokens| Web
    Web -->|Bearer token, server side| API[Go / chi API]
    API -->|Discovery + JWKS validation| IdP
    API -->|Resolve subject and roles| PG[(PostgreSQL)]
    API -->|Append completion| Events[(progress_event)]
    Events -->|Same transaction| Projection[(enrollment_progress)]
    API -->|Derived safeguarding status| Web
```

The default local demo substitutes fixed synthetic subjects so a clean clone needs no account
or secret. `compose.oidc.yml` supplies a standards-based local provider and proves the complete
browser boundary without pretending it is Auth0. Hosted-provider interoperability is not claimed.

## Quick start

Requirements: Docker with Compose.

```sh
git clone https://github.com/nick-bellows/learning-center-reference
cd learning-center-reference
docker compose up --build
```

Open:

- `http://localhost:3000/learn` — learner enrollment and progress
- `http://localhost:3000/admin/compliance` — role-protected compliance view
- `http://localhost:3000/members` — three focused eligibility-rule examples
- `http://localhost:8080/health` — readiness, including a PostgreSQL ping

The API embeds and applies migrations, then loads an idempotent synthetic seed. Ports bind
to `127.0.0.1`; Compose does not expose the demo outside the local machine.

To exercise real redirect/callback/session behavior locally, use the OIDC overlay:

```sh
docker compose -f compose.yml -f compose.oidc.yml up --build
```

Open `/learn`, sign in as Alex Coach, then sign out and choose Casey Admin for the administrator
path. The guided landing page links each UI behavior to its implementation and tests. Run
`./scripts/reset-demo.ps1` from PowerShell to clear mutable fictional enrollment/progress state.

## What is implemented

| Capability | Evidence |
| --- | --- |
| Go REST API and contract | `api/internal/httpapi`, semantically validated `api/openapi.yaml` |
| Authentication and RBAC | OIDC verifier, Authorization Code + PKCE web session, local provider fixture, explicit demo adapter; roles resolved by `internal/store` |
| Course workflow | Published catalog, idempotent enrollment, sequential lesson completion, learner dashboard |
| PostgreSQL state | Five versioned migrations, embedded transactional runner, idempotent synthetic seed |
| Bounded event sourcing | Immutable completion events and transactional progress projection in migration `0005` |
| Eligibility | Pure, boundary-tested Go rule derived from expiring facts and active holds |
| Credentials contract v1 | Service-token `GET /v1/members/{subject}/credentials` implementing `learning-center.credentials.v1` (scope `credentials:read`); shape pinned by the consumer's fixtures under `api/testdata/contracts` |
| Administrator workflow | Role-protected compliance roster with current reasons and earliest credential expiry |
| Web and accessibility | Next.js 16, TypeScript, semantic UI, keyboard focus/reduced motion, automated axe WCAG A/AA gate |
| Operations | JSON request logs without tokens/PII, DB-aware health check, timeouts, graceful shutdown, request/body limits, scoped demo reset |
| Delivery | Non-root container images; GitHub Actions for vet/race/tests, real-Postgres integration, vulnerability checks, web build, Compose/OIDC e2e, and accessibility |

## Verification

API unit and integration tests (the integration tests require the local database):

```sh
docker compose up -d db
cd api
go vet ./...
DATABASE_URL="postgres://lcr:change-me-locally@localhost:5432/lcr?sslmode=disable" go test ./...
```

Web checks:

```sh
cd web
npm ci
npm run lint
npm run build
npx playwright install chromium
PLAYWRIGHT_BASE_URL=http://localhost:3000 npm run test:a11y
```

CI also starts the complete Compose stack and exercises authentication failures, role
boundaries, enrollment retry behavior, ordered progress, dashboard persistence, the admin
view, and all four rendered routes. See `.github/workflows/ci.yml`.

## Engineering decisions

- [Go for the API](docs/decisions/0002-go-for-the-api.md)
- [Confine event sourcing to learner progress](docs/decisions/0003-event-sourcing-scope.md)
- [Historical hosting option](docs/decisions/0004-hosting-vercel-fly-auth0-sanity.md)
- [One bounded recruiter deployment](docs/decisions/0005-one-bounded-recruiter-deployment.md)
- [Local OIDC and public-demo boundary](docs/decisions/0006-local-oidc-and-public-demo-boundary.md)
- [Prepared recruiter-demo runbook](docs/deploy-recruiter-demo.md)
- [Domain model and assumptions](docs/domain-model.md)
- [Interview guide](docs/INTERVIEW_GUIDE.md)

## Security and privacy boundaries

- The OIDC verifier fails closed; unsupported or missing auth configuration cannot expose
  protected routes.
- Ownership is checked before a learner can append progress to an enrollment.
- Logs record request metadata, never bearer tokens or member details.
- UUIDs are validated before database casts. SQL uses pgx parameters throughout.
- The public eligibility example contains fixed synthetic records only. A real deployment
  would protect member-level eligibility and apply organization-level authorization.
- `.env` files are ignored. No real member data, provider secrets, or cloud credentials are
  required or committed.
- Public-mode startup rejects demo auth, HTTP application/provider URLs, missing confidential
  client/session secrets, and the known local placeholder.

## Deliberate limitations

- No hosted demo or paid cloud resources have been created.
- Browser OIDC login/callback/session/logout is proven against the local fixture. No Auth0 tenant
  or other hosted provider has been created or verified.
- `content_ref` is the headless-CMS integration seam; no Sanity project is provisioned.
- Assessment attempts, passing scores, credentials issued from course completion,
  certificates, i18n, uploads, notifications, and organization tenancy are not implemented.
- Published course content is treated as immutable after learners enroll; schema evolution
  for an in-progress course would require a versioning policy.
- Automated axe checks catch only a subset of accessibility issues; manual keyboard,
  screen-reader, zoom, and contrast review remains necessary before a WCAG conformance claim.
- The compliance query favors readable code over large-roster optimization and would need
  pagination and a set-based query at production scale.

## Repository map

```text
api/      Go service, OpenAPI contract, OIDC adapter, domain/store code, migrations
web/      Next.js/TypeScript learner and administrator experiences, axe tests
db/       Explicitly synthetic, idempotent local demonstration data
docs/     Domain model, ADRs, screenshots, deployment notes, interview guide
```

## Next bounded milestone

Add assessment attempts and a passing rule, then issue an expiring credential that changes the
derived eligibility roster and audit history. A hosted recruiter demo is separately prepared but
remains gated on explicit account and spending approval.

No open-source license has been selected. Choose one before inviting third-party reuse.
