# scripts

Operational helpers for the local demo.

- `reset-demo.ps1` — clears mutable enrollment/progress state for the fictional demo
  association by running the scoped `/resetdemo` command inside the API container.
- `smoke_public_demo.py` — read-only post-deploy smoke checks (API readiness, landing page,
  security headers, and the OIDC login redirect) for a configured recruiter deployment. It
  creates nothing and mutates nothing.
