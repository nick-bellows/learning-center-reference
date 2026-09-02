# Web

Next.js App Router, TypeScript, and Tailwind front end for the implemented workflow.

- `/` explains the bounded reference implementation.
- `/learn` loads catalog/dashboard data in Server Components and mutates enrollment/progress
  through Server Actions. The Go API remains the authorization boundary.
- `/admin/compliance` uses the synthetic administrator identity to render current derived
  eligibility and credential-expiry evidence.
- `/members` keeps three fixed rule examples visible for focused troubleshooting.
- `tests/accessibility.spec.ts` runs axe WCAG 2.0/2.1 A and AA rules on all four routes.

Run local checks:

```sh
npm ci
npm run lint
npm run build
npx playwright install chromium
PLAYWRIGHT_BASE_URL=http://localhost:3000 npm run test:a11y
```

`API_BASE_URL`, `DEMO_LEARNER_TOKEN`, and `DEMO_ADMIN_TOKEN` are consumed server-side.
The demo identifiers are fixed synthetic local values, not production secrets or a claim of
completed browser login/session handling.

Independent portfolio project. Fictional data only. Not affiliated with or endorsed by
U.S. Soccer or any member organization.
