# Architecture

## Pieces

```
 browser / Capacitor app (Vue 3)  --/api-->  Go binary  --REST-->  Vimeo
        |                                       |
        | tus upload (direct)                   | database/sql
        v                                       v
      Vimeo                                 SQLite file
```

One Go binary serves the built web app at `/` and the API at `/api`. The Capacitor shells load the same web
build from the device and call the API over HTTPS.

- **Identity:** Firebase Authentication provides Google, Apple, and Microsoft sign-in. In browsers the app uses
  FirebaseUI v7 core (`@firebase-oss/ui-core`); FirebaseUI has no Vue components, so the sign-in screen is ours.
  Inside the Capacitor shells the app uses the Capacitor Firebase Authentication plugin, because Google refuses
  OAuth inside embedded WebViews. Both yield a Firebase ID token sent as a Bearer token. The API verifies it with
  the Firebase Admin SDK for Go and maps it to a `users` row on first sight. Adapters:
  `apps/app/src/services/auth.ts` and `apps/api/internal/auth`.
- **Local sign-in:** the Firebase Auth emulator (`npm run dev:auth`). Both sides read an emulator host variable
  and otherwise run the exact production code path. There is no development bypass.
- **Video storage:** one Vimeo account owned by the operator. The API holds the token. Creators never see it.
  Upload uses the Vimeo tus flow: the API creates the upload and returns a one-time upload link, then the device
  sends the file bytes straight to Vimeo.
- **Playback:** the API checks access and returns the Vimeo id plus unlisted hash. The app embeds the player.
- **Database:** SQLite via `modernc.org/sqlite` (pure Go). The schema is one SQL file embedded in the binary
  (`internal/db/migrations`), applied on start. Queries are hand-written in `internal/store`, one file per table.

## API packages

| Package | Role |
|---|---|
| `cmd/api` | Entry point: config, database, HTTP server, graceful shutdown |
| `internal/config` | Environment variables and `.env` loading |
| `internal/db` | Open SQLite with pragmas, run migrations |
| `internal/model` | Row structs and the JSON shapes mirrored in `apps/app/src/types.ts` |
| `internal/store` | SQL. No business rules |
| `internal/access` | Access resolution and tree pruning |
| `internal/server` | Routes (Go 1.22 `ServeMux` patterns), auth middleware, handlers, SPA fallback, end-to-end tests |
| `internal/auth`, `internal/vimeo` | Firebase token verification, Vimeo client |
| `internal/httpx` | Typed errors, JSON helpers, CORS, request logging |
| `web` | `embed.FS` holding the built web app (`npm run build` fills it) |

## Access model

Every creator has one root directory (`parent_id IS NULL`). Every video lives in exactly one directory.

A viewer can see a video when an active `access_grants` row exists for:
- the video itself, or
- any directory on the path from the video's directory up to the creator's root.

Grants reference directory and video ids, not names or paths, so renames keep access and moves change it
implicitly. Revocation sets `revoked_at` and keeps the row.

A `qr_codes` row holds a random `code`. The QR image and the "Copy link" button use
`https://<host>/r/<code>`. `<host>` is `PUBLIC_URL`, or the origin of the request when it is not set. Redeeming
creates a grant with the code id and the client IP. Expired or revoked codes are refused.

Any phone camera opens the link in the browser. The `/r/:code` route sends it to the scan page, which redeems the
code after sign-in. The in-app scanner reads the same link directly. To make the camera open the installed app
instead of the browser, set up Android App Links and iOS Universal Links for the host; `services/deeplinks.ts`
already routes `/r/` URLs.

Codes printed before this change encode `wcsw://redeem/<code>`. Redeem still accepts them, and the Android app
registers the `wcsw` scheme in `AndroidManifest.xml`.

Resolution loads the creator's directories and videos (two queries) and walks the tree in memory. At a few
hundred videos per creator this is a few milliseconds and keeps the SQL simple.

## Audit log

There is no audit table. `qr_codes` and `access_grants` carry `created_at`, `revoked_at`, and the client IP,
so the audit log is one `UNION ALL` query over those two tables (`internal/store/audit.go`). It answers the
spec's questions: who gained access to what, from which IP, with which code, and when codes were generated or
revoked. Two consequences: moves and deletes are not logged, and deleting a directory or video deletes its
grants and therefore its history.

## Data model

See `apps/api/internal/db/migrations/0001_init.sql`. Tables: users, directories, videos, qr_codes,
access_grants. Timestamps are Unix milliseconds. Ids are UUID strings.

## API surface

All paths are under `/api`.

| Method and path | Purpose |
|---|---|
| `GET /me`, `PATCH /me` | Account settings |
| `GET /library` | Creators who shared with me, plus my own collection |
| `GET /library/{creatorId}` | Directory tree pruned to what I may see |
| `POST /directories`, `PATCH /directories/{id}`, `DELETE /directories/{id}?mode=delete\|move` | Create, rename or move, delete |
| `POST /videos/upload-ticket`, `POST /videos/{id}/complete` | Two-step upload through Vimeo tus |
| `GET /videos/{id}`, `GET /videos/{id}/playback`, `PATCH /videos/{id}`, `DELETE /videos/{id}` | Read, play, rename or move, delete |
| `POST /share`, `GET /share`, `DELETE /share/{id}` | QR codes |
| `GET /access`, `DELETE /access/{id}` | View and revoke grants |
| `POST /redeem` | Scan result |
| `GET /audit` | Creator access history |

Errors are `{ "error": "<code>", "message": "<text>" }` with codes `not_found`, `forbidden`, `bad_request`,
`conflict`, `unauthenticated`, `vimeo_not_configured`, `vimeo_error`, `internal`.

## Deferred features

Both are in the spec and are not built yet. Where they plug in:

- **Creator banners.** Add a `banner_url` column to `users`, an upload endpoint, and a place to store the file
  (object storage, not the API's disk). The library list and collection header show the image in place of the name.
- **Push notifications.** Add a `devices` table keyed by push token, a `POST /devices` route called from the app
  after `PushNotifications.register()`, and a send on grant creation in `handlers_redeem.go` through Firebase
  Cloud Messaging in the Admin SDK.

## Vimeo privacy modes

- `unlisted` (default): the video is reachable only with its hash. Works from any origin, including the
  Capacitor WebView. Available on all plans.
- `disable`: hidden from vimeo.com and embeddable only on `VIMEO_EMBED_DOMAINS`. Needs the Core plan or above.
  The Capacitor WebView origin may not pass the domain check. Test on a device before relying on it.

## Open decisions

- Popup versus redirect for web OAuth. FirebaseUI defaults to popups. If mobile browsers block them, switch to
  `providerRedirectStrategy()` in `services/auth.ts`.
- Backups: SQLite is one file. Decide where it is copied and how often.
