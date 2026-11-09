import fs from 'node:fs'
import path from 'node:path'
import Fastify from 'fastify'
import cors from '@fastify/cors'
import multipart from '@fastify/multipart'
import fastifyStatic from '@fastify/static'
import { ZodError } from 'zod'
import { env } from './env.js'
import { registerAuth } from './auth.js'
import { HttpError } from './errors.js'
import meRoutes from './routes/me.js'
import libraryRoutes from './routes/library.js'
import directoryRoutes from './routes/directories.js'
import videoRoutes from './routes/videos.js'
import shareRoutes from './routes/share.js'
import accessRoutes from './routes/access.js'
import redeemRoutes from './routes/redeem.js'
import auditRoutes from './routes/audit.js'
import deviceRoutes from './routes/devices.js'

const app = Fastify({ logger: true, trustProxy: true })

await app.register(cors, { origin: env.corsOrigins, credentials: true })
await app.register(multipart, { limits: { fileSize: 5 * 1024 * 1024, files: 1 } })

const uploadRoot = path.resolve(env.uploadDir)
fs.mkdirSync(uploadRoot, { recursive: true })
await app.register(fastifyStatic, { root: uploadRoot, prefix: '/static/' })

app.setErrorHandler((err: unknown, _req, reply) => {
  if (err instanceof HttpError) {
    return reply.status(err.statusCode).send({ error: err.code, message: err.message })
  }
  if (err instanceof ZodError) {
    const message = err.issues.map((i) => `${i.path.join('.') || 'body'}: ${i.message}`).join('; ')
    return reply.status(400).send({ error: 'validation', message })
  }
  const e = err as { statusCode?: unknown; message?: unknown }
  const status = typeof e.statusCode === 'number' ? e.statusCode : 500
  if (status >= 500) app.log.error(err)
  const message = status >= 500 ? 'Internal server error' : String(e.message ?? 'Request failed')
  return reply.status(status).send({ error: status >= 500 ? 'internal' : 'error', message })
})

app.get('/health', () => ({ ok: true }))

registerAuth(app)

await app.register(meRoutes)
await app.register(libraryRoutes)
await app.register(directoryRoutes)
await app.register(videoRoutes)
await app.register(shareRoutes)
await app.register(accessRoutes)
await app.register(redeemRoutes)
await app.register(auditRoutes)
await app.register(deviceRoutes)

await app.listen({ port: env.port, host: env.host })
if (env.devAuthBypass) app.log.warn('DEV_AUTH_BYPASS is on: x-dev-user header is trusted')
