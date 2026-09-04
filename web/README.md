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

## Authentication modes

- `WEB_AUTH_MODE=demo` (default) maps two fixed synthetic identifiers server-side. The browser
  never receives the API base URL or the demo tokens.
- `WEB_AUTH_MODE=oidc` runs the real Authorization Code + PKCE flow: the Next server owns an
  AES-GCM-encrypted, HttpOnly session and verifies the ID token's issuer, audience, and nonce.
  This path is proven against the local OIDC fixture (`compose.oidc.yml`) and its browser test;
  no hosted provider such as Auth0 has been configured or verified.

Public configuration variables are listed in `.env.public.example`. A public deployment
(`WEB_DEPLOYMENT_ENV=public`) rejects demo auth, non-HTTPS URLs, a missing client secret, and
the known local session-secret placeholder.

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
