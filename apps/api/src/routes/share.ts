import { randomBytes } from 'node:crypto'
import type { FastifyInstance } from 'fastify'
import type { QrCode } from '@prisma/client'
import { z } from 'zod'
import { QR_SCHEME, type QrCodeInfo } from '@wcs/shared'
import { db } from '../db.js'
import { requireCreator } from '../auth.js'
import { audit } from '../audit.js'
import { badRequest, notFound } from '../errors.js'

const target = z
  .object({
    directoryId: z.string().uuid().optional(),
    videoId: z.string().uuid().optional(),
  })
  .refine((t) => (t.directoryId ? 1 : 0) + (t.videoId ? 1 : 0) === 1, {
    message: 'Provide exactly one of directoryId or videoId',
  })

const createSchema = target.and(
  z.object({
    expiresAt: z.string().datetime().optional(),
  }),
)

const idParam = z.object({ id: z.string().uuid() })

function toInfo(q: QrCode & { _count: { grants: number } }): QrCodeInfo {
  return {
    id: q.id,
    code: q.code,
    payload: `${QR_SCHEME}${q.code}`,
    directoryId: q.directoryId,
    videoId: q.videoId,
    expiresAt: q.expiresAt?.toISOString() ?? null,
    revokedAt: q.revokedAt?.toISOString() ?? null,
    createdAt: q.createdAt.toISOString(),
    redemptions: q._count.grants,
  }
}

async function assertOwnedTarget(userId: string, t: { directoryId?: string; videoId?: string }) {
  if (t.directoryId) {
    const d = await db.directory.findUnique({ where: { id: t.directoryId } })
    if (!d || d.ownerId !== userId) throw notFound('Directory')
  } else if (t.videoId) {
    const v = await db.video.findUnique({ where: { id: t.videoId } })
    if (!v || v.ownerId !== userId) throw notFound('Video')
  }
}

export default async function shareRoutes(app: FastifyInstance) {
  /** Create a QR code for a directory or video, with an optional expiry. */
  app.post('/share', async (req, reply): Promise<QrCodeInfo> => {
    requireCreator(req.user)
    const body = createSchema.parse(req.body)
    await assertOwnedTarget(req.user.id, body)
    if (body.expiresAt && new Date(body.expiresAt) <= new Date()) {
      throw badRequest('Expiration must be in the future')
    }
    const qr = await db.qrCode.create({
      data: {
        code: randomBytes(18).toString('base64url'),
        creatorId: req.user.id,
        directoryId: body.directoryId ?? null,
        videoId: body.videoId ?? null,
        expiresAt: body.expiresAt ? new Date(body.expiresAt) : null,
      },
      include: { _count: { select: { grants: true } } },
    })
    await audit({
      type: 'qr.created',
      creatorId: req.user.id,
      actorId: req.user.id,
      ip: req.ip,
      directoryId: qr.directoryId,
      videoId: qr.videoId,
      qrCodeId: qr.id,
      detail: { expiresAt: qr.expiresAt?.toISOString() ?? null },
    })
    reply.status(201)
    return toInfo(qr)
  })

  /** List QR codes for a directory or video, newest first. */
  app.get('/share', async (req): Promise<QrCodeInfo[]> => {
    requireCreator(req.user)
    const q = target.parse(req.query)
    await assertOwnedTarget(req.user.id, q)
    const codes = await db.qrCode.findMany({
      where: { creatorId: req.user.id, directoryId: q.directoryId ?? null, videoId: q.videoId ?? null },
      include: { _count: { select: { grants: true } } },
      orderBy: { createdAt: 'desc' },
    })
    return codes.map(toInfo)
  })

  /** Revoke a QR code. Grants already redeemed with it stay in place. */
  app.delete('/share/:id', async (req): Promise<QrCodeInfo> => {
    requireCreator(req.user)
    const { id } = idParam.parse(req.params)
    const existing = await db.qrCode.findUnique({ where: { id } })
    if (!existing || existing.creatorId !== req.user.id) throw notFound('QR code')
    const qr = await db.qrCode.update({
      where: { id },
      data: { revokedAt: existing.revokedAt ?? new Date() },
      include: { _count: { select: { grants: true } } },
    })
    if (!existing.revokedAt) {
      await audit({
        type: 'qr.revoked',
        creatorId: req.user.id,
        actorId: req.user.id,
        ip: req.ip,
        directoryId: qr.directoryId,
        videoId: qr.videoId,
        qrCodeId: qr.id,
      })
    }
    return toInfo(qr)
  })
}
