# 6. Test browser identity locally; fail closed in public configuration

- **Status:** accepted and implemented locally
- **Date:** 2026-09-02

## Context

The API already verified OIDC tokens, but fixed demo bearer identifiers did not prove the browser
redirect, PKCE, callback, nonce/state, encrypted session cookie, logout, or explicit role-switching
path. Creating an Auth0 tenant would add an external account and secret dependency.

## Decision

Use a tiny local-only OIDC fixture for automated browser tests. It exposes discovery/JWKS,
Authorization Code + PKCE, signed one-hour tokens, single-use two-minute codes, and exactly two
fictional identities. The Next.js server owns an AES-GCM-encrypted, HttpOnly, SameSite session;
the API still resolves authorization roles from PostgreSQL. The fixture refuses non-local callback
URLs and is not part of any public deployment plan.

Public configuration requires HTTPS app/API/issuer URLs, OIDC mode, a confidential-client secret,
and a non-placeholder session secret. The API independently requires OIDC, PostgreSQL, and an
HTTPS issuer. Request size, concurrency, rate, and safe response-header limits are applied before
the handlers.

## Consequences

- CI can prove the whole browser boundary without claiming Auth0 or paying for infrastructure.
- Hosted OIDC interoperability remains unverified until a real provider is configured and tested.
- A shared public demo needs the scoped reset command and an external scheduler or operator; this
  repository proves the reset behavior but does not claim a schedule exists.
- The session is intentionally short-lived and contains only an encrypted access token, subject,
  and expiry. Refresh tokens are not requested or stored for this bounded demo.
