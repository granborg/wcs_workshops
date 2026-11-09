/**
 * Identity provider adapter. Two implementations:
 *  - dev:      VITE_DEV_AUTH=true. Sends an x-dev-user header; the API must run with DEV_AUTH_BYPASS=true.
 *  - firebase: Google, Apple, and Microsoft sign-in through Firebase Authentication. Uses the Capacitor
 *              Firebase plugin so the same code runs on web and in the native shells.
 */

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
    return { 'x-dev-user': this.getDevEmail() }
  },
  setDevEmail(email: string) {
    safeSet(DEV_EMAIL_KEY, email.trim().toLowerCase())
  },
  getDevEmail() {
    return safeGet(DEV_EMAIL_KEY) ?? import.meta.env.VITE_DEV_USER_EMAIL ?? 'dev@example.com'
  },
}

let firebaseReady: Promise<typeof import('@capacitor-firebase/authentication')> | null = null

async function firebase() {
  if (!firebaseReady) {
    firebaseReady = (async () => {
      const { initializeApp, getApps } = await import('firebase/app')
      if (getApps().length === 0) {
        initializeApp({
          apiKey: import.meta.env.VITE_FIREBASE_API_KEY,
          authDomain: import.meta.env.VITE_FIREBASE_AUTH_DOMAIN,
          projectId: import.meta.env.VITE_FIREBASE_PROJECT_ID,
          appId: import.meta.env.VITE_FIREBASE_APP_ID,
          messagingSenderId: import.meta.env.VITE_FIREBASE_MESSAGING_SENDER_ID,
        })
      }
      return import('@capacitor-firebase/authentication')
    })()
  }
  return firebaseReady
}

const firebaseAuth: AuthService = {
  isDev: false,
  async isSignedIn() {
    const { FirebaseAuthentication } = await firebase()
    const { user } = await FirebaseAuthentication.getCurrentUser()
    return user !== null
  },
  async signIn(provider) {
    const { FirebaseAuthentication } = await firebase()
    if (provider === 'google') await FirebaseAuthentication.signInWithGoogle()
    else if (provider === 'apple') await FirebaseAuthentication.signInWithApple()
    else await FirebaseAuthentication.signInWithMicrosoft()
  },
  async signOut() {
    const { FirebaseAuthentication } = await firebase()
    await FirebaseAuthentication.signOut()
  },
  async getHeaders() {
    const { FirebaseAuthentication } = await firebase()
    const { token } = await FirebaseAuthentication.getIdToken()
    const headers: Record<string, string> = {}
    if (token) headers.Authorization = `Bearer ${token}`
    return headers
  },
  setDevEmail() {
    // not applicable
  },
  getDevEmail() {
    return ''
  },
}

export const authService: AuthService = import.meta.env.VITE_DEV_AUTH === 'true' ? devAuth : firebaseAuth
