# WCS Workshops

Mobile-first app where creators organize videos in folders and share them with viewers through QR codes.
Videos are hosted on Vimeo. Users sign in with Google, Apple, or Microsoft through Firebase Authentication,
using FirebaseUI v7 core on the web and the Capacitor Firebase plugin in the native shells.

## Layout

| Path | What it is |
|---|---|
| `apps/app` | Vue 3 (`<script setup lang="ts">`) + Vite app, wrapped with Capacitor for iOS and Android |
| `apps/api` | Go HTTP API. SQLite through `database/sql` with hand-written SQL. Owns the Vimeo token, access lists, audit log, push |
| `packages/shared` | TypeScript types for the API's JSON shapes, used by the app |
| `docs/SPEC.md` | Product specification |
| `docs/ARCHITECTURE.md` | Technical decisions, data model, API surface |

## First run

Requirements: Node 22 or newer, npm 10 or newer, Go 1.23 or newer (the Go toolchain auto-downloads the
version named in `apps/api/go.mod`). No C compiler is needed: the SQLite driver is pure Go.

```bash
npm install
cp .env.example apps/api/.env      # then edit values
cp .env.example apps/app/.env      # then edit values
npm run dev:api                    # http://localhost:3000, creates apps/api/data/wcs.db on first start
npm run dev:app                    # http://localhost:5173
```

With `DEV_AUTH_BYPASS=true` in the API and `VITE_DEV_AUTH=true` in the app, no Firebase project is needed.
The app sends an `X-Dev-User` header and the API creates that user on first request. Sign out and enter a
different email to switch between test users.

## Checks

```bash
npm run typecheck                  # app and shared types
npm run test:api                   # Go unit and end-to-end API tests (no Vimeo token needed)
```

The API test suite drives the HTTP layer against a temporary SQLite file and covers role switching, folders,
QR share, redeem as a second user, pruned library views, revocation, and the audit log.

## Mobile builds

```bash
npm run build -w apps/app
npm run cap:sync                   # copies dist/ into android/ and ios/
npx -w apps/app cap open android   # Android Studio
npx -w apps/app cap open ios       # Xcode, macOS only
```

The `android/` platform is committed. The `ios/` platform is added on a Mac with `npx -w apps/app cap add ios`.
After adding iOS, register the `wcsw` URL scheme in Xcode (Info > URL Types) so camera-app scans open the app.

Native sign-in and push need Firebase config files that are not committed: `google-services.json` for Android
and `GoogleService-Info.plist` for iOS. Set `VITE_DEV_AUTH=false` and fill the `VITE_FIREBASE_*` values.

## Vimeo

Create an app at <https://developer.vimeo.com/apps>. Answer **No** to "Will people besides you be able
to access your app?". Generate a token with scopes `public private create edit delete upload video_files`
and put it in `VIMEO_ACCESS_TOKEN`. Paid plans get upload access automatically. Free accounts must request it.

## Deploying the API

`go build -C apps/api -o bin/api ./cmd/api` produces one static binary. It needs the `.env` values (or real
environment variables), a writable directory for `DATABASE_PATH` and `UPLOAD_DIR`, and nothing else.
SQLite means one API process per database file. Run a single instance.
