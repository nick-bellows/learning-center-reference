# 2. Use Go for the API

- **Status:** accepted
- **Date:** 2026-08-07

## Context

The target role (U.S. Soccer Federation, Software Engineer) lists **Go** first among its
preferred backend languages, alongside PHP/Laravel. The author knows Python and C#/C++ but
not Go professionally. The front end is TypeScript/React regardless.

## Decision

Build the API in **Go** (`net/http` + the `chi` router), documented by an OpenAPI `/v1`
contract, over PostgreSQL via `pgx`.

Rationale:
- It is the JD's first-listed preferred language — a direct differentiator.
- Go maps cleanly onto the author's existing C# knowledge (static types, interfaces,
  generics), so the learning cost is low. The two genuinely new idioms are **explicit
  error handling** (`if err != nil`) and **composition over inheritance**.
- A REST API over Postgres is inside the first week of Go competence.

Rejected alternative: **TypeScript everywhere.** Simpler (one language) and still satisfies
the *required* "server-side language," but it forgoes the Go-preferred signal, which is a
cheap, high-value differentiator here.

## Consequences

- One more language to learn, but a small, opinionated one, learned by building.
- We keep PHP/Laravel out of scope — Go already covers a preferred backend language, and a
  second backend would be wasted effort. (We can note awareness of their public
  `soccer-id-sdk-php` when relevant.)
- If Go materially slows the timeline, the fallback is a TypeScript API plus one small Go
  service (e.g. certificate generation) so the Go claim stays truthful.
