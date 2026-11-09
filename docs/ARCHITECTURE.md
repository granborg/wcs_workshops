# Architecture

## Pieces

```
 Capacitor app (Vue 3)  --HTTPS-->  Fastify API  --REST-->  Vimeo
        |                                |
        | tus upload (direct)            | Prisma
        v                                v
      Vimeo                           SQLite (dev) / Postgres (prod)
```

- **Identity:** Firebase Authentication provides Google, Apple, and Microsoft sign-in. The app sends the Firebase
  ID token as a Bearer token. The API verifies it with `firebase-admin` and maps it to a local `User` row.
  Firebase is isolated behind `apps/app/src/services/auth.ts` and `apps/api/src/auth.ts`, so it can be swapped.
- **Push:** Firebase Cloud Messaging through `firebase-admin`. Devices register their token with `POST /devices`.
- **Video storage:** one Vimeo account owned by the operator. The API holds the token. Creators never see it.
  Upload uses the Vimeo tus flow: the API creates the upload and returns a one-time upload link, then the device
  sends the file bytes straight to Vimeo.
- **Playback:** the API checks access and returns the Vimeo id plus unlisted hash. The app embeds the player.

## Access model

Every creator has one root `Directory` (parentId null). Every `Video` lives in exactly one directory.

A viewer can see a video when an active `AccessGrant` exists for:
- the video itself, or
- any directory on the path from the video's directory up to the creator's root.

Grants reference directory and video GUIDs, not names or paths, so renames keep access and moves change it
implicitly. Revocation sets `revokedAt` and keeps the row for the audit trail.

A `QrCode` row holds a random `code`. The QR image encodes `wcsw://redeem/<code>`. Redeeming creates a grant and
records the code id on it. Expired or revoked codes are refused.

The in-app scanner reads the payload directly. When the phone's camera app scans the code instead, the OS opens
the app through the `wcsw` scheme. Android registers it in `AndroidManifest.xml`. iOS needs a URL Type entry in
Xcode. `services/deeplinks.ts` routes the URL to the scan page.

## Data model

See `apps/api/prisma/schema.prisma`. Tables: User, Directory, Video, QrCode, AccessGrant, Device, AuditEvent.

## API surface

| Method and path | Purpose |
|---|---|
| `GET /me`, `PATCH /me`, `PUT /me/banner` | Account settings |
| `GET /library` | Creators who shared with me, plus my own collection |
| `GET /library/:creatorId` | Directory tree pruned to what I may see |
| `POST /directories`, `PATCH /directories/:id`, `DELETE /directories/:id` | Create, rename or move, delete |
| `POST /videos/upload-ticket`, `POST /videos/:id/complete` | Two-step upload through Vimeo tus |
| `GET /videos/:id/playback`, `PATCH /videos/:id`, `DELETE /videos/:id` | Play, rename or move, delete |
| `POST /share`, `GET /share`, `DELETE /share/:id` | QR codes |
| `GET /access`, `DELETE /access/:id` | View and revoke grants |
| `POST /redeem` | Scan result |
| `GET /audit` | Creator audit log |
| `POST /devices` | Register push token |

## Development auth bypass

With `DEV_AUTH_BYPASS=true` the API accepts `x-dev-user: <email>` and creates that user on first request.
The app sends that header when `VITE_DEV_AUTH=true`. Never enable either in production. The API refuses to
start with the bypass on when `NODE_ENV=production`.

## Vimeo privacy modes

- `unlisted` (default): the video is reachable only with its hash. Works from any origin, including the
  Capacitor WebView. Available on all plans.
- `disable`: hidden from vimeo.com and embeddable only on `VIMEO_EMBED_DOMAINS`. Needs the Core plan or above.
  The Capacitor WebView origin may not pass the domain check. Test on a device before relying on it.

## Not yet decided

- Production database. The Prisma schema uses SQLite. Change the datasource provider to `postgresql` and re-run
  migrations when a server is chosen.
- Banner image storage in production. Development writes to `apps/api/uploads/`.
- Whether to confirm the Firebase choice for identity and push, or use another provider.
