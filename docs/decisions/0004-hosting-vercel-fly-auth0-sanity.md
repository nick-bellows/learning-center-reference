# 4. Host on Vercel + Fly.io + Auth0 + Sanity (max stack fidelity)

- **Status:** accepted
- **Date:** 2026-08-07

## Context

The project needs a live, always-on, low-cost demo. Two goals were in tension:
minimizing the number of services, versus matching U.S. Soccer's exact stack (a portfolio
whose whole purpose is to mirror their platform). A single-host option (Railway: web + API
+ Postgres in one project, ~$5/mo) was evaluated and would use fewer accounts.

## Decision

Prioritize **stack fidelity**: deploy the Next.js web app on **Vercel**, the Go API and
Postgres on **Fly.io**, authenticate via **Auth0** (OIDC), and store course content in
**Sanity**. Keep the committed Dockerfiles and a `docs/deploy-aws.md` note so the app is
truthfully AWS-deployable; stand up a throwaway **AWS** (App Runner + RDS) deployment near
interview time for the exact-stack talking point.

Rationale: every product maps to a JD-named technology (Vercel→Next.js, Fly→containers,
Auth0, Sanity, AWS). For this role, "runs on their stack" is worth more than "fewest
accounts."

## Consequences

- Five external accounts to manage instead of one or two — accepted trade-off.
- Strong, literal alignment with the posting's technology list.
- **No Kubernetes:** containers on Fly.io / App Runner satisfy the intent; k8s is
  disproportionate effort for a demo.
- The hosted demo must not impersonate U.S. Soccer: neutral branding, synthetic data, and a
  visible "not affiliated" disclaimer.
