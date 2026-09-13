# Glossary

Definitions of the domain and technical terms used across this repository, with pointers to
where each one lives in the code. Everything here describes what is implemented; planned work
is marked as such.

### Domain

- **Member** — a person known to the fictional federation: a learner, coach, referee,
  instructor, or administrator. Identified by a UUID and an identity-provider subject
  (`member.auth_subject`); never by name. Roles live in `member_role` (migration `0001`).
- **Role** — an application permission resolved from the database, never from a token
  claim: `learner`, `instructor`, `admin`, `coach`, `referee`. The API checks the required
  role on every protected route (`requireRole` in `internal/httpapi`).
- **Course / module / lesson** — the published learning content structure (migration
  `0002`). A course is `sequential` or `open`; a sequential course refuses an out-of-order
  completion with `409`. Lesson content itself is referenced by `content_ref`, the seam for a
  headless CMS that is not provisioned.
- **Enrollment** — a member's participation in a course, created idempotently (retrying the
  request returns the existing row). Status is `active`, `completed`, or `withdrawn`; a
  withdrawn enrollment refuses further completions.
- **Progress event** — an immutable `lesson_completed` record in `progress_event`
  (migration `0005`). The log is append-only for the runtime database role, which cannot
  update or delete rows in it.
- **Projection** — `enrollment_progress`, the read model (completed/total/percent) rebuilt
  from progress events. It is written in the same transaction as the event and can be
  checked or rebuilt from the log by `cmd/reconcileprogress` (`internal/projection`).
- **Safeguarding facts** — the expiring records eligibility is derived from: an approved
  background check, SafeSport training, and, for coaches and referees, a role credential
  (coaching license or referee recertification). Each has an in-effect date and an expiry
  date (migrations `0003`, `0004`, `0006`).
- **Disciplinary hold** — a record that suspends a member while `lifted_at` is null and
  `placed_at` has passed. Any active hold overrides every other fact.
- **Eligibility** — a member's participation status, one of `eligible`, `suspended`, or
  `ineligible_lapsed`, with a reason. It is never stored: `internal/safeguarding` recomputes
  it from the facts on every request. See `docs/domain-model.md` section 4.
- **Weakest link** — a member holding both the coach and referee roles needs a current
  credential for each; the earliest of the latest per-role expiries governs, and one role
  with nothing on file means "missing".
- **Inclusive expiry** — a credential expiring on a date is valid through that whole day in
  UTC and lapses at the next midnight. `safeguarding.Current` is the single implementation.
- **In effect** — a record counts only once its approval, completion, or issue date has
  arrived; a future-dated record cannot grant eligibility early. The SQL filters and the Go
  rule both evaluate this on the UTC date.
- **Credentials contract v1** — `GET /v1/members/{subject}/credentials`, read by another
  service (the fictional federation's member-services lab) with a service token carrying
  scope `credentials:read`. It exposes existing facts and derived eligibility; nothing is
  issued. The consumer's fixtures under `api/testdata/contracts` pin its shape.
- **Prerequisite graph** — coaching licenses branch (A splits into youth and senior tracks),
  so prerequisites are described as a directed acyclic graph in the domain model. Not
  implemented in the schema; documented as future work.
- **Age group / cutoff** — a player's competitive age group derives from date of birth and
  the organization's cutoff rule (birth year or school year). Documented in the domain
  model, which is why the schema stores a date of birth rather than an age.

### Identity and sessions

- **OIDC (OpenID Connect)** — the standards-based login the web app performs with
  Authorization Code + PKCE. The API validates signature, issuer, audience, and expiry on
  every bearer token (`internal/authn`); the web app validates the ID token's issuer,
  audience, nonce, and subject (`web/lib/oidc.ts`).
- **Local OIDC fixture** — `cmd/oidcfixture`, a small standards-based provider used by
  `compose.oidc.yml` and CI so the full redirect, callback, session, and logout path is
  proven without an external account. It is a test dependency, never an internet provider.
- **Demo mode** — `AUTH_MODE=demo`: two fixed synthetic bearer tokens map to the seeded
  learner and administrator. Explicit, local-only, refused by public-mode configuration.
- **Service token** — a bearer token that identifies a calling system rather than a person
  and carries a scope (`scope` or `scp` claim). No member row is resolved for it.
- **Session cookie** — `lcr_session`, an HttpOnly, SameSite=Lax cookie sealed with
  AES-256-GCM under a key derived from `SESSION_SECRET`; the cookie name is authenticated as
  additional data so a login transaction cookie can never be presented as a session.
- **Login transaction** — `lcr_oidc_transaction`, the sealed state, nonce, PKCE verifier,
  and allow-listed return path for one in-flight sign-in. Single-use and ten minutes long.

### Database

- **Migration** — a versioned SQL file that changes the schema. `api/migrations/*.up.sql`
  are embedded in the API binary and applied forward-only under an advisory lock
  (`internal/dbsetup`); the `.down.sql` files document reversal and are not applied
  automatically.
- **Runtime role** — `lcr_runtime` (migration `0007`), a DML-only role request handling runs
  as. It cannot change the schema, read the migration ledger, or update or delete progress
  events. Migrations, the seed, and the operator commands run as the owner.
- **Seed** — `db/seed/seed.sql`, idempotent synthetic data whose dates are relative to the
  day it is applied, so the three example eligibility statuses never rot.
- **UUID** — the 128-bit random identifier used for every primary key, so ids are neither
  guessable nor enumerable. The API validates the format before any database cast.
- **CHECK constraint** — a database-enforced rule on a column, such as the allowed role or
  status values, or the chronology rule that an expiry follows its issue date (migration
  `0006`). Text plus CHECK is used instead of enum types so adding a value is a one-line
  change.
- **Partial index** — an index over a subset of rows; only active disciplinary holds are
  indexed because those are all eligibility ever asks about.
- **Advisory lock** — a PostgreSQL application-level lock; the migration runner holds one
  so two API instances starting together cannot both apply a migration.
- **Throwaway test database** — `internal/testdb` creates, migrates, and drops a private
  database per integration test so parallel packages never share rows.

### API and operations

- **Route pattern logging** — the request log records the matched pattern
  (`/v1/members/{subject}/credentials`), never the raw path, so subjects and member ids do
  not reach the logs.
- **Request id** — a server-generated `X-Request-Id` on every response that matches the
  `request_id` field of the corresponding log line. Inbound values are ignored.
- **JSON error contract** — every error, including an unknown path, a wrong method, a
  recovered panic, throttling (`429`), and unconfigured dependencies (`503`), is a JSON
  `{"error": ...}` body documented in `api/openapi.yaml` and checked by the conformance test.
- **Concurrency limit and rate limit** — two bounded 429 sources: at most
  `MAX_CONCURRENT_REQUESTS` in flight, and at most `RATE_LIMIT_PER_MINUTE` per client
  address (rightmost `X-Forwarded-For` when `TRUST_PROXY=1`).
- **Health check** — `GET /health` pings the database and answers `503` when it is
  unreachable, so a platform never routes traffic to an instance that cannot serve it.
- **Demo reset** — `cmd/resetdemo`, which deletes only the mutable enrollment and progress
  state of the fictional association after an explicit confirmation value.
- **Distroless image** — the API's runtime base image contains the static binary and
  nothing else (no shell), which is why migrations and the seed are applied by the binary.

### Web

- **Server Component** — the App Router default: renders on the server, fetches the API
  directly with a server-held token, and ships no component JavaScript to the browser.
  Every page in `web/app` is one.
- **Server Action** — a server-side function invoked from a form (`web/app/learn/actions.ts`)
  for enrollment and lesson completion; Next checks the request origin on these.
- **Content Security Policy** — the response header that limits the page to same-origin
  resources; `Strict-Transport-Security` is left to the TLS terminator.
- **axe** — the automated WCAG A/AA checker run by Playwright over every route and the
  OIDC-only states. It catches a subset of issues; the manual review checklist in
  `docs/accessibility-manual-review.md` covers the rest and has not been run yet.

### Planned, not implemented

- **Assessment and credential issuance** — attempts, passing rules, and an expiring
  credential issued from course completion that changes derived eligibility. Held by
  decision B1 in `ROADMAP.md`.
- **Headless CMS** — editable course content behind `content_ref`. No project exists.
- **Hosted demo and hosted identity provider** — gated on account and cost approval
  (`ROADMAP.md`, Lane B). Nothing runs in production; the local Compose stack and CI are the
  only exercised environments.
