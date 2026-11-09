import type { FastifyInstance } from 'fastify'
import { z } from 'zod'
import { QR_SCHEME, type RedeemResult } from '@wcs/shared'
import { db } from '../db.js'
import { audit } from '../audit.js'
import { notifyUser } from '../notify.js'
import { rootDirectoryFor, toCreatorSummary } from '../access.js'
import { badRequest, notFound } from '../errors.js'

const bodySchema = z.object({ code: z.string().trim().min(8).max(200) })

/** Accepts the bare code, the wcsw://redeem/<code> payload, or any URL whose last path segment is the code. */
function extractCode(raw: string): string {
  if (raw.startsWith(QR_SCHEME)) return raw.slice(QR_SCHEME.length)
  if (raw.includes('://')) {
    try {
      const last = new URL(raw).pathname.split('/').filter(Boolean).pop()
      if (last) return last
    } catch {
      // fall through and treat the input as a bare code
    }
  }
  return raw
}

export default async function redeemRoutes(app: FastifyInstance) {
  app.post('/redeem', async (req): Promise<RedeemResult> => {
    const { code: raw } = bodySchema.parse(req.body)
    const code = extractCode(raw)
    const qr = await db.qrCode.findUnique({
      where: { code },
      include: { creator: true },
    })
    if (!qr || qr.revokedAt) throw notFound('QR code')
    if (qr.expiresAt && qr.expiresAt <= new Date()) throw badRequest('This QR code has expired')
    if (qr.creatorId === req.user.id) throw badRequest('This is your own collection')

    const root = await rootDirectoryFor(qr.creatorId)
    if (!root) throw notFound('Collection')

    const existing = await db.accessGrant.findFirst({
      where: {
        userId: req.user.id,
        revokedAt: null,
        directoryId: qr.directoryId,
        videoId: qr.videoId,
      },
    })

    if (!existing) {
      const grant = await db.accessGrant.create({
        data: {
          userId: req.user.id,
          directoryId: qr.directoryId,
          videoId: qr.videoId,
          qrCodeId: qr.id,
        },
      })
      await audit({
        type: 'access.granted',
        creatorId: qr.creatorId,
        actorId: req.user.id,
        subjectId: req.user.id,
        ip: req.ip,
        directoryId: qr.directoryId,
        videoId: qr.videoId,
        qrCodeId: qr.id,
        detail: { grantId: grant.id },
      })
      const what = qr.directoryId ? 'a folder' : 'a video'
      void notifyUser(req.user.id, 'New access', `${qr.creator.name} shared ${what} with you`, {
        creatorId: qr.creatorId,
        ...(qr.directoryId ? { directoryId: qr.directoryId } : {}),
        ...(qr.videoId ? { videoId: qr.videoId } : {}),
      })
    }

    return {
      creator: toCreatorSummary(qr.creator, root.id, req.user.id),
      directoryId: qr.directoryId,
      videoId: qr.videoId,
      added: !existing,
    }
  })
}
