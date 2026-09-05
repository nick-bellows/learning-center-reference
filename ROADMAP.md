# Roadmap

Last verified: 2026-09-04

## Handoff snapshot

| Field | Current state |
| --- | --- |
| Lifecycle | `ACTIVE` — portfolio-ready for review; product completion planned |
| Portfolio role | Primary software-engineering and soccer-domain evidence |
| Public claim | Local Docker and CI-verified learner/admin workflow; no hosted deployment claim |
| Data boundary | Fictional federation, identities, courses, credentials, and eligibility facts only |
| License | MIT (`LICENSE`) |
| CI | Five green jobs on `main`: `api`, `web`, `e2e`, `oidc-e2e`, `secret-scan` |
| External review | Two independent reviews (2026-09-04) returned **ADVANCE**; their in-scope findings are addressed |
| Technical next step | Assessment-to-credential issuance (see Lane A) |
| Presentation next step | Recruiter screencast (Lane A); hosted demo only after account/cost approval (Lane B) |

The implemented slice is meaningful: identity verification, database-backed roles, course
listing, idempotent enrollment, ordered lesson completion, transactional progress
events/projection, learner dashboard, administrator compliance view, a service-to-service
credentials contract, effective-date-correct eligibility, and route-pattern request logging.
Read `README.md` and `docs/INTERVIEW_GUIDE.md` before changing scope.

## What "complete" means

The project has two completion tiers. Tier 1 is done; Tier 2 is the remaining plan.

- **Tier 1 — portfolio-complete (reached).** Honest, hardened, reviewable evidence: the
  vertical slice runs cold, CI is green including a secret scan, claims match the code, and
  both external reviews said ADVANCE. Nothing below is required to send this to an employer.
- **Tier 2 — product-complete (planned).** The credential lifecycle is connected end to end,
  operational gaps (projection rebuild, least-privilege database role, negative-path web
  tests) are closed, and a resettable synthetic demo is optionally hosted.

The two lanes below carry Tier 2. Lane A is executable now with no accounts, spending, or
secrets. Lane B needs your decision, an account, a cost approval, or a repository setting.

## Lane A — Claude Code can execute now

Reversible code, docs, tests, and CI only. No external accounts, no spending, no secrets.
Each item ships as its own PR with CI green before merge; you review the PR. Ordered by
hiring value.

- [ ] **A1 · Recruiter screencast in the README (S–M).** Record the local OIDC learner →
  admin journey and code tour, embed an optimized GIF/short clip in the README. Gives a
  recruiter the strongest part of the project without a hosted URL. Both external reviews
  named this the highest-value recruiter-access improvement. *(You review the recorded asset
  in the PR.)*
- [ ] **A2 · Credential-issuance milestone (L, multi-day).** Implement the bounded workflow:

  ```text
  learner completes lessons -> assessment attempt -> passing result
  -> credential issued with effective/expiry dates -> eligibility recalculated
  -> administrator roster and audit history update
  ```

  All synthetic; no accounts. Eligibility stays derived; event sourcing stays confined to
  progress unless a new invariant justifies extending it. This turns the homepage's honest
  "next milestone" note into a demonstrated lifecycle and connects the two workflows.
  **Gated on decision B1** (scope/priority go-ahead) before the build starts.
- [ ] **A3 · Projection rebuild/reconcile command (S–M).** Add a command that rebuilds
  `enrollment_progress` from `progress_event`, with a test, and reference it in the interview
  guide's projection-corruption row. Closes a named operational gap.
- [ ] **A4 · Least-privilege database role (M).** Add a migration creating a restricted
  runtime role (DML only); run migrations/seed as owner and runtime queries as the limited
  role via a second connection string. Addresses the single-owner-connection finding while
  staying local/synthetic.
- [ ] **A5 · Negative-path browser auth tests (M).** Cover tampered/expired session cookies,
  callback state mismatch, provider `error=` responses, expired transactions, and hostile
  `returnTo`, against the local OIDC fixture. The custom session/callback code is currently
  proven only on the happy path.
- [ ] **A6 · Operational-path unit tests (S–M).** Cover `dbsetup` (migration re-run
  idempotency and rollback-on-failure) and `demoreset`/`RESET_CONFIRM`. These paths are
  currently exercised only indirectly by the Compose e2e job.
- [ ] **A7 · OpenAPI ↔ router conformance (M).** Assert handler responses against
  `openapi.yaml` (today `openapi_test.go` validates the document, not the handlers), document
  the `429/500/503` responses the API can emit, and make the concurrency throttle return JSON
  so every error shares one contract.
- [ ] **A8 · CI action pinning (S).** SHA-pin the GitHub Actions and move off the Node 20
  action runtime to clear the deprecation warnings. Reproducible, supply-chain-clean CI.
- [ ] **A9 · Manual-accessibility checklist scaffold (S).** Add the keyboard / screen-reader /
  zoom / contrast checklist document so the human pass in B6 has a place to record findings.
  Automated axe is not a conformance claim.

## Lane B — needs your input, approval, or account

- [ ] **B1 · Decide whether to build the credential-issuance milestone now.** Scope/priority
  call. It is the largest remaining engineering item (A2) and the centerpiece of "product
  complete," but it is optional for a portfolio that already reviews as ADVANCE. Say go and
  Claude Code builds it; say hold and the honest "next milestone" boundary stays.
- [ ] **B2 · Approve and provision a hosted recruiter demo.** Requires creating accounts,
  approving a small monthly ceiling, and supplying secrets — none of which Claude Code can do.
  Recommended shape is below. Once accounts and secrets exist, Claude Code can wire the
  deployment config and run the smoke checks; it cannot create accounts, enable billing, or
  hold secrets. Runbook: `docs/deploy-recruiter-demo.md`.
- [ ] **B3 · Create a hosted OIDC tenant (e.g. Auth0 Free) and verify the browser flow
  against it.** The redirect/callback/session/logout path is proven only against the local
  fixture today; hosted-provider interoperability stays unclaimed until a real tenant is
  configured and its negative and end-to-end tests pass. Needs an external account.
- [ ] **B4 · Enable Dependabot security updates and vulnerability alerts (repository
  setting).** `.github/dependabot.yml` already turns on version updates; security updates and
  alerts are a free toggle in repository settings that only the owner can flip.
- [ ] **B5 · Publish the demo link (only after B2/B3 pass).** Add the homepage/README live URL
  and, if desired, refresh the profile pin. Owner action and judgment; remove the link if the
  demo becomes slow, unsafe, or over budget.
- [ ] **B6 · Run the manual accessibility review.** Keyboard, screen-reader, zoom, and
  contrast testing is human work; do it (using the A9 checklist) before making any WCAG
  conformance claim.
- [ ] **B7 · Provision a headless CMS (deferred).** Only if editable course content becomes
  part of a slice. `content_ref` is the integration seam; do not provision for a keyword.

## Deployment decision (Lane B detail)

Recommended preview shape, pending explicit account and cost approval:

- `web/`: Vercel Hobby, while this remains a personal non-commercial portfolio project.
- `api/`: one small Railway service with billing alerts and an approved policy ceiling;
  expected planning cost is approximately $5/month, rechecked before creation.
- PostgreSQL: Neon Free, synthetic seed only, with scale-to-zero behavior included in the
  smoke test.
- Identity: Auth0 Free development tenant.

A $0 alternative is Vercel + Render Free + Neon + Auth0. It is not preferred for the primary
recruiter link because an idle Render service can take about a minute to wake. No deployment
has been created or validated. Current provider prices, terms, commands, and limits must be
rechecked immediately before use. `docs/decisions/0004-hosting-vercel-fly-auth0-sanity.md` is
historical and superseded by ADR 0005; a new decision record must supersede ADR 0005 before
deployment if the chosen providers differ.

## Credentials contract v1 status

Provider side of `learning-center.credentials.v1`, the read contract consumed by the fictional
federation's member-services lab: `GET /v1/members/{subject}/credentials`, authenticated by a
service token carrying scope `credentials:read` rather than by a person. This exposes existing
credential facts and derived eligibility; nothing is issued.

- Status: **validated** for the handler, the scope middleware, OIDC `scope`/`scp` parsing, the
  by-subject store query (real-Postgres integration test), the fixture-shape contract test
  against the consumer's reference responses, and the Compose end-to-end curls in CI using the
  synthetic demo service token.
- Status: **planned** for interoperability against the consumer stack with a shared identity
  provider. No shared provider or client-credentials tenant exists, so only the local demo
  token path has been exercised end to end. *(Closed by B3.)*

## Stop conditions

- Do not deploy `AUTH_MODE=demo`, seed a real person, or expose unrestricted administrator writes.
- Do not claim Auth0, Vercel, Railway, Neon, Sanity, AWS, or public deployment until the
  corresponding path has actually been exercised and retained evidence exists.
- Do not add organization tenancy, uploads, notifications, Kubernetes, Kafka, or broad CQRS
  before the credential slice is complete.
- Do not use U.S. Soccer branding or imply this models a private organizational system.

## Verification before changing status

Use the commands in `README.md`. At minimum, verify Go vet/tests, real-Postgres integration,
web lint/build, Compose end to end, axe automation, secret scan, and the post-deploy smoke
path. Automated axe results are not a WCAG conformance claim; retain manual keyboard, zoom,
contrast, and screen-reader notes separately (see B6).
