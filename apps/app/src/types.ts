// JSON shapes returned by the Go API. Mirrors apps/api/internal/model/model.go.
// No enums: the app compiles with erasableSyntaxOnly.

export const USER_ROLES = ['viewer', 'creator'] as const
export type UserRole = (typeof USER_ROLES)[number]

/** placeholder: no file yet. The creator uploads it later. */
export const VIDEO_STATUSES = ['placeholder', 'uploading', 'processing', 'ready', 'failed'] as const
export type VideoStatus = (typeof VIDEO_STATUSES)[number]

export const AUDIT_TYPES = [
  'qr.created',
  'qr.revoked',
  'access.granted',
  'access.revoked',
  'upload_link.created',
  'upload_link.used',
  'upload_link.revoked',
] as const
export type AuditType = (typeof AUDIT_TYPES)[number]

/** URL scheme encoded in QR images. The app also accepts the bare code. */
export const QR_SCHEME = 'wcsw://redeem/'

export interface Me {
  id: string
  name: string
  email: string | null
  role: UserRole
  /** Root directory id when the user is a creator. */
  rootDirectoryId: string | null
}

export interface CreatorSummary {
  id: string
  name: string
  rootDirectoryId: string
  /** True when this is the signed-in user's own collection. */
  own: boolean
  /** True when the signed-in user is an admin of at least one folder of this collection. */
  admin: boolean
}

export interface VideoSummary {
  id: string
  ownerId: string
  title: string
  directoryId: string
  status: VideoStatus
  durationSec: number | null
  /** Frame size in pixels and a thumbnail link, null until Vimeo has processed the video. */
  width: number | null
  height: number | null
  thumbnailUrl: string | null
  createdAt: string
  /** Set only by GET /videos/{id}: the signed-in user is the owner or an admin of the video. */
  canManage: boolean
}

export interface DirectoryNode {
  id: string
  name: string
  parentId: string | null
  children: DirectoryNode[]
  videos: VideoSummary[]
}

export interface LibraryTree {
  creator: CreatorSummary
  /** True for the owner, who also manages folders and admins. */
  canManage: boolean
  /**
   * Folders where the signed-in user is an admin. The user manages the videos, share codes and access of
   * these folders and of every folder under them.
   */
  adminDirectoryIds: string[]
  root: DirectoryNode
}

export interface DirectoryRecord {
  id: string
  ownerId: string
  parentId: string | null
  name: string
  createdAt: string
}

export interface QrCodeInfo {
  id: string
  code: string
  /** Full payload to encode in the QR image. */
  payload: string
  directoryId: string | null
  videoId: string | null
  expiresAt: string | null
  revokedAt: string | null
  createdAt: string
  redemptions: number
}

export interface AccessGrantInfo {
  id: string
  user: { id: string; name: string }
  directoryId: string | null
  videoId: string | null
  qrCodeId: string | null
  createdAt: string
}

export interface AuditEventInfo {
  id: string
  at: string
  type: AuditType
  /** The viewer affected, for access events. */
  subject: { id: string; name: string } | null
  ip: string | null
  directoryId: string | null
  videoId: string | null
  /** Folder name or video title, as the app shows it. A root folder shows as the creator's name. */
  targetName: string
  qrCodeId: string | null
}

export interface AdminInfo {
  id: string
  user: { id: string; name: string }
  email: string | null
  directoryId: string
  createdAt: string
}

/** An upload link as its owner or an admin sees it. */
export interface UploadLinkInfo {
  id: string
  code: string
  /** Full link to send or to encode in a QR image. */
  url: string
  videoId: string
  /** The last user who started an upload with the link. */
  usedBy: { id: string; name: string } | null
  usedAt: string | null
  revokedAt: string | null
  createdAt: string
}

/** What the person who opens an upload link sees. */
export interface UploadLinkTarget {
  videoId: string
  videoTitle: string
  creatorName: string
  /** False when the video already has a file: the link no longer accepts uploads. */
  open: boolean
}

export interface UploadTicket {
  videoId: string
  /** tus upload endpoint on Vimeo. */
  uploadLink: string
}

export interface PlaybackInfo {
  videoId: string
  vimeoId: string
  /** Unlisted hash, null when privacy mode is domain-based. */
  hash: string | null
  embedUrl: string
}

export interface RedeemResult {
  creator: CreatorSummary
  directoryId: string | null
  videoId: string | null
  /** False when the user already had this access. */
  added: boolean
}

export interface ApiError {
  error: string
  message: string
}
