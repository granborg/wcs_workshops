import type { Directory, Video } from '@prisma/client'
import type { CreatorSummary, DirectoryNode, VideoStatus, VideoSummary } from '@wcs/shared'
import { db } from './db.js'
import { env } from './env.js'

export interface Collection {
  dirs: Directory[]
  dirsById: Map<string, Directory>
  videos: Video[]
  root: Directory
}

export async function loadCollection(creatorId: string): Promise<Collection | null> {
  const [dirs, videos] = await Promise.all([
    db.directory.findMany({ where: { ownerId: creatorId }, orderBy: { name: 'asc' } }),
    db.video.findMany({ where: { ownerId: creatorId }, orderBy: { title: 'asc' } }),
  ])
  const root = dirs.find((d) => d.parentId === null)
  if (!root) return null
  return { dirs, dirsById: new Map(dirs.map((d) => [d.id, d])), videos, root }
}

/** Ids of the directory and every ancestor up to the root. */
export function ancestorIds(dirId: string, dirsById: Map<string, Directory>): string[] {
  const out: string[] = []
  let cur = dirsById.get(dirId)
  while (cur) {
    out.push(cur.id)
    cur = cur.parentId ? dirsById.get(cur.parentId) : undefined
  }
  return out
}

/** Ids of the directory and every descendant. */
export function subtreeIds(dirId: string, dirs: Directory[]): string[] {
  const byParent = new Map<string, Directory[]>()
  for (const d of dirs) {
    if (!d.parentId) continue
    const list = byParent.get(d.parentId) ?? []
    list.push(d)
    byParent.set(d.parentId, list)
  }
  const out = [dirId]
  for (let i = 0; i < out.length; i++) {
    const id = out[i] as string
    for (const child of byParent.get(id) ?? []) out.push(child.id)
  }
  return out
}

export interface ViewerGrants {
  directoryIds: Set<string>
  videoIds: Set<string>
}

export async function activeGrantsFor(viewerId: string, creatorId: string): Promise<ViewerGrants> {
  const grants = await db.accessGrant.findMany({
    where: {
      userId: viewerId,
      revokedAt: null,
      OR: [{ directory: { ownerId: creatorId } }, { video: { ownerId: creatorId } }],
    },
    select: { directoryId: true, videoId: true },
  })
  return {
    directoryIds: new Set(grants.map((g) => g.directoryId).filter((x): x is string => x !== null)),
    videoIds: new Set(grants.map((g) => g.videoId).filter((x): x is string => x !== null)),
  }
}

export function videoVisible(video: Video, grants: ViewerGrants, col: Collection): boolean {
  if (grants.videoIds.has(video.id)) return true
  return ancestorIds(video.directoryId, col.dirsById).some((id) => grants.directoryIds.has(id))
}

/** True when the viewer may play this video: owner, or an active grant on it or an ancestor directory. */
export async function canView(viewerId: string, video: Video): Promise<boolean> {
  if (video.ownerId === viewerId) return true
  const col = await loadCollection(video.ownerId)
  if (!col) return false
  const grants = await activeGrantsFor(viewerId, video.ownerId)
  return videoVisible(video, grants, col)
}

export function toVideoSummary(v: Video): VideoSummary {
  return {
    id: v.id,
    title: v.title,
    directoryId: v.directoryId,
    status: v.status as VideoStatus,
    durationSec: v.durationSec,
    createdAt: v.createdAt.toISOString(),
  }
}

/**
 * Builds the directory tree. With `grants` null the full tree is returned (owner view).
 * Otherwise the tree is pruned: a directory is kept when it is granted, lies under a granted
 * directory, or contains something visible further down.
 */
export function buildTree(col: Collection, grants: ViewerGrants | null): DirectoryNode {
  const childrenOf = new Map<string, Directory[]>()
  for (const d of col.dirs) {
    if (!d.parentId) continue
    const list = childrenOf.get(d.parentId) ?? []
    list.push(d)
    childrenOf.set(d.parentId, list)
  }
  const videosOf = new Map<string, Video[]>()
  for (const v of col.videos) {
    if (grants && !videoVisible(v, grants, col)) continue
    const list = videosOf.get(v.directoryId) ?? []
    list.push(v)
    videosOf.set(v.directoryId, list)
  }

  function build(dir: Directory, inherited: boolean): DirectoryNode | null {
    const granted = inherited || grants === null || grants.directoryIds.has(dir.id)
    const children = (childrenOf.get(dir.id) ?? [])
      .map((c) => build(c, granted))
      .filter((n): n is DirectoryNode => n !== null)
    const videos = (videosOf.get(dir.id) ?? []).map(toVideoSummary)
    if (!granted && children.length === 0 && videos.length === 0) return null
    return { id: dir.id, name: dir.name, parentId: dir.parentId, children, videos }
  }

  return (
    build(col.root, false) ?? { id: col.root.id, name: col.root.name, parentId: null, children: [], videos: [] }
  )
}

export function bannerUrl(path: string | null): string | null {
  if (!path) return null
  return path.startsWith('http') ? path : `${env.publicApiUrl}${path}`
}

export function toCreatorSummary(
  user: { id: string; name: string; bannerUrl: string | null },
  rootDirectoryId: string,
  viewerId: string,
): CreatorSummary {
  return {
    id: user.id,
    name: user.name,
    bannerUrl: bannerUrl(user.bannerUrl),
    rootDirectoryId,
    own: user.id === viewerId,
  }
}

export function rootDirectoryFor(creatorId: string) {
  return db.directory.findFirst({ where: { ownerId: creatorId, parentId: null } })
}
