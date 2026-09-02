# 5. Operate one bounded recruiter deployment

- **Status:** accepted as a plan; execution requires account and cost approval
- **Date:** 2026-09-02

## Context

The working local slice now carries more evidence than a broad architecture scaffold. A public demo would add value only if it proves the remaining browser authentication/session boundary and lets a reviewer complete the real learner/admin workflow. Hosting every portfolio service, or provisioning every technology named in a job description, would create cost and security obligations without comparable evidence.

ADR 0004 selected Fly.io plus several external systems before deployment. It was never executed. Fly.io now describes usage-based pricing and requires a payment method, so the assumed free path is not a sound default. Current platform terms and prices remain external facts and must be rechecked before creating resources.

## Decision

Prepare one resettable, synthetic Learning Center recruiter demo:

- deploy the Next.js frontend from `web/` to Vercel Hobby while the project remains personal and non-commercial;
- deploy the Go container from `api/` to one small Railway service with billing alerts and a documented monthly policy ceiling;
- use Neon Free PostgreSQL for synthetic demonstration state;
- use an Auth0 Free development tenant for browser redirect, callback, logout, and API tokens; and
- keep Sanity as an unprovisioned content integration seam until editable course content is part of a completed workflow.

The choice is intentionally boring: use managed services for the external boundaries the project needs to prove, while keeping application behavior and migrations in this repository. Deployment is not evidence until the hosted smoke path has actually run.

Official pages checked for this decision:

- <https://vercel.com/pricing>
- <https://railway.com/pricing>
- <https://neon.com/pricing>
- <https://auth0.com/pricing>
- <https://fly.io/docs/about/pricing/>

## Consequences

- The expected paid planning cost is one small Railway subscription; the other selected tiers currently offer free entry points. A policy ceiling is not a claim that the provider enforces a hard cap. No billing or account creation is authorized by this ADR.
- The public configuration must refuse `AUTH_MODE=demo` and must use secure server-side sessions, restrictive redirects, request limits, rate limiting, safe errors, and token/PII-free logs.
- Shared synthetic writes need an automated reset or per-session isolation before the URL is published.
- The project will retain Compose as the self-contained full-system path and the local OIDC fixture as the reproducible authentication proof.
- Render Free remains a $0 preview alternative, but its documented idle spin-down and slow first request make it a weaker primary recruiter link.
- Replit remains a prototyping option, not a second deployment architecture for this repository.
- AWS remains a documented architecture option, not a deployment claim or interview-time spending requirement.

## Exit and teardown

Before publishing, record resource names, environment variables, owners, cost alerts, reset schedule, smoke command, and deletion steps in a deployment runbook. If the link cannot remain secure, responsive, and within the approved ceiling, remove it from the README and tear the resources down rather than leaving a degraded recruiter path.
