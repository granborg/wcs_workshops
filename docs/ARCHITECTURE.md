# Architecture

## Pieces

```
 Capacitor app (Vue 3)  --HTTPS-->  Go API  --REST-->  Vimeo
        |                              |
        | tus upload (direct)          | database/sql
        v                              v
      Vimeo                         SQLite file
```

- **Identity:** Firebase Authentication provides Google, Apple, and Microsoft sign-in. In browsers the app uses
  FirebaseUI v7 core (`@firebase-oss/ui-core`) for the sign-in flow; FirebaseUI has no Vue components, so the
  sign-in screen is ours. Inside the Capacitor shells the app uses the Capacitor Firebase Authentication plugin,
  because Google refuses OAuth inside embedded WebViews. Both paths yield a Firebase ID token that the app sends
  as a Bearer token. The API verifies it with the Firebase Admin SDK for Go and maps it to a local `users` row.
  The adapters are `apps/app/src/services/auth.ts` and `apps/api/internal/auth`.
- **Push:** Firebase Cloud Messaging through the Admin SDK (`internal/notify`). Devices register their token
  with `POST /devices`. When Firebase is not configured, sends are logged and skipped.
- **Video storage:** one Vimeo account owned by the operator. The API holds the token. Creators never see it.
  Upload uses the Vimeo tus flow: the API creates the upload and returns a one-time upload link, then the device
  sends the file bytes straight to Vimeo.
- **Playback:** the API checks access and returns the Vimeo id plus unlisted hash. The app embeds the player.
- **Database:** SQLite via `modernc.org/sqlite` (pure Go). Migrations are plain SQL files embedded in the
  binary (`internal/db/migrations`) and applied in name order on start. Queries are hand-written in
  `internal/store`, one file per table.

## API packages

| Package | Role |
|---|---|
| `cmd/api` | Entry point: config, database, HTTP server, graceful shutdown |
| `internal/config` | Environment variables and `.env` loading |
| `internal/db` | Open SQLite with pragmas, run migrations |
| `internal/model` | Row structs and the JSON shapes mirrored in `packages/shared` |
| `internal/store` | SQL. No business rules |
| `internal/access` | Access resolution and tree pruning |
| `internal/server` | Routes (Go 1.22 `ServeMux` patterns), auth middleware, handlers, end-to-end tests |
| `internal/auth`, `internal/notify`, `internal/vimeo` | Firebase token verification, FCM, Vimeo client |
| `internal/httpx` | Typed errors, JSON helpers, CORS, request logging |

## Access model

Every creator has one root directory (`parent_id IS NULL`). Every video lives in exactly one directory.

A viewer can see a video when an active `access_grants` row exists for:
- the video itself, or
- any directory on the path from the video's directory up to the creator's root.

Grants reference directory and video ids, not names or paths, so renames keep access and moves change it
implicitly. Revocation sets `revoked_at` and keeps the row for the audit trail.

A `qr_codes` row holds a random `code`. The QR image encodes `wcsw://redeem/<code>`. Redeeming creates a grant
and records the code id on it. Expired or revoked codes are refused.

The in-app scanner reads the payload directly. When the phone's camera app scans the code instead, the OS opens
the app through the `wcsw` scheme. Android registers it in `AndroidManifest.xml`. iOS needs a URL Type entry in
Xcode. `services/deeplinks.ts` routes the URL to the scan page.

Resolution loads the creator's directories and videos (two queries) and walks the tree in memory. At a few
hundred videos per creator this is a few milliseconds and keeps the SQL simple.

## Data model

See `apps/api/internal/db/migrations/0001_init.sql`. Tables: users, directories, videos, qr_codes,
access_grants, devices, audit_events. Timestamps are Unix milliseconds. Ids are UUID strings.

## API surface

| Method and path | Purpose |
|---|---|
| `GET /me`, `PATCH /me`, `PUT /me/banner`, `DELETE /me/banner` | Account settings |
| `GET /library` | Creators who shared with me, plus my own collection |
| `GET /library/{creatorId}` | Directory tree pruned to what I may see |
| `POST /directories`, `PATCH /directories/{id}`, `DELETE /directories/{id}?mode=delete\|move` | Create, rename or move, delete |
| `POST /videos/upload-ticket`, `POST /videos/{id}/complete` | Two-step upload through Vimeo tus |
| `GET /videos/{id}`, `GET /videos/{id}/playback`, `PATCH /videos/{id}`, `DELETE /videos/{id}` | Read, play, rename or move, delete |
| `POST /share`, `GET /share`, `DELETE /share/{id}` | QR codes |
| `GET /access`, `DELETE /access/{id}` | View and revoke grants |
| `POST /redeem` | Scan result |
| `GET /audit` | Creator audit log |
| `POST /devices`, `DELETE /devices/{token}` | Push tokens |

Errors are `{ "error": "<code>", "message": "<text>" }` with codes `not_found`, `forbidden`, `bad_request`,
`conflict`, `unauthenticated`, `vimeo_not_configured`, `vimeo_error`, `internal`.

## Development auth bypass

With `DEV_AUTH_BYPASS=true` the API accepts `X-Dev-User: <email>` and creates that user on first request.
The app sends that header when `VITE_DEV_AUTH=true`. The API refuses to start with the bypass on when
`APP_ENV=production`.

## Vimeo privacy modes

- `unlisted` (default): the video is reachable only with its hash. Works from any origin, including the
  Capacitor WebView. Available on all plans.
- `disable`: hidden from vimeo.com and embeddable only on `VIMEO_EMBED_DOMAINS`. Needs the Core plan or above.
  The Capacitor WebView origin may not pass the domain check. Test on a device before relying on it.

## Open decisions

- Popup versus redirect for web OAuth. FirebaseUI defaults to popups. If mobile browsers block them, switch to
  `providerRedirectStrategy()` in `services/auth.ts`.
- Banner image storage in production. Development writes to `apps/api/uploads/`.
- Backups: SQLite is one file. Decide where it is copied and how often.
