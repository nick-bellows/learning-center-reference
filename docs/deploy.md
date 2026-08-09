# Deploy runbook

Live demo target: **web on Vercel**, **API + Postgres on Fly.io**. Both run on Nick's own
accounts; the login steps are interactive, so run the `!`-prefixed commands yourself.

## Prerequisites (one time)
- A Fly.io account and `flyctl` installed (`winget install Fly.Flyctl`, or see fly.io/docs).
- A Vercel account.

## API + Postgres on Fly.io
From the `api/` directory:

1. **Log in** (opens a browser):
   ```
   ! fly auth login
   ```
2. **Create the app** without deploying yet (reads the committed `fly.toml` + `Dockerfile`):
   ```
   ! fly launch --no-deploy
   ```
   Pick a unique app name; keep the Dockerfile it detects.
3. **Create + attach Postgres** (this sets the `DATABASE_URL` secret automatically):
   ```
   ! fly postgres create
   ! fly postgres attach <your-postgres-app-name>
   ```
4. **Run the migrations against the Fly database.** Open a local proxy to it, then apply the
   SQL from `api/migrations/` (and optionally `db/seed/seed.sql`) the same way we do locally:
   ```
   ! fly proxy 5432 -a <your-postgres-app-name>
   # in another shell, point psql/migrate at localhost:5432 with the Fly DB creds
   ```
5. **Deploy** and smoke-test:
   ```
   ! fly deploy
   ! curl https://<your-app>.fly.dev/health
   ```

## Web on Vercel
1. Import the GitHub repo in Vercel and set **Root Directory = `web`**.
2. Add an environment variable **`API_BASE_URL`** = your Fly API URL
   (e.g. `https://<your-app>.fly.dev`). The web app fetches the API **server-side**, so there
   is no CORS to configure.
3. Deploy. Visit `/members` to see live eligibility badges.

## Notes
- The API image is a distroless static binary (see `api/Dockerfile`) — verified to build
  locally with `docker build ./api`.
- AWS alternative (App Runner + RDS) for the exact-JD-stack talking point is captured
  separately in `docs/deploy-aws.md` (planned).
- Keep all seeded data synthetic; never deploy real member data.
