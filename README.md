# Learning Center Reference

> Status: **draft / scaffolded.** A working reference implementation of a soccer-federation Learning Center — course delivery, coaching-education licensing, referee recertification, and safeguarding compliance — built to demonstrate full-stack engineering on a modern stack.

**🔗 Live demo:** _coming at milestone M2_ · **Demo login:** _provided with the demo_

> ⚠️ **Independent portfolio project — not affiliated with, endorsed by, or containing data from U.S. Soccer or any member organization. All names and records are fictional.**

A learner enrolls in courses, completes lessons and assessments, and earns verifiable certificates. An administrator sees who is compliant, who lapses in the next 30/60/90 days, and why. The interesting part is not the CRUD — it's that **eligibility to participate is a *derived* value**: it falls out of background-check expiry, safeguarding-training expiry, and (for referees) recertification status, and it flips automatically the moment any input lapses.

## Why this exists

This is a portfolio project. The domain rules are modeled on **publicly documented** youth-soccer coaching-education, refereeing, and safeguarding practice, generalized — **not** on any employer's internal system, and containing **no** real member, player, or proprietary data. All seed data is synthetic and labeled as such.

## Stack

| Layer | Tech | Why |
| --- | --- | --- |
| Web | TypeScript · Next.js (App Router) · Tailwind | Typed UI aligned to a small design-token set; responsive; `en` + `es` |
| API | Go · chi · OpenAPI | Typed HTTP service over a documented `/v1` contract |
| Data | PostgreSQL · golang-migrate | Relational core with versioned migrations |
| Auth | Auth0 (OIDC) | Roles: learner / instructor / admin |
| Content | Sanity CMS | Course/lesson content authored as structured documents, not hand-built CRUD |
| Progress | Append-only event log + projection | Event sourcing in one bounded context only |
| Delivery | Docker · AWS · GitHub Actions | Reproducible build; CI gates lint, types, tests, accessibility |
| Access | WCAG 2.1 AA target | Accessibility is tested in CI, not asserted |

## Quick start

```powershell
Copy-Item .env.example .env   # fill in local values; never commit .env
docker compose up db          # Postgres comes up first (localhost-only)
# api and web come online as they are built out — see docs/ and the milestones below
```

## Repository map

```text
docs/     Architecture, the domain model (read this first), accessibility
api/      Go service (chi + OpenAPI) over Postgres
web/      Next.js + TypeScript + Tailwind, en/es
studio/   Sanity schema for course content
db/       Synthetic seed data (labeled)
scripts/  One-command dev + local check helpers
```

## Milestones

- [ ] **M0 — Foundation:** repo scaffold, compose, CI green. *(in progress)*
- [ ] **M1 — Domain model:** `docs/domain-model.md` written and reviewed before any schema.
- [ ] **M2 — Vertical slice:** sign in → view course → complete lesson → progress persists.
- [ ] **M3 — Compliance:** coaching/referee/safeguarding rules, derived status, admin dashboard.
- [ ] **M4 — Release:** certificates, `es` locale, WCAG write-up, live demo, README polish.

## Disclaimer

Educational reference implementation. Not affiliated with, endorsed by, or containing data from any soccer federation or member organization.
