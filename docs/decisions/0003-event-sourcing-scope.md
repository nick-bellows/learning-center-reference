# 3. Confine event sourcing to the progress context

- **Status:** accepted
- **Date:** 2026-08-07

## Context

The JD lists **CQRS / event sourcing** as a preferred skill. Event sourcing (storing an
append-only log of events and projecting read models from them) is powerful but easy to
over-apply — spreading it across a whole system adds large complexity for little benefit.

## Decision

Use event sourcing in **exactly one bounded context: learner progress.** Progress events
(lesson started/completed, assessment attempted/passed) are appended to a `progress_events`
table; a projection builds a `progress_read` model the UI queries. Every other context
(courses, licenses, referee grades, safeguarding records) uses ordinary CRUD tables.

## Consequences

- We can speak to CQRS/event sourcing truthfully, with a real, well-chosen example, and
  explain *why we did not use it elsewhere* — which demonstrates judgment, not just knowledge.
- Progress gets a natural audit trail (useful for a compliance-adjacent product).
- Slightly more code in that one context than plain CRUD would need; deliberately contained.
