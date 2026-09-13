# Web

Next.js App Router, TypeScript, and Tailwind front end for the implemented workflow.

- `/` explains the bounded reference implementation and links each step to its code.
- `/learn` loads catalog/dashboard data in Server Components and mutates enrollment/progress
  through Server Actions. The Go API remains the authorization boundary.
- `/admin/compliance` uses the administrator identity to render current derived eligibility
  and credential-expiry evidence.
- `/members` keeps three fixed rule examples visible for focused troubleshooting.
- `/auth/error` is the generic sign-in failure page; it exposes no provider or token detail.
- `app/api/auth/{login,callback,logout}` own the browser OIDC redirect, callback, and logout.
  Logout is a POST that also checks the request's `Origin` against `APP_BASE_URL`.
- `app/error.tsx`, `app/global-error.tsx`, and `app/not-found.tsx` are the branded failure
  states: a route error with a re-fetching "Try again", a root-layout or configuration failure,
  and an unmatched URL that keeps the site chrome.
- `instrumentation.ts` validates the configuration once at server start, so a bad deployment
  setting fails the process with a logged reason instead of erroring on every request.

## Authentication modes

- `WEB_AUTH_MODE=demo` (default) maps two fixed synthetic identifiers server-side. The browser
  never receives the API base URL or the demo tokens.
- `WEB_AUTH_MODE=oidc` runs the real Authorization Code + PKCE flow: the Next server owns an
  AES-GCM-encrypted, HttpOnly session and verifies the ID token's issuer, audience, and nonce.
  This path is proven against the local OIDC fixture (`compose.oidc.yml`) by two browser
  suites: `tests/auth.spec.ts` (happy path) and `tests/auth-negative.spec.ts` (tampered, forged,
  expired, and token-less sessions; mismatched state, provider `error=`, forged code, expired
  transaction, and replayed callbacks; hostile `returnTo`). No hosted provider such as Auth0
  has been configured or verified.

Public configuration variables are listed in `.env.public.example`. A public deployment
(`WEB_DEPLOYMENT_ENV=public`) rejects demo auth, non-HTTPS URLs, a missing client secret, and
the known local session-secret placeholder.

Every variable the web server reads:

| Variable | Purpose |
| --- | --- |
| `WEB_DEPLOYMENT_ENV` | `local` (default) or `public`; public enables the checks above and secure cookies |
| `WEB_AUTH_MODE` | `demo` (default) or `oidc` |
| `API_BASE_URL` | Go API origin the server fetches from (never sent to the browser) |
| `APP_BASE_URL` | This app's public origin; used for the OIDC redirect URI and the logout origin check |
| `DEMO_LEARNER_TOKEN`, `DEMO_ADMIN_TOKEN` | Demo-mode bearer tokens presented to the API server-side (defaults match `compose.yml`) |
| `OIDC_ISSUER_URL`, `OIDC_CLIENT_ID`, `OIDC_CLIENT_SECRET`, `OIDC_AUDIENCE` | OIDC provider settings (`oidc` mode) |
| `SESSION_SECRET` | At least 32 characters; derives the AES-GCM key for the session and transaction cookies |
| `SESSION_COOKIE_SECURE` | `1` forces the `Secure` cookie flag in local mode (public mode always sets it) |

Every server-side API call is bounded by a 5-second timeout so a hung API reaches the pages'
"service unavailable" states instead of hanging the render. Responses carry a
`Content-Security-Policy` that allows only same-origin resources (inline script/style stay
allowed because the App Router emits them; a nonce-based policy is the follow-up before any
hosted deployment). `Strict-Transport-Security` is left to the TLS terminator.

`tests/accessibility.spec.ts` runs axe against the demo-mode pages and the 404 page;
`tests/accessibility-oidc.spec.ts` (part of `npm run test:auth`) covers the states only the
OIDC overlay produces: signed-out prompts, the administrator refusal, the sign-in error page,
and a 390px viewport. Neither is a WCAG conformance claim; see
`docs/accessibility-manual-review.md`.

`tests/screencast.spec.ts` is a recording aid, not a gate: it runs only with
`PLAYWRIGHT_SCREENCAST=1` and is driven by `scripts/record-screencast.ps1` to produce the README
walkthrough GIF.

## Run local checks

```sh
npm ci
npm run lint
npm run build
npx playwright install chromium
PLAYWRIGHT_BASE_URL=http://localhost:3000 npm run test:a11y
```

The browser OIDC journey is covered by `npm run test:auth`, which runs against the local OIDC
overlay (`docker compose -f compose.yml -f compose.oidc.yml up --build`) with `PLAYWRIGHT_OIDC=1`.

Independent portfolio project. Fictional data only. Not affiliated with or endorsed by
U.S. Soccer or any member organization.
