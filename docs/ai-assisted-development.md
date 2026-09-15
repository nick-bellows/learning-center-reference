# AI-assisted development

Factual note, kept short on purpose.

**What was used.** This repository was built by the author with Claude Code
(Anthropic) as a pair programmer, working from a written roadmap
(`ROADMAP.md`) whose scope, lanes, and hold decisions the author set. AI
assistance covered drafting Go and TypeScript code, tests, CI workflow steps,
ADRs, and documentation. No language model appears anywhere in the runtime
path; the application is a Go API, a Next.js front end, PostgreSQL, and an
OIDC boundary.

**What the author owns.** Scope and the decisions recorded in
`docs/decisions/`; the claim boundary that the README and roadmap repeat
(fictional federation, synthetic data, local Docker and CI evidence only, no
hosted deployment claim); the hold on the credential-issuance milestone (A2,
decision B1); review of every pull request before merge; the response to the
external code reviews; and every statement made about the project. Claude
Code drafted code and prose under that direction. Drafts were read, run, and
edited before they were committed, not accepted wholesale.

**How the work is recorded.** At the local `main` this note describes
(commit `89cd3f8`, 2026-09-12):

- 50 commits, of which 27 carry a `Co-Authored-By: Claude ...` trailer naming
  the model that drafted the change. The rest are the 17 merge commits and
  six early feature and documentation commits made without a trailer.
- 16 pull requests merged into `main` by merge commit (#1–#3, #15–#16,
  #18–#24, #27–#30), each reviewed by the author. The Lane A work in
  `ROADMAP.md` shipped as one PR per item with CI green before merge.
- Two independent external code reviews on 2026-09-04 returned ADVANCE and a
  closing review on 2026-09-12 found no blocking defects; their findings and
  the PRs that addressed them are recorded in `ROADMAP.md`.

**How correctness is established independently of authorship.** Nothing in
this repository asks to be trusted on the author's or the tool's word:

- The Go API has 56 `Test` functions across 18 files, run with `go vet`, the
  race detector, and `govulncheck` in CI. Integration tests run against a real
  PostgreSQL service, including the migration runner, the DML-only runtime
  role, the projection rebuild, and the scoped demo reset.
- `openapi_conformance_test.go` checks every handler status and body,
  including `429`, `500`, and `503`, against `api/openapi.yaml`, so the
  contract and the code cannot drift apart silently.
- The browser authentication boundary is tested on its failure paths:
  `web/tests/auth-negative.spec.ts` covers 16 negative cases (tampered,
  forged, expired, and token-less session cookies; callback state, provider
  error, forged code, expired transaction, and replay; hostile `returnTo`
  values) against a standards-based local OIDC fixture, alongside the happy
  path in `web/tests/auth.spec.ts`.
- CI (`.github/workflows/ci.yml`) runs five jobs on every push: `api`, `web`,
  `e2e` (the README quick start executed verbatim, then the learner and admin
  workflow, projection drift repair, and axe WCAG A/AA checks over the five
  routes and the 404 page), `oidc-e2e` (the happy and negative browser suites,
  a read-only smoke boundary, and axe over the OIDC-only page states at
  desktop and phone width), and `secret-scan` (gitleaks over the full
  history). Every third-party action is pinned to a commit SHA.
- The README walkthrough GIF is recorded by `scripts/record-screencast.ps1`
  from `web/tests/screencast.spec.ts` against the local Compose stack, so the
  demonstration is reproducible from the repository.

**What this does not claim.** No productivity metric is offered; counting
AI-assisted lines or estimating time saved would be unfalsifiable here. The
automated accessibility checks are not a WCAG conformance claim, and the
manual review in `docs/accessibility-manual-review.md` has not been run. The
author's working claim is not "written unaided" but "understood, verified,
and defensible line by line", which is what an interview can test directly.
