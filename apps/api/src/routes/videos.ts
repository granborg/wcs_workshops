import type { FastifyInstance, FastifyRequest } from 'fastify'
import type { Video } from '@prisma/client'
import { z } from 'zod'
import type { PlaybackInfo, UploadTicket, VideoSummary } from '@wcs/shared'
import { db } from '../db.js'
import { requireCreator } from '../auth.js'
import { audit } from '../audit.js'
import { canView, toVideoSummary } from '../access.js'
import { badRequest, forbidden, notFound } from '../errors.js'
import * as vimeo from '../vimeo.js'
import { ownedDirectory } from './directories.js'

const idParam = z.object({ id: z.string().uuid() })

const ticketSchema = z.object({
  title: z.string().trim().min(1).max(120),
  directoryId: z.string().uuid(),
  sizeBytes: z.number().int().positive(),
})

const patchSchema = z
  .object({
    title: z.string().trim().min(1).max(120).optional(),
    directoryId: z.string().uuid().optional(),
  })
  .refine((b) => b.title !== undefined || b.directoryId !== undefined, { message: 'Nothing to change' })

async function ownedVideo(req: FastifyRequest, id: string) {
  const video = await db.video.findUnique({ where: { id } })
  if (!video || video.ownerId !== req.user.id) throw notFound('Video')
  return video
}

/** Ask Vimeo for the transcode state and store it. Returns the refreshed row. */
async function refreshStatus(video: Video): Promise<Video> {
  if (!video.vimeoId || video.status === 'ready' || video.status === 'failed') return video
  const remote = await vimeo.getVideo(video.vimeoId)
  let status = video.status
  if (remote.transcode?.status === 'complete') status = 'ready'
  else if (remote.transcode?.status === 'error' || remote.upload?.status === 'error') status = 'failed'
  else if (remote.upload?.status === 'complete') status = 'processing'
  return db.video.update({
    where: { id: video.id },
    data: { status, durationSec: remote.duration > 0 ? Math.round(remote.duration) : video.durationSec },
  })
}

export default async function videoRoutes(app: FastifyInstance) {
  /** Step 1 of upload: reserve the video on Vimeo and return the tus link for the device. */
  app.post('/videos/upload-ticket', async (req, reply): Promise<UploadTicket> => {
    requireCreator(req.user)
    const body = ticketSchema.parse(req.body)
    await ownedDirectory(req, body.directoryId)
    const created = await vimeo.createTusUpload(body.title, body.sizeBytes)
    const video = await db.video.create({
      data: {
        ownerId: req.user.id,
        directoryId: body.directoryId,
        title: body.title,
        vimeoId: created.vimeoId,
        vimeoHash: created.hash,
        status: 'uploading',
      },
    })
    reply.status(201)
    return { videoId: video.id, uploadLink: created.uploadLink }
  })

  /** Step 2 of upload: the device finished sending bytes. Pull the status from Vimeo. */
  app.post('/videos/:id/complete', async (req): Promise<VideoSummary> => {
    const { id } = idParam.parse(req.params)
    const video = await ownedVideo(req, id)
    const marked = video.status === 'uploading' ? await db.video.update({ where: { id }, data: { status: 'processing' } }) : video
    return toVideoSummary(await refreshStatus(marked))
  })

  app.get('/videos/:id', async (req): Promise<VideoSummary> => {
    const { id } = idParam.parse(req.params)
    const video = await db.video.findUnique({ where: { id } })
    if (!video || !(await canView(req.user.id, video))) throw notFound('Video')
    return toVideoSummary(video.status === 'processing' ? await refreshStatus(video) : video)
  })

  app.get('/videos/:id/playback', async (req): Promise<PlaybackInfo> => {
    const { id } = idParam.parse(req.params)
    const video = await db.video.findUnique({ where: { id } })
    if (!video) throw notFound('Video')
    if (!(await canView(req.user.id, video))) throw forbidden('No access to this video')
    if (!video.vimeoId) throw badRequest('Video has not been uploaded yet')
    return {
      videoId: video.id,
      vimeoId: video.vimeoId,
      hash: video.vimeoHash,
      embedUrl: vimeo.embedUrl(video.vimeoId, video.vimeoHash),
    }
  })

  /** Rename and/or move. Moving changes inherited access implicitly; direct grants follow the video. */
  app.patch('/videos/:id', async (req): Promise<VideoSummary> => {
    requireCreator(req.user)
    const { id } = idParam.parse(req.params)
    const body = patchSchema.parse(req.body)
    const video = await ownedVideo(req, id)
    if (body.directoryId !== undefined) await ownedDirectory(req, body.directoryId)

    const updated = await db.video.update({
      where: { id },
      data: { title: body.title, directoryId: body.directoryId },
    })
    if (body.title !== undefined && video.vimeoId) {
      try {
        await vimeo.renameVideo(video.vimeoId, body.title)
      } catch (err) {
        req.log.warn({ err, videoId: id }, 'Vimeo rename failed')
      }
    }
    if (body.directoryId !== undefined && body.directoryId !== video.directoryId) {
      await audit({
        type: 'video.moved',
        creatorId: req.user.id,
        actorId: req.user.id,
        ip: req.ip,
        videoId: id,
        detail: { from: video.directoryId, to: body.directoryId },
      })
    }
    return toVideoSummary(updated)
  })

  app.delete('/videos/:id', async (req, reply) => {
    requireCreator(req.user)
    const { id } = idParam.parse(req.params)
    const video = await ownedVideo(req, id)
    if (video.vimeoId) {
      try {
        await vimeo.deleteVideo(video.vimeoId)
      } catch (err) {
        req.log.warn({ err, videoId: id }, 'Vimeo delete failed; removing local record anyway')
      }
    }
    await db.video.delete({ where: { id } })
    await audit({ type: 'video.deleted', creatorId: req.user.id, actorId: req.user.id, ip: req.ip, videoId: id })
    return reply.status(204).send()
  })
}
