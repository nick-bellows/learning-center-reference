# Roadmap

Last verified: 2026-09-02

## Handoff snapshot

| Field | Current state |
| --- | --- |
| Lifecycle | `ACTIVE` - working portfolio vertical slice |
| Portfolio role | Primary software-engineering and soccer-domain evidence |
| Public claim | Local Docker and CI-verified learner/admin workflow; no hosted deployment claim |
| Data boundary | Fictional federation, identities, courses, credentials, and eligibility facts only |
| Technical next step | Assessment-to-credential issuance |
| Presentation next step | Execute the prepared secure recruiter-demo runbook after approval |

The implemented slice is already meaningful: identity verification, database-backed roles, course listing, idempotent enrollment, ordered lesson completion, transactional progress events/projection, learner dashboard, and administrator compliance view. Read `README.md` and `docs/INTERVIEW_GUIDE.md` before changing scope.

## Completed milestone - local recruiter demo boundary

Goal: let a reviewer experience the existing workflow without weakening the repository's authentication, privacy, or evidence claims.

Repository-side work is complete: browser OIDC redirect/callback/logout, encrypted secure-session
handling, database role resolution, explicit identity switching, scoped reset, request/body limits,
safe errors, guided code tour, local OIDC fixture, and browser E2E coverage. External account
creation, hosted-provider verification, and public deployment remain deliberately gated.

### Work

1. Connect the existing redirect, callback, logout, and secure session boundary to an approved hosted OIDC provider. Keep the current Go verifier.
2. Retain the local standards-compliant OIDC fixture as the reproducible CI boundary.
3. Define two fictional demo journeys: learner and administrator. Make role switching an explicit sign-out/sign-in action, not a request parameter or client-controlled role.
4. Choose a reset model before exposing writes: per-session seeded tenant, scheduled reset, or tightly bounded shared data. Document the trade-off and prove one reset path.
5. Add public-demo controls: secure cookies, restrictive redirect allowlist, environment validation, request/body limits, rate limiting, safe error pages, no token/PII logging, and a visible portfolio disclaimer.
6. Add a short guided landing page that links each UI step to the underlying API, migration, test, and design decision. Do not create placeholder features for the tour.
7. Add deployment smoke checks for `/health`, learner login, enrollment retry, ordered completion, persisted dashboard state, administrator authorization, and reset behavior.

### Acceptance criteria

- A logged-out reviewer can understand the problem, fictional-data boundary, and two demo paths before signing in.
- Local authentication uses a real OIDC redirect/session; `AUTH_MODE=demo` cannot start in a public deployment configuration. Hosted verification remains pending.
- Learner and administrator permissions are resolved server-side and covered by negative authorization tests.
- All mutations affect synthetic data and can be reset without manual database editing.
- The public URL opens promptly on desktop/mobile, has a keyboard-complete happy path, and exposes no secrets or stack traces.
- CI, a fresh local Compose run, and post-deploy smoke checks pass. Each result is recorded at the boundary it proves.
- Monthly spend and teardown steps are documented before billing is enabled.

## Deployment decision

Recommended preview shape, pending explicit account and cost approval:

- `web/`: Vercel Hobby, while this remains a personal non-commercial portfolio project.
- `api/`: one small Railway service with billing alerts and an approved policy ceiling; expected planning cost is approximately $5/month, rechecked before creation.
- PostgreSQL: Neon Free, synthetic seed only, with scale-to-zero behavior included in the smoke test.
- Identity: Auth0 Free development tenant.

A $0 alternative is Vercel + Render Free + Neon + Auth0. It is not preferred for the primary recruiter link because an idle Render service can take about a minute to wake. Replit is not the primary plan: adapting this existing multi-service architecture to a platform-specific all-in-one project adds work without strengthening the implementation evidence.

No deployment has been created or validated. Current provider prices, terms, commands, and limits must be rechecked immediately before use. `docs/decisions/0004-hosting-vercel-fly-auth0-sanity.md` is historical; a new decision record must supersede it before deployment because Fly.io no longer offers the originally assumed permanent free path.

## Next engineering milestone - credential issuance

After the demo boundary is secure and stable, implement one bounded workflow:

```text
learner completes lessons -> assessment attempt -> passing result
-> credential issued with effective/expiry dates -> eligibility recalculated
-> administrator roster and audit history update
```

Eligibility remains derived from underlying facts. Event sourcing remains confined to progress unless a new invariant justifies extending it. Sanity remains an integration seam until course-content editing is needed by this slice; do not provision it for a keyword.

## Stop conditions

- Do not deploy `AUTH_MODE=demo`, seed a real person, or expose unrestricted administrator writes.
- Do not claim Auth0, Vercel, Railway, Neon, Sanity, AWS, or public deployment until the corresponding path has actually been exercised and retained evidence exists.
- Do not add organization tenancy, uploads, notifications, Kubernetes, Kafka, or broad CQRS before the credential slice is complete.
- Do not use U.S. Soccer branding or imply this models a private organizational system.

## Verification before changing status

Use the commands in `README.md`. At minimum, verify Go vet/tests, real-Postgres integration, web lint/build, Compose end to end, axe automation, secret scan, and the post-deploy smoke path. Automated axe results are not a WCAG conformance claim; retain manual keyboard, zoom, contrast, and screen-reader notes separately.
