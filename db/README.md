# db

Synthetic seed data only. Every member, enrollment, license, and credential here is
**fictional and visibly labeled**; no real member, player, or employer data ever enters this
project.

`seed/seed.sql` inserts the synthetic members, roles, safeguarding records, role credentials,
and one published course. It is idempotent (fixed ids + `ON CONFLICT`) and is applied on API
startup via `SEED_FILE`. Credential dates are relative to first-seed time (`current_date +/-`
an interval) so the derived-eligibility demo shows stable statuses whenever the project is
cloned rather than lapsing on a fixed calendar date.
