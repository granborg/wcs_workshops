import fs from 'node:fs/promises'
import path from 'node:path'
import { randomUUID } from 'node:crypto'
import type { FastifyInstance } from 'fastify'
import type { User } from '@prisma/client'
import { z } from 'zod'
import { USER_ROLES, type Me, type UserRole } from '@wcs/shared'
import { db } from '../db.js'
import { env } from '../env.js'
import { requireCreator } from '../auth.js'
import { bannerUrl, rootDirectoryFor } from '../access.js'
import { badRequest, conflict } from '../errors.js'

async function toMe(user: User): Promise<Me> {
  const root = user.role === 'creator' ? await rootDirectoryFor(user.id) : null
  return {
    id: user.id,
    name: user.name,
    email: user.email,
    role: user.role as UserRole,
    bannerUrl: bannerUrl(user.bannerUrl),
    rootDirectoryId: root?.id ?? null,
  }
}

const patchSchema = z.object({
  name: z.string().trim().min(1).max(80).optional(),
  role: z.enum(USER_ROLES).optional(),
})

const BANNER_TYPES: Record<string, string> = {
  'image/png': '.png',
  'image/jpeg': '.jpg',
  'image/webp': '.webp',
}

async function removeBannerFile(user: User) {
  if (!user.bannerUrl?.startsWith('/static/')) return
  await fs.rm(path.resolve(env.uploadDir, path.basename(user.bannerUrl)), { force: true })
}

export default async function meRoutes(app: FastifyInstance) {
  app.get('/me', async (req) => toMe(req.user))

  app.patch('/me', async (req) => {
    const body = patchSchema.parse(req.body)
    const user = req.user

    if (body.role && body.role !== user.role) {
      if (body.role === 'creator') {
        if (!(await rootDirectoryFor(user.id))) {
          await db.directory.create({ data: { ownerId: user.id, name: 'Collection' } })
        }
      } else {
        const videos = await db.video.count({ where: { ownerId: user.id } })
        if (videos > 0) {
          throw conflict('Delete every video in your collection before switching to a viewer account')
        }
        // Deleting the root cascades to sub-directories, QR codes and grants.
        await db.directory.deleteMany({ where: { ownerId: user.id, parentId: null } })
      }
    }

    const updated = await db.user.update({
      where: { id: user.id },
      data: { name: body.name, role: body.role },
    })
    return toMe(updated)
  })

  app.put('/me/banner', async (req) => {
    requireCreator(req.user)
    const file = await req.file()
    if (!file) throw badRequest('No file uploaded')
    const ext = BANNER_TYPES[file.mimetype]
    if (!ext) throw badRequest('Banner must be PNG, JPEG, or WebP')
    const name = `${req.user.id}-${randomUUID()}${ext}`
    await fs.writeFile(path.resolve(env.uploadDir, name), await file.toBuffer())
    await removeBannerFile(req.user)
    const updated = await db.user.update({ where: { id: req.user.id }, data: { bannerUrl: `/static/${name}` } })
    return toMe(updated)
  })

  app.delete('/me/banner', async (req) => {
    await removeBannerFile(req.user)
    const updated = await db.user.update({ where: { id: req.user.id }, data: { bannerUrl: null } })
    return toMe(updated)
  })
}
