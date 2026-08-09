# web

The Learning Center front end — Next.js (App Router) + TypeScript + Tailwind.

- `app/page.tsx` — landing page
- `app/members/page.tsx` — member participation status; a Server Component that fetches the
  Go API's eligibility endpoint and renders status badges

## Run locally

Requires the API + database running (see the repo root). Then:

```bash
npm install
npm run dev
```

Open http://localhost:3000. Set `API_BASE_URL` if the API is not at `http://localhost:8080`.

> Independent portfolio project — not affiliated with, endorsed by, or containing data from
> U.S. Soccer or any member organization. All data is fictional.
