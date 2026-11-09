import type { FastifyInstance } from 'fastify'
import { z } from 'zod'
import { db } from '../db.js'

const registerSchema = z.object({
  token: z.string().min(10).max(4096),
  platform: z.enum(['ios', 'android', 'web']),
})

const tokenParam = z.object({ token: z.string().min(10).max(4096) })

export default async function deviceRoutes(app: FastifyInstance) {
  /** Register or re-assign a push token to the signed-in user. */
  app.post('/devices', async (req, reply) => {
    const body = registerSchema.parse(req.body)
    await db.device.upsert({
      where: { pushToken: body.token },
      create: { userId: req.user.id, pushToken: body.token, platform: body.platform },
      update: { userId: req.user.id, platform: body.platform },
    })
    return reply.status(204).send()
  })

  /** Remove a push token, for sign-out. */
  app.delete('/devices/:token', async (req, reply) => {
    const { token } = tokenParam.parse(req.params)
    await db.device.deleteMany({ where: { pushToken: token, userId: req.user.id } })
    return reply.status(204).send()
  })
}
