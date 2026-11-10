/**
 * Identity adapter over Firebase Authentication. Two implementations, chosen at startup:
 *  - web:    FirebaseUI v7 core (@firebase-oss/ui-core) over the Firebase JS SDK. Runs in browsers.
 *  - native: Capacitor Firebase Authentication plugin. Runs inside the iOS and Android shells, where
 *            Google refuses OAuth in embedded WebViews, so the native provider SDKs are required.
 *
 * Local development uses the Firebase Auth emulator: set VITE_FIREBASE_AUTH_EMULATOR_HOST (for example
 * 127.0.0.1:9099) and no real Firebase project is needed. The emulator offers fake Google, Apple and
 * Microsoft accounts through the same buttons, and accepts any email and password.
 *
 * Email and password accounts need the Email/Password provider turned on in the Firebase console.
 */

import { Capacitor } from '@capacitor/core'

export type Provider = 'google' | 'apple' | 'microsoft'

export interface AuthService {
  isSignedIn(): Promise<boolean>
  signIn(provider: Provider): Promise<void>
  signInWithEmail(email: string, password: string): Promise<void>
  createAccount(email: string, password: string): Promise<void>
  sendPasswordReset(email: string): Promise<void>
  signOut(): Promise<void>
  getHeaders(): Promise<Record<string, string>>
}

const EMULATOR = import.meta.env.VITE_FIREBASE_AUTH_EMULATOR_HOST

// Against the emulator any non-empty config works; the demo- project prefix needs no real project.
// `||`, not `??`: Vite turns an empty line such as `VITE_FIREBASE_API_KEY=` into "", not undefined.
const firebaseConfig = {
  apiKey: import.meta.env.VITE_FIREBASE_API_KEY || (EMULATOR ? 'demo-api-key' : undefined),
  authDomain: import.meta.env.VITE_FIREBASE_AUTH_DOMAIN || (EMULATOR ? 'demo-wcs.firebaseapp.com' : undefined),
  projectId: import.meta.env.VITE_FIREBASE_PROJECT_ID || (EMULATOR ? 'demo-wcs' : undefined),
  appId: import.meta.env.VITE_FIREBASE_APP_ID || (EMULATOR ? '1:0:web:demo' : undefined),
  messagingSenderId: import.meta.env.VITE_FIREBASE_MESSAGING_SENDER_ID || undefined,
}

async function firebaseApp() {
  const { getApps, initializeApp } = await import('firebase/app')
  return getApps()[0] ?? initializeApp(firebaseConfig)
}

function bearer(token: string | null | undefined): Record<string, string> {
  return token ? { Authorization: `Bearer ${token}` } : {}
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
    const { getAuth, connectAuthEmulator } = await import('firebase/auth')
    const { initializeUI } = await import('@firebase-oss/ui-core')
    const auth = getAuth(app)
    if (EMULATOR) connectAuthEmulator(auth, `http://${EMULATOR}`, { disableWarnings: true })
    // Default behaviors: popup OAuth strategy. Swap in providerRedirectStrategy() from
    // @firebase-oss/ui-core if popups prove unreliable on mobile browsers.
    return { ui: initializeUI({ app, auth }), auth }
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
  async signInWithEmail(email, password) {
    const { auth } = await webUI()
    const { signInWithEmailAndPassword } = await import('firebase/auth')
    await signInWithEmailAndPassword(auth, email, password)
  },
  async createAccount(email, password) {
    const { auth } = await webUI()
    const { createUserWithEmailAndPassword } = await import('firebase/auth')
    await createUserWithEmailAndPassword(auth, email, password)
  },
  async sendPasswordReset(email) {
    const { auth } = await webUI()
    const { sendPasswordResetEmail } = await import('firebase/auth')
    await sendPasswordResetEmail(auth, email)
  },
  async signOut() {
    const { auth } = await webUI()
    await auth.signOut()
  },
  async getHeaders() {
    const { auth } = await webUI()
    await authReady(auth)
    return bearer(await auth.currentUser?.getIdToken())
  },
}

/* ---------------- native: Capacitor Firebase plugin ---------------- */

let pluginPromise: Promise<typeof import('@capacitor-firebase/authentication').FirebaseAuthentication> | null = null

function nativePlugin() {
  pluginPromise ??= (async () => {
    await firebaseApp()
    const { FirebaseAuthentication } = await import('@capacitor-firebase/authentication')
    if (EMULATOR) {
      const [host, port] = EMULATOR.split(':')
      await FirebaseAuthentication.useEmulator({ host: host ?? '127.0.0.1', port: Number(port ?? 9099) })
    }
    return FirebaseAuthentication
  })()
  return pluginPromise
}

const nativeAuth: AuthService = {
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
  async signInWithEmail(email, password) {
    await (await nativePlugin()).signInWithEmailAndPassword({ email, password })
  },
  async createAccount(email, password) {
    await (await nativePlugin()).createUserWithEmailAndPassword({ email, password })
  },
  async sendPasswordReset(email) {
    await (await nativePlugin()).sendPasswordResetEmail({ email })
  },
  async signOut() {
    await (await nativePlugin()).signOut()
  },
  async getHeaders() {
    const { token } = await (await nativePlugin()).getIdToken()
    return bearer(token)
  },
}

export const authService: AuthService = Capacitor.isNativePlatform() ? nativeAuth : webAuth
