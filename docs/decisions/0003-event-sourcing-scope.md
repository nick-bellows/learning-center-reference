# 3. Confine event sourcing to the progress context

- **Status:** accepted and implemented
- **Date:** 2026-08-07
- **Implemented:** 2026-09-01

## Context

Event sourcing—storing an append-only log of events and projecting read models from it—is
powerful but easy to over-apply. Spreading it across a small system would add operational
complexity without a corresponding product need.

## Decision

Use event sourcing in **exactly one bounded context: learner progress.** The implemented
slice appends `lesson_completed` facts to `progress_event`; a transactional
`enrollment_progress` projection serves the dashboard. The event uniqueness constraint
makes client retries idempotent, and locking the enrollment serializes concurrent
completion requests. Every other context—courses, roles, credentials, and safeguarding
records—uses ordinary relational tables.

The event insert, projection refresh, and enrollment-status update happen in one database
transaction. This avoids introducing an event broker and eventual consistency where the
current scale and workflow do not require either.

## Consequences

- The repository contains a working, bounded event-sourcing example and a clear reason it
  was not spread across the application.
- Progress gets a natural audit trail while dashboard reads remain simple.
- Course content is currently treated as immutable after enrollment. Supporting edits to
  active courses requires explicit versioning rather than silently changing totals.
- This is not a globally asynchronous architecture. A broker/outbox becomes justified only
  when an independent downstream system genuinely needs progress events.
