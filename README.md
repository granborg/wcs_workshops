# WCS Workshops

Mobile-first app where creators organize videos in folders and share them with viewers through QR codes.
Videos are hosted on Vimeo. Users sign in with Google, Apple, or Microsoft through Firebase Authentication.

## Layout

| Path | What it is |
|---|---|
| `apps/app` | Vue 3 (`<script setup lang="ts">`) + Vite app, wrapped with Capacitor for iOS and Android |
| `apps/api` | Go HTTP API and static server for the built app. SQLite through `database/sql` with hand-written SQL |
| `docs/SPEC.md` | Product specification |
| `docs/ARCHITECTURE.md` | Technical decisions, data model, API surface |

## First run

Requirements: Node 22 or newer, Go 1.23 or newer (the toolchain named in `apps/api/go.mod` downloads on
demand), and Java 21 or newer for the Firebase Auth emulator. No C compiler: the SQLite driver is pure Go.

```bash
npm install
cp .env.example apps/api/.env      # defaults already point at the emulator
cp .env.example apps/app/.env
npm run dev:auth                   # Firebase Auth emulator on 127.0.0.1:9099 (downloads firebase-tools once)
npm run dev:api                    # http://localhost:3000, creates apps/api/data/wcs.db on first start
npm run dev:app                    # http://localhost:5173, proxies /api to the Go server
```

Open <http://localhost:5173> and press any sign-in button. The emulator shows a fake account picker, so you can
create as many test users as you need without a Firebase project. Make one a creator in Settings.

## Checks

```bash
npm run typecheck                  # Vue app
npm run test:api                   # Go tests: HTTP end-to-end against a temp SQLite file, token verification
```

## Production build

```bash
npm run build                      # builds the web app, embeds it, compiles apps/api/bin/api
```

The binary serves the web app at `/` and the API at `/api`. It needs the `.env` values (or real environment
variables) and a writable directory for `DATABASE_PATH`. For real sign-in, set `GOOGLE_APPLICATION_CREDENTIALS`
to a Firebase service account file, unset `FIREBASE_AUTH_EMULATOR_HOST`, and fill the `VITE_FIREBASE_*` values
before building. SQLite means one API process per database file: run a single instance and back up the file.

## Mobile builds

```bash
VITE_API_URL=https://your-host/api npm run build -w apps/app
npm run cap:sync                   # copies dist/ into android/ and ios/
npx -w apps/app cap open android   # Android Studio
npx -w apps/app cap open ios       # Xcode, macOS only
```

The `android/` platform is committed. The `ios/` platform is added on a Mac with `npx -w apps/app cap add ios`.
After adding iOS, register the `wcsw` URL scheme in Xcode (Info > URL Types) so camera-app scans open the app.
Native sign-in needs `google-services.json` (Android) and `GoogleService-Info.plist` (iOS); neither is committed.

## Vimeo

Create an app at <https://developer.vimeo.com/apps>. Answer **No** to "Will people besides you be able
to access your app?". Generate a token with scopes `public private create edit delete upload video_files`
and put it in `VIMEO_ACCESS_TOKEN`. Paid plans get upload access automatically. Free accounts must request it.
