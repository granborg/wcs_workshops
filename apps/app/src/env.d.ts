/// <reference types="vite/client" />

interface ImportMetaEnv {
  /** Absolute API base URL. Leave unset for same-origin "/api" (Vite proxy in dev, Go binary in prod). */
  readonly VITE_API_URL?: string
  /** host:port of the Firebase Auth emulator for local development, e.g. 127.0.0.1:9099. */
  readonly VITE_FIREBASE_AUTH_EMULATOR_HOST?: string
  readonly VITE_FIREBASE_API_KEY?: string
  readonly VITE_FIREBASE_AUTH_DOMAIN?: string
  readonly VITE_FIREBASE_PROJECT_ID?: string
  readonly VITE_FIREBASE_APP_ID?: string
  readonly VITE_FIREBASE_MESSAGING_SENDER_ID?: string
}

interface ImportMeta {
  readonly env: ImportMetaEnv
}
