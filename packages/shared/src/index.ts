// Types shared between the Capacitor app and the API.
// No enums: the app compiles with erasableSyntaxOnly.

export const USER_ROLES = ['viewer', 'creator'] as const
export type UserRole = (typeof USER_ROLES)[number]

export const VIDEO_STATUSES = ['uploading', 'processing', 'ready', 'failed'] as const
export type VideoStatus = (typeof VIDEO_STATUSES)[number]

export const AUDIT_TYPES = [
  'qr.created',
  'qr.revoked',
  'access.granted',
  'access.revoked',
  'video.deleted',
  'video.moved',
  'directory.moved',
  'directory.deleted',
] as const
export type AuditType = (typeof AUDIT_TYPES)[number]

/** URL scheme encoded in QR images. The app also accepts the bare code. */
export const QR_SCHEME = 'wcsw://redeem/'

export interface Me {
  id: string
  name: string
  email: string | null
  role: UserRole
  bannerUrl: string | null
  /** Root directory id when the user is a creator. */
  rootDirectoryId: string | null
}

export interface CreatorSummary {
  id: string
  name: string
  bannerUrl: string | null
  rootDirectoryId: string
  /** True when this is the signed-in user's own collection. */
  own: boolean
}

export interface VideoSummary {
  id: string
  title: string
  directoryId: string
  status: VideoStatus
  durationSec: number | null
  createdAt: string
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
  /** Whether the viewer may manage items (owner). */
  canManage: boolean
  root: DirectoryNode
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
  actor: { id: string; name: string } | null
  subject: { id: string; name: string } | null
  ip: string | null
  directoryId: string | null
  videoId: string | null
  qrCodeId: string | null
  detail: Record<string, unknown> | null
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
