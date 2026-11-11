import type { DirectoryNode } from '@/types'

export function findNode(root: DirectoryNode, id: string): DirectoryNode | null {
  if (root.id === id) return root
  for (const c of root.children) {
    const hit = findNode(c, id)
    if (hit) return hit
  }
  return null
}

/** Nodes from the root down to and including `id`. Empty when `id` is not in the tree. */
export function pathTo(root: DirectoryNode, id: string): DirectoryNode[] {
  if (root.id === id) return [root]
  for (const c of root.children) {
    const sub = pathTo(c, id)
    if (sub.length > 0) return [root, ...sub]
  }
  return []
}

export interface FlatDirectory {
  id: string
  name: string
  depth: number
}

/** Pre-order flattening with depth, for folder pickers. */
export function flatten(root: DirectoryNode, depth = 0): FlatDirectory[] {
  const out: FlatDirectory[] = [{ id: root.id, name: root.name, depth }]
  for (const c of root.children) out.push(...flatten(c, depth + 1))
  return out
}

export interface FolderPath extends FlatDirectory {
  /** Full path from the root, for example "Ana / Season 1 / Week 1". The root shows as `rootName`. */
  path: string
}

/** Pre-order flattening with the full path of each folder, for folder pickers. */
export function folderPaths(root: DirectoryNode, rootName: string): FolderPath[] {
  const out: FolderPath[] = []
  const walk = (node: DirectoryNode, depth: number, parentPath: string | null) => {
    const path = parentPath === null ? rootName : `${parentPath} / ${node.name}`
    out.push({ id: node.id, name: node.name, depth, path })
    for (const c of node.children) walk(c, depth + 1, path)
  }
  walk(root, 0, null)
  return out
}

export function subtreeIds(node: DirectoryNode): string[] {
  const out = [node.id]
  for (const c of node.children) out.push(...subtreeIds(c))
  return out
}

export function countVideos(node: DirectoryNode): number {
  return node.videos.length + node.children.reduce((n, c) => n + countVideos(c), 0)
}
