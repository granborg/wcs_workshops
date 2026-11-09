import type { FastifyInstance } from 'fastify'
import { z } from 'zod'
import type { CreatorSummary, LibraryTree } from '@wcs/shared'
import { db } from '../db.js'
import { activeGrantsFor, buildTree, loadCollection, rootDirectoryFor, toCreatorSummary } from '../access.js'
import { forbidden, notFound } from '../errors.js'

const creatorParam = z.object({ creatorId: z.string().uuid() })

export default async function libraryRoutes(app: FastifyInstance) {
  /** Creators who shared with me. My own collection comes first when I am a creator. */
  app.get('/library', async (req): Promise<CreatorSummary[]> => {
    const me = req.user
    const out: CreatorSummary[] = []

    if (me.role === 'creator') {
      const root = await rootDirectoryFor(me.id)
      if (root) out.push(toCreatorSummary(me, root.id, me.id))
    }

    const grants = await db.accessGrant.findMany({
      where: { userId: me.id, revokedAt: null },
      select: { directory: { select: { ownerId: true } }, video: { select: { ownerId: true } } },
    })
    const creatorIds = new Set<string>()
    for (const g of grants) {
      const owner = g.directory?.ownerId ?? g.video?.ownerId
      if (owner && owner !== me.id) creatorIds.add(owner)
    }
    if (creatorIds.size === 0) return out

    const creators = await db.user.findMany({
      where: { id: { in: [...creatorIds] } },
      include: { directories: { where: { parentId: null }, select: { id: true } } },
      orderBy: { name: 'asc' },
    })
    for (const c of creators) {
      const root = c.directories[0]
      if (root) out.push(toCreatorSummary(c, root.id, me.id))
    }
    return out
  })

  /** Directory tree for one creator, pruned to what I may see. Owners get the full tree. */
  app.get('/library/:creatorId', async (req): Promise<LibraryTree> => {
    const { creatorId } = creatorParam.parse(req.params)
    const creator = await db.user.findUnique({ where: { id: creatorId } })
    const col = creator ? await loadCollection(creatorId) : null
    if (!creator || !col) throw notFound('Creator')

    const own = creator.id === req.user.id
    const grants = own ? null : await activeGrantsFor(req.user.id, creatorId)
    if (grants && grants.directoryIds.size === 0 && grants.videoIds.size === 0) {
      throw forbidden('No access to this collection')
    }
    return {
      creator: toCreatorSummary(creator, col.root.id, req.user.id),
      canManage: own,
      root: buildTree(col, grants),
    }
  })
}
