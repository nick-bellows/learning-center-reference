# scripts

Operational helpers for the local demo.

- `reset-demo.ps1` — clears mutable enrollment/progress state for the fictional demo
  association by running the scoped `/resetdemo` command inside the API container.
- `reconcile-progress.ps1` — compares the `enrollment_progress` projection with the
  append-only `progress_event` log inside the API container and prints a JSON drift report
  (exit code 3 on drift). `-Apply` rebuilds the drifted rows in one transaction.
- `record-screencast.ps1` — resets the demo, drives the learner → administrator journey through
  the local OIDC overlay with Playwright (`web/tests/screencast.spec.ts`, video on), and renders
  the capture to `docs/assets/recruiter-walkthrough.gif` with ffmpeg (two-pass palette). Needs
  the OIDC stack running and `ffmpeg` on PATH; `-BaseUrl`, `-Width`, `-Fps`, `-NoReset` tune it.
- `smoke_public_demo.py` — read-only post-deploy smoke checks (API readiness, landing page,
  security headers, and the OIDC login redirect) for a configured recruiter deployment. It
  creates nothing and mutates nothing.
