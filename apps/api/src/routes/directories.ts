import type { FastifyInstance, FastifyRequest } from 'fastify'
import { z } from 'zod'
import { db } from '../db.js'
import { requireCreator } from '../auth.js'
import { audit } from '../audit.js'
import { loadCollection, subtreeIds } from '../access.js'
import { badRequest, notFound } from '../errors.js'
import * as vimeo from '../vimeo.js'

const idParam = z.object({ id: z.string().uuid() })

const createSchema = z.object({
  name: z.string().trim().min(1).max(120),
  parentId: z.string().uuid(),
})

const patchSchema = z
  .object({
    name: z.string().trim().min(1).max(120).optional(),
    parentId: z.string().uuid().optional(),
  })
  .refine((b) => b.name !== undefined || b.parentId !== undefined, { message: 'Nothing to change' })

const deleteQuery = z.discriminatedUnion('mode', [
  z.object({ mode: z.literal('delete') }),
  z.object({ mode: z.literal('move'), targetDirectoryId: z.string().uuid() }),
])

export async function ownedDirectory(req: FastifyRequest, id: string) {
  const dir = await db.directory.findUnique({ where: { id } })
  if (!dir || dir.ownerId !== req.user.id) throw notFound('Directory')
  return dir
}

export default async function directoryRoutes(app: FastifyInstance) {
  app.post('/directories', async (req, reply) => {
    requireCreator(req.user)
    const body = createSchema.parse(req.body)
    await ownedDirectory(req, body.parentId)
    const dir = await db.directory.create({
      data: { ownerId: req.user.id, parentId: body.parentId, name: body.name },
    })
    return reply.status(201).send(dir)
  })

  /** Rename and/or move. Renaming never touches grants. Moving changes inherited access implicitly. */
  app.patch('/directories/:id', async (req) => {
    requireCreator(req.user)
    const { id } = idParam.parse(req.params)
    const body = patchSchema.parse(req.body)
    const dir = await ownedDirectory(req, id)

    if (body.parentId !== undefined) {
      if (dir.parentId === null) throw badRequest('The root directory cannot be moved')
      if (body.parentId === dir.id) throw badRequest('A directory cannot be moved into itself')
      await ownedDirectory(req, body.parentId)
      const col = await loadCollection(req.user.id)
      if (col && subtreeIds(dir.id, col.dirs).includes(body.parentId)) {
        throw badRequest('A directory cannot be moved into one of its own sub-directories')
      }
    }

    const updated = await db.directory.update({
      where: { id },
      data: { name: body.name, parentId: body.parentId },
    })
    if (body.parentId !== undefined && body.parentId !== dir.parentId) {
      await audit({
        type: 'directory.moved',
        creatorId: req.user.id,
        actorId: req.user.id,
        ip: req.ip,
        directoryId: dir.id,
        detail: { from: dir.parentId, to: body.parentId },
      })
    }
    return updated
  })

  /**
   * Delete a directory and its sub-directories.
   * mode=delete removes every video inside (also on Vimeo).
   * mode=move relocates every video inside to targetDirectoryId first.
   */
  app.delete('/directories/:id', async (req, reply) => {
    requireCreator(req.user)
    const { id } = idParam.parse(req.params)
    const query = deleteQuery.parse(req.query)
    const dir = await ownedDirectory(req, id)
    if (dir.parentId === null) throw badRequest('The root directory cannot be deleted')

    const col = await loadCollection(req.user.id)
    if (!col) throw notFound('Collection')
    const subtree = subtreeIds(dir.id, col.dirs)
    const videos = col.videos.filter((v) => subtree.includes(v.directoryId))

    if (query.mode === 'move') {
      if (subtree.includes(query.targetDirectoryId)) {
        throw badRequest('Target directory is inside the directory being deleted')
      }
      await ownedDirectory(req, query.targetDirectoryId)
      if (videos.length > 0) {
        await db.video.updateMany({
          where: { id: { in: videos.map((v) => v.id) } },
          data: { directoryId: query.targetDirectoryId },
        })
      }
    } else {
      for (const v of videos) {
        if (v.vimeoId) {
          try {
            await vimeo.deleteVideo(v.vimeoId)
          } catch (err) {
            req.log.warn({ err, videoId: v.id }, 'Vimeo delete failed; removing local record anyway')
          }
        }
        await db.video.delete({ where: { id: v.id } })
        await audit({ type: 'video.deleted', creatorId: req.user.id, actorId: req.user.id, ip: req.ip, videoId: v.id })
      }
    }

    await db.directory.delete({ where: { id } })
    await audit({
      type: 'directory.deleted',
      creatorId: req.user.id,
      actorId: req.user.id,
      ip: req.ip,
      directoryId: id,
      detail: { mode: query.mode, videos: videos.length },
    })
    return reply.status(204).send()
  })
}
