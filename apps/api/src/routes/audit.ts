import type { FastifyInstance } from 'fastify'
import { z } from 'zod'
import type { AuditEventInfo, AuditType } from '@wcs/shared'
import { db } from '../db.js'
import { requireCreator } from '../auth.js'

const querySchema = z.object({
  limit: z.coerce.number().int().min(1).max(200).default(50),
  /** ISO timestamp; return events strictly before it. */
  before: z.string().datetime().optional(),
})

export default async function auditRoutes(app: FastifyInstance) {
  app.get('/audit', async (req): Promise<AuditEventInfo[]> => {
    requireCreator(req.user)
    const q = querySchema.parse(req.query)
    const events = await db.auditEvent.findMany({
      where: { creatorId: req.user.id, ...(q.before ? { at: { lt: new Date(q.before) } } : {}) },
      orderBy: { at: 'desc' },
      take: q.limit,
    })

    const userIds = new Set<string>()
    for (const e of events) {
      if (e.actorId) userIds.add(e.actorId)
      if (e.subjectId) userIds.add(e.subjectId)
    }
    const users = userIds.size
      ? await db.user.findMany({ where: { id: { in: [...userIds] } }, select: { id: true, name: true } })
      : []
    const byId = new Map(users.map((u) => [u.id, u]))
    const ref = (id: string | null) => (id ? (byId.get(id) ?? { id, name: 'Deleted user' }) : null)

    return events.map((e) => ({
      id: e.id,
      at: e.at.toISOString(),
      type: e.type as AuditType,
      actor: ref(e.actorId),
      subject: ref(e.subjectId),
      ip: e.ip,
      directoryId: e.directoryId,
      videoId: e.videoId,
      qrCodeId: e.qrCodeId,
      detail: e.detail ? (JSON.parse(e.detail) as Record<string, unknown>) : null,
    }))
  })
}
