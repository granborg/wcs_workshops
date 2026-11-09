import type { FastifyInstance, FastifyRequest } from 'fastify'
import type { User } from '@prisma/client'
import { db } from './db.js'
import { env } from './env.js'
import { HttpError } from './errors.js'

declare module 'fastify' {
  interface FastifyRequest {
    user: User
  }
}

interface Identity {
  authUid: string
  email: string | null
  name: string
}

let firebaseAuth: import('firebase-admin/auth').Auth | undefined

async function getFirebaseAuth() {
  if (!firebaseAuth) {
    const { getApps, initializeApp, applicationDefault } = await import('firebase-admin/app')
    const { getAuth } = await import('firebase-admin/auth')
    const app = getApps()[0] ?? initializeApp({ credential: applicationDefault() })
    firebaseAuth = getAuth(app)
  }
  return firebaseAuth
}

async function identify(req: FastifyRequest): Promise<Identity> {
  const devUser = req.headers['x-dev-user']
  if (env.devAuthBypass && typeof devUser === 'string' && devUser.length > 0) {
    const email = devUser.trim().toLowerCase()
    return { authUid: `dev:${email}`, email, name: email.split('@')[0] ?? email }
  }

  const header = req.headers.authorization
  if (!header?.startsWith('Bearer ')) {
    throw new HttpError(401, 'unauthenticated', 'Missing bearer token')
  }
  const token = header.slice('Bearer '.length)
  try {
    const auth = await getFirebaseAuth()
    const decoded = await auth.verifyIdToken(token)
    return {
      authUid: decoded.uid,
      email: decoded.email ?? null,
      name: decoded.name ?? decoded.email?.split('@')[0] ?? 'New user',
    }
  } catch {
    throw new HttpError(401, 'unauthenticated', 'Invalid or expired token')
  }
}

/** Resolve the identity to a User row, creating it on first sign-in. */
async function upsertUser(identity: Identity): Promise<User> {
  const existing = await db.user.findUnique({ where: { authUid: identity.authUid } })
  if (existing) return existing
  return db.user.create({
    data: { authUid: identity.authUid, email: identity.email, name: identity.name },
  })
}

const PUBLIC_PREFIXES = ['/health', '/static/']

/** Registers an onRequest hook that sets request.user for every route except public ones. */
export function registerAuth(app: FastifyInstance) {
  app.addHook('onRequest', async (req) => {
    if (PUBLIC_PREFIXES.some((p) => req.url.startsWith(p))) return
    const identity = await identify(req)
    req.user = await upsertUser(identity)
  })
}

export function requireCreator(user: User) {
  if (user.role !== 'creator') throw new HttpError(403, 'forbidden', 'Creator account required')
}
