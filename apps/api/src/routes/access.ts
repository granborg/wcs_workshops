import type { FastifyInstance } from 'fastify'
import { z } from 'zod'
import type { AccessGrantInfo } from '@wcs/shared'
import { db } from '../db.js'
import { requireCreator } from '../auth.js'
import { audit } from '../audit.js'
import { notFound } from '../errors.js'

const target = z
  .object({
    directoryId: z.string().uuid().optional(),
    videoId: z.string().uuid().optional(),
  })
  .refine((t) => (t.directoryId ? 1 : 0) + (t.videoId ? 1 : 0) === 1, {
    message: 'Provide exactly one of directoryId or videoId',
  })

const idParam = z.object({ id: z.string().uuid() })

export default async function accessRoutes(app: FastifyInstance) {
  /** Active grants directly on a directory or video. Inherited access is visible on the ancestor. */
  app.get('/access', async (req): Promise<AccessGrantInfo[]> => {
    requireCreator(req.user)
    const q = target.parse(req.query)
    const grants = await db.accessGrant.findMany({
      where: {
        revokedAt: null,
        ...(q.directoryId
          ? { directoryId: q.directoryId, directory: { ownerId: req.user.id } }
          : { videoId: q.videoId, video: { ownerId: req.user.id } }),
      },
      include: { user: { select: { id: true, name: true } } },
      orderBy: { createdAt: 'desc' },
    })
    return grants.map((g) => ({
      id: g.id,
      user: g.user,
      directoryId: g.directoryId,
      videoId: g.videoId,
      qrCodeId: g.qrCodeId,
      createdAt: g.createdAt.toISOString(),
    }))
  })

  /** Revoke one grant. The row is kept with revokedAt set, for the audit trail. */
  app.delete('/access/:id', async (req, reply) => {
    requireCreator(req.user)
    const { id } = idParam.parse(req.params)
    const grant = await db.accessGrant.findUnique({
      where: { id },
      include: { directory: { select: { ownerId: true } }, video: { select: { ownerId: true } } },
    })
    const owner = grant?.directory?.ownerId ?? grant?.video?.ownerId
    if (!grant || owner !== req.user.id) throw notFound('Access grant')
    if (!grant.revokedAt) {
      await db.accessGrant.update({ where: { id }, data: { revokedAt: new Date() } })
      await audit({
        type: 'access.revoked',
        creatorId: req.user.id,
        actorId: req.user.id,
        subjectId: grant.userId,
        ip: req.ip,
        directoryId: grant.directoryId,
        videoId: grant.videoId,
        qrCodeId: grant.qrCodeId,
      })
    }
    return reply.status(204).send()
  })
}
