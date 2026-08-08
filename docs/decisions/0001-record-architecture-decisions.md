# 1. Record architecture decisions

- **Status:** accepted
- **Date:** 2026-08-07

## Context

This project is a portfolio piece whose author must be able to explain *why* every
significant choice was made — the target role's interviews probe trade-offs, not syntax,
and prohibit AI assistance during the interview itself.

## Decision

We keep short **Architecture Decision Records (ADRs)** in `docs/decisions/`. Each records
the context, the decision, and the consequences of one significant choice. They are
numbered in the order made. An ADR is immutable once accepted; if a decision changes, we
add a new ADR that supersedes the old one.

## Consequences

- A written trail of reasoning to study before an interview and speak from with confidence.
- New contributors (and future-me) can see why the system is shaped the way it is.
- Small ongoing cost: write one short note per significant decision.
