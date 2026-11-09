/**
 * Identity provider adapter. Three implementations, chosen at startup:
 *  - dev:    VITE_DEV_AUTH=true. Sends an X-Dev-User header; the API must run with DEV_AUTH_BYPASS=true.
 *  - web:    FirebaseUI v7 core (@firebase-oss/ui-core) over the Firebase JS SDK. Runs in browsers.
 *  - native: Capacitor Firebase Authentication plugin. Runs inside the iOS and Android shells, where
 *            Google refuses OAuth in embedded WebViews, so the native provider SDKs are required.
 *
 * All three produce a Firebase ID token (or the dev header) that the API verifies.
 */

import { Capacitor } from '@capacitor/core'

export type Provider = 'google' | 'apple' | 'microsoft'

export interface AuthService {
  readonly isDev: boolean
  isSignedIn(): Promise<boolean>
  signIn(provider: Provider): Promise<void>
  signOut(): Promise<void>
  getHeaders(): Promise<Record<string, string>>
  /** Dev mode only: switch the acting user. */
  setDevEmail(email: string): void
  getDevEmail(): string
}

const firebaseConfig = {
  apiKey: import.meta.env.VITE_FIREBASE_API_KEY,
  authDomain: import.meta.env.VITE_FIREBASE_AUTH_DOMAIN,
  projectId: import.meta.env.VITE_FIREBASE_PROJECT_ID,
  appId: import.meta.env.VITE_FIREBASE_APP_ID,
  messagingSenderId: import.meta.env.VITE_FIREBASE_MESSAGING_SENDER_ID,
}

async function firebaseApp() {
  const { getApps, initializeApp } = await import('firebase/app')
  return getApps()[0] ?? initializeApp(firebaseConfig)
}

const noDev = {
  setDevEmail() {
    // not applicable outside dev mode
  },
  getDevEmail() {
    return ''
  },
}

/* ---------------- dev ---------------- */

const DEV_EMAIL_KEY = 'wcsw.devEmail'
const DEV_SIGNED_IN_KEY = 'wcsw.devSignedIn'

function safeGet(key: string): string | null {
  try {
    return localStorage.getItem(key)
  } catch {
    return null
  }
}

function safeSet(key: string, value: string | null) {
  try {
    if (value === null) localStorage.removeItem(key)
    else localStorage.setItem(key, value)
  } catch {
    // storage unavailable; the session simply does not persist
  }
}

const devAuth: AuthService = {
  isDev: true,
  async isSignedIn() {
    return safeGet(DEV_SIGNED_IN_KEY) === '1'
  },
  async signIn() {
    if (!this.getDevEmail()) this.setDevEmail(import.meta.env.VITE_DEV_USER_EMAIL ?? 'dev@example.com')
    safeSet(DEV_SIGNED_IN_KEY, '1')
  },
  async signOut() {
    safeSet(DEV_SIGNED_IN_KEY, null)
  },
  async getHeaders() {
    return { 'X-Dev-User': this.getDevEmail() }
  },
  setDevEmail(email: string) {
    safeSet(DEV_EMAIL_KEY, email.trim().toLowerCase())
  },
  getDevEmail() {
    return safeGet(DEV_EMAIL_KEY) ?? import.meta.env.VITE_DEV_USER_EMAIL ?? 'dev@example.com'
  },
}

/* ---------------- web: FirebaseUI core ---------------- */

type WebUI = {
  ui: import('@firebase-oss/ui-core').FirebaseUIStore
  auth: import('firebase/auth').Auth
}

let webUIPromise: Promise<WebUI> | null = null

function webUI(): Promise<WebUI> {
  webUIPromise ??= (async () => {
    const app = await firebaseApp()
    const { getAuth } = await import('firebase/auth')
    const { initializeUI } = await import('@firebase-oss/ui-core')
    const auth = getAuth(app)
    // Default behaviors: popup OAuth strategy and reCAPTCHA config. Swap in providerRedirectStrategy()
    // from @firebase-oss/ui-core if popups prove unreliable on mobile browsers.
    const ui = initializeUI({ app, auth })
    return { ui, auth }
  })()
  return webUIPromise
}

/** Resolves once Firebase has restored any persisted session. */
function authReady(auth: import('firebase/auth').Auth): Promise<void> {
  return new Promise((resolve) => {
    const stop = auth.onAuthStateChanged(() => {
      stop()
      resolve()
    })
  })
}

async function providerFor(p: Provider) {
  const { GoogleAuthProvider, OAuthProvider } = await import('firebase/auth')
  if (p === 'google') return new GoogleAuthProvider()
  const provider = new OAuthProvider(p === 'apple' ? 'apple.com' : 'microsoft.com')
  provider.addScope('email')
  if (p === 'apple') provider.addScope('name')
  return provider
}

const webAuth: AuthService = {
  ...noDev,
  isDev: false,
  async isSignedIn() {
    const { auth } = await webUI()
    await authReady(auth)
    return auth.currentUser !== null
  },
  async signIn(provider) {
    const { ui } = await webUI()
    const { signInWithProvider } = await import('@firebase-oss/ui-core')
    // FirebaseUI wraps popup handling, pending-credential linking and error translation.
    await signInWithProvider(ui.get(), await providerFor(provider))
  },
  async signOut() {
    const { auth } = await webUI()
    await auth.signOut()
  },
  async getHeaders() {
    const { auth } = await webUI()
    await authReady(auth)
    const token = await auth.currentUser?.getIdToken()
    const headers: Record<string, string> = {}
    if (token) headers.Authorization = `Bearer ${token}`
    return headers
  },
}

/* ---------------- native: Capacitor Firebase plugin ---------------- */

async function nativePlugin() {
  await firebaseApp()
  const { FirebaseAuthentication } = await import('@capacitor-firebase/authentication')
  return FirebaseAuthentication
}

const nativeAuth: AuthService = {
  ...noDev,
  isDev: false,
  async isSignedIn() {
    const { user } = await (await nativePlugin()).getCurrentUser()
    return user !== null
  },
  async signIn(provider) {
    const plugin = await nativePlugin()
    if (provider === 'google') await plugin.signInWithGoogle()
    else if (provider === 'apple') await plugin.signInWithApple()
    else await plugin.signInWithMicrosoft()
  },
  async signOut() {
    await (await nativePlugin()).signOut()
  },
  async getHeaders() {
    const { token } = await (await nativePlugin()).getIdToken()
    const headers: Record<string, string> = {}
    if (token) headers.Authorization = `Bearer ${token}`
    return headers
  },
}

function pick(): AuthService {
  if (import.meta.env.VITE_DEV_AUTH === 'true') return devAuth
  return Capacitor.isNativePlatform() ? nativeAuth : webAuth
}

export const authService: AuthService = pick()
