# Interview Guide

This guide explains the code that exists. It is not a script for claiming production use,
U.S. Soccer affiliation, a hosted deployment, or experience that the repository cannot prove.

## Sixty-second architecture explanation

The Next.js application renders learner and administrator experiences and calls a Go API
server-side. In production mode, the API uses OIDC discovery to validate a signed bearer
token's issuer, audience, signature, and expiry. It then maps the stable subject to a member
and roles in PostgreSQL; request or token roles are not trusted. Learners can idempotently
enroll and append ordered lesson-completion events. The API updates a dashboard projection
in the same transaction. Administrators see eligibility computed from current credential
facts and holds rather than a stored boolean.

Default local Docker can map two fixed synthetic bearer identifiers. The optional OIDC overlay
instead proves Authorization Code + PKCE, signed JWTs, nonce/state checks, an encrypted HttpOnly
session, logout, and explicit learner/admin switching against a local-only provider fixture. It
does not pretend that Auth0 or another hosted provider has been tested.

## Where to point an interviewer

| Topic | Evidence |
| --- | --- |
| HTTP and authorization boundary | `api/internal/httpapi/router.go` |
| OIDC and fail-closed demo seam | `api/internal/authn/authn.go` |
| Browser OIDC/session boundary | `web/app/api/auth`, `web/lib/oidc.ts`, `web/lib/session.ts` |
| Local provider + browser E2E | `api/cmd/oidcfixture`, `web/tests/auth.spec.ts` |
| Role and ownership resolution | `api/internal/store/store.go` |
| Progress transaction and idempotency | `Store.CompleteLesson` in `api/internal/store/store.go` |
| Event/projection schema | `api/migrations/0005_progress.up.sql` |
| Eligibility rule and boundaries | `api/internal/safeguarding/eligibility.go` and its tests |
| Service-to-service credentials contract | `api/internal/credentials`, `authenticateService` in `router.go`, fixtures under `api/testdata/contracts/learning-center.credentials.v1` |
| API contract | `api/openapi.yaml` and `openapi_test.go` |
| Learner interaction | `web/app/learn/page.tsx` and `actions.ts` |
| Administrator interaction | `web/app/admin/compliance/page.tsx` |
| Accessibility gate | `web/tests/accessibility.spec.ts` |
| End-to-end proof | `.github/workflows/ci.yml` |

## Why these choices

### Go and chi

The API needs clear HTTP boundaries, predictable concurrency, and straightforward deployment.
Go keeps the service small and explicit; chi adds routing and middleware without a framework
owning the domain model. The cost is more hand-written mapping than a full ORM/framework.

### Next.js Server Components and Actions

The browser never needs the API base URL or the local demo bearer identifiers. Server
Components load views and Server Actions perform mutations, while the Go API remains the
authorization boundary. The local OIDC test session proves application behavior but is not
evidence of hosted-provider interoperability or production operation.

### PostgreSQL and parameterized pgx queries

The workflow is relational: members have multiple roles, courses contain ordered lessons,
and enrollments connect the two. Foreign keys, uniqueness constraints, transactions, and
row locks enforce invariants close to the data. Direct pgx queries make those decisions
visible, at the expense of writing scan/mapping code.

### Event sourcing only for progress

Completion is an immutable fact and benefits from retry-safe audit history. The dashboard,
however, should not replay every event on every request, so `enrollment_progress` is a read
projection. A single transaction keeps it consistent. Eligibility and course metadata do not
need event sourcing and use ordinary tables.

### Derived eligibility

A stored `eligible` flag can become wrong the instant a credential expires. The domain
function instead evaluates the current background check, SafeSport training, applicable role
credential, and active holds. A hold overrides all positive credentials. Expiry boundaries
are table-tested in UTC.

## Hard problems actually solved

- **Retries:** database uniqueness makes repeated enrollment and completion requests return
  current state without duplicating credit.
- **Ordering:** the transaction checks all prior lessons in a sequential course before adding
  a completion event and returns a domain-specific conflict otherwise.
- **Concurrent writes:** locking the enrollment serializes progress changes for one learner.
- **Authorization:** the verified subject resolves to database roles, then ownership is checked
  again for the target enrollment.
- **Fresh compliance:** administrator results call the pure eligibility rule over current data;
  no mutable status flag exists.
- **Honest local identity:** demo mode is explicit; the OIDC fixture signs real JWTs and exercises
  the redirect/session boundary; public configuration fails closed. The fixture is never presented
  as Auth0 or a production identity provider.

## Likely questions to prepare for

### Why not put roles in the JWT?

Provider roles can be useful, but this application treats PostgreSQL as the authorization
source so a role change takes effect without waiting for token refresh and organization rules
remain application-controlled. At larger scale, role lookups could be cached with deliberate
revocation behavior.

### What happens if two completion requests arrive together?

Both transactions lock the same enrollment row. The first appends the unique event and updates
the projection; the second then sees the unique event and returns the same progress with
`recorded: false`. The integration test also verifies ordinary retries.

### Why no message broker or eventual projection worker?

There is one service and one transactional database. Synchronous projection gives stronger
consistency with less operational surface. An outbox and consumer become worthwhile only when
independent downstream systems need the event.

### How is OIDC different from the local demo?

OIDC mode discovers provider metadata and cryptographically verifies the token. Demo mode maps
two fixed local identifiers to synthetic subjects and must be selected explicitly. The web now
implements Authorization Code + PKCE, callback, nonce/state verification, encrypted session, and
logout. Those are tested locally; do not call the path an Auth0 integration until Auth0 is used.

### How does a service token differ from a person token?

A person token identifies a human. The API resolves its subject to a member row and
authorises on database roles, so a valid token for an unprovisioned person is a 403 and a
token never carries authority on its own. A service token identifies another system: an
OAuth2 client-credentials token for this API's audience whose `scope` claim (or `scp`
array) the identity provider granted to that client. `GET /v1/members/{subject}/credentials`
verifies it with the same OIDC verifier, authorises on the `credentials:read` scope, and
never resolves a member for the caller. The two paths share one trust configuration but
authorise differently: roles from PostgreSQL for people, scopes from the provider for
services.

Local demo mode mirrors this with `DEMO_SERVICE_TOKEN`, a synthetic identifier mapped to the
service subject and scope. A person's demo token on the service route is valid but unscoped
and receives 403, exactly as with a real provider. The response carries only what the
consumer needs to decide participation (statuses, dates, roles): no hold reasons, display
names, or dates of birth. The subject appears in the request path and therefore in request
logs, as member ids already do on the eligibility route; a production deployment would log
the route pattern instead.

### What does the accessibility test prove?

It runs axe WCAG 2.0/2.1 A and AA rules against all four rendered routes. It proves only that
axe found no automatically detectable violations in that run. It does not prove full WCAG 2.1
AA conformance; manual keyboard, screen-reader, zoom, and visual review are still required.

### Why is the member eligibility example unauthenticated?

It is a deliberately narrow read-only reference endpoint over fixed, fictional, enumerable UUIDs
so the eligibility rule can be inspected without an account. That choice is not appropriate for
real member data. A production route would require authentication, organization-scoped
authorization, non-enumerable lookup behavior, and a privacy review before returning status or
reason fields.

### What would break at production scale?

The administrator compliance loader performs several readable queries per member and has no
pagination. A production version would use organization-scoped, set-based SQL, pagination,
authorization filters, indexes proven with query plans, and metrics around latency/errors.

## Failure scenarios and current behavior

| Failure | Current behavior | Production follow-up |
| --- | --- | --- |
| Database unavailable | `/health` returns 503; DB-backed routes fail | Alert on readiness and error rate; use managed backups/failover |
| Missing/invalid token | 401 without verification details; web asks for a fresh login | Consider renewal only if the bounded demo needs longer sessions |
| Valid but unprovisioned subject | 403 | Audited invitation/provisioning workflow |
| Learner calls admin endpoint | 403 from database role | Organization-aware policy checks |
| Learner targets another enrollment | 403 before mutation | Retain audit event for denied access if required |
| Later lesson completed first | 409; no event written | UI already exposes only the next lesson |
| Repeated enrollment/completion | Existing state returned | Add idempotency keys if mutations gain external side effects |
| Credential expires overnight | Next read computes lapsed status | Scheduled notifications and operational dashboard |
| Projection corruption | No repair command yet | Rebuild projection from events and reconcile in CI/operations |

## Improvements with more time or external credentials

1. Connect an approved hosted OIDC tenant and verify the existing callback/session/logout path.
2. Add organization tenancy to every read/write policy and test cross-organization denial.
3. Version published courses so lesson edits cannot silently change active enrollments.
4. Add assessment attempts and passing rules, then issue an expiring credential whose state
   contributes to eligibility.
5. Add projection rebuild/reconciliation tooling and operational metrics.
6. Run manual accessibility review and document findings before any conformance claim.
7. Deploy only after a cost/security review; this repository currently proves local and CI
   behavior, not cloud operation.
