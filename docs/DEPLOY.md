# Deploying to Fly.io

One Fly machine runs the Go binary. The SQLite file lives on a Fly volume. The binary serves the web app and
the API from the same hostname, so browsers need no CORS and no second host.

## One-time setup

1. Install flyctl and sign in: <https://fly.io/docs/flyctl/install/>, then `fly auth login`.
2. Create the app and the volume. Pick a region near your users (`fly platform regions`).

   ```bash
   fly apps create wcs-workshops
   fly volumes create data --region ord --size 1
   ```

   If you choose another app name or region, change `app` and `primary_region` in `fly.toml`.
3. Firebase: create a project, enable the Google, Apple, and Microsoft providers, and add your Fly hostname
   (`wcs-workshops.fly.dev` or your custom domain) under Authentication > Settings > Authorized domains.
   Download a service account key from Project settings > Service accounts.
4. Fill `fly.toml`: the `VITE_FIREBASE_*` build args from Project settings > Your apps, and
   `FIREBASE_PROJECT_ID` under `[env]`.
5. Set the secrets. The service account JSON is passed as one string:

   ```bash
   fly secrets set VIMEO_ACCESS_TOKEN="..." FIREBASE_CREDENTIALS_JSON="$(cat service-account.json)"
   ```

## Deploy

```bash
fly deploy
```

Fly builds the Dockerfile remotely, so Docker is not needed on your machine. The first build takes a few
minutes (Node and Go toolchains). Check it:

```bash
fly status
curl https://wcs-workshops.fly.dev/api/health
```

Open the hostname in a browser, sign in, and set your account to Creator in Settings.

## Mobile builds against Fly

```bash
VITE_API_URL=https://wcs-workshops.fly.dev/api \
VITE_FIREBASE_API_KEY=... VITE_FIREBASE_AUTH_DOMAIN=... VITE_FIREBASE_PROJECT_ID=... VITE_FIREBASE_APP_ID=... \
npm run build -w apps/app && npm run cap:sync
```

## Operations

- **Logs:** `fly logs`.
- **Scale:** keep `min_machines_running = 1` and never more than one machine. SQLite has one writer. The
  smallest shared machine is enough; the API moves metadata only, video bytes go straight to Vimeo.
- **Backups:** `fly volumes snapshots list <volume-id>` shows daily snapshots Fly keeps for five days. For
  point-in-time restore, add Litestream later: run it as a second process in the image and stream the file to
  Tigris (Fly's object storage) or Backblaze B2.
- **Custom domain:** `fly certs add workshops.example.com`, then add the DNS records it prints and add the
  domain to Firebase authorized domains.
- **Schema changes:** add `apps/api/internal/db/migrations/0002_*.sql`. It runs on the next start.

## Why not scale to zero

`auto_stop_machines` is off on purpose. Stopping the machine would make the first request after idle slow and,
more important, a restart during a write is the one scenario SQLite on a volume should avoid. The idle cost of
one shared machine is a few dollars a month.
