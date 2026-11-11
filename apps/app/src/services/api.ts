import type {
  AccessGrantInfo,
  AdminInfo,
  ApiError,
  AuditEventInfo,
  CreatorSummary,
  DirectoryRecord,
  LibraryTree,
  Me,
  PlaybackInfo,
  QrCodeInfo,
  RedeemResult,
  UploadLinkInfo,
  UploadLinkTarget,
  UploadTicket,
  UserRole,
  VideoDownload,
  VideoSummary,
} from '@/types'
import { authService } from './auth'

// Relative by default: the Go binary serves the app and the API from one origin, and the Vite dev
// server proxies /api. Native builds must set VITE_API_URL to the absolute API URL.
const BASE = (import.meta.env.VITE_API_URL ?? '/api').replace(/\/$/, '')

export class ApiRequestError extends Error {
  readonly status: number
  readonly code: string

  constructor(status: number, code: string, message: string) {
    super(message)
    this.status = status
    this.code = code
  }
}

export function errorMessage(err: unknown): string {
  if (err instanceof Error) return err.message
  return String(err)
}

async function request<T>(method: string, path: string, body?: unknown): Promise<T> {
  const headers: Record<string, string> = { ...(await authService.getHeaders()) }
  const init: RequestInit = { method, headers }
  if (body !== undefined) {
    headers['Content-Type'] = 'application/json'
    init.body = JSON.stringify(body)
  }
  const res = await fetch(`${BASE}${path}`, init)
  if (res.status === 204) return undefined as T
  const data: unknown = await res.json().catch(() => null)
  if (!res.ok) {
    const e = (data ?? {}) as Partial<ApiError>
    throw new ApiRequestError(res.status, e.error ?? 'error', e.message ?? res.statusText)
  }
  return data as T
}

function qs(params: Record<string, string | number | undefined>): string {
  const sp = new URLSearchParams()
  for (const [k, v] of Object.entries(params)) if (v !== undefined) sp.set(k, String(v))
  const s = sp.toString()
  return s ? `?${s}` : ''
}

/** A new video. videoDate is YYYY-MM-DD. */
export interface NewVideo {
  title: string
  directoryId: string
  videoDate?: string
}

export type ShareTarget = { directoryId: string; videoId?: undefined } | { videoId: string; directoryId?: undefined }

export const api = {
  me: {
    get: () => request<Me>('GET', '/me'),
    patch: (patch: { name?: string; role?: UserRole }) => request<Me>('PATCH', '/me', patch),
    /** Deletes the account and its sign-in. A creator must delete every video in their collection first. */
    remove: () => request<void>('DELETE', '/me'),
  },
  library: {
    list: () => request<CreatorSummary[]>('GET', '/library'),
    tree: (creatorId: string) => request<LibraryTree>('GET', `/library/${creatorId}`),
  },
  directories: {
    create: (name: string, parentId: string) => request<DirectoryRecord>('POST', '/directories', { name, parentId }),
    rename: (id: string, name: string) => request<DirectoryRecord>('PATCH', `/directories/${id}`, { name }),
    move: (id: string, parentId: string) => request<DirectoryRecord>('PATCH', `/directories/${id}`, { parentId }),
    remove: (id: string, mode: 'delete' | 'move', targetDirectoryId?: string) =>
      request<void>('DELETE', `/directories/${id}${qs({ mode, targetDirectoryId })}`),
  },
  videos: {
    ticket: (video: NewVideo, sizeBytes: number) =>
      request<UploadTicket>('POST', '/videos/upload-ticket', { ...video, sizeBytes }),
    /** A video with no file yet. Upload to it later with ticketFor. */
    placeholder: (video: NewVideo) => request<VideoSummary>('POST', '/videos/placeholder', video),
    /** Upload ticket for an existing video: a placeholder, an upload that did not finish, or a failed video. */
    ticketFor: (id: string, sizeBytes: number) => request<UploadTicket>('POST', `/videos/${id}/upload-ticket`, { sizeBytes }),
    complete: (id: string) => request<VideoSummary>('POST', `/videos/${id}/complete`),
    get: (id: string) => request<VideoSummary>('GET', `/videos/${id}`),
    playback: (id: string) => request<PlaybackInfo>('GET', `/videos/${id}/playback`),
    /** The sizes the owner or an admin can download, largest first. The links expire. */
    downloads: (id: string) => request<VideoDownload[]>('GET', `/videos/${id}/downloads`),
    rename: (id: string, title: string) => request<VideoSummary>('PATCH', `/videos/${id}`, { title }),
    move: (id: string, directoryId: string) => request<VideoSummary>('PATCH', `/videos/${id}`, { directoryId }),
    /** Sets the date (YYYY-MM-DD), or clears it with null. */
    setDate: (id: string, videoDate: string | null) =>
      request<VideoSummary>('PATCH', `/videos/${id}`, { videoDate: videoDate ?? '' }),
    remove: (id: string) => request<void>('DELETE', `/videos/${id}`),
  },
  share: {
    create: (target: ShareTarget, expiresAt?: string) => request<QrCodeInfo>('POST', '/share', { ...target, expiresAt }),
    list: (target: ShareTarget) => request<QrCodeInfo[]>('GET', `/share${qs(target)}`),
    revoke: (id: string) => request<QrCodeInfo>('DELETE', `/share/${id}`),
  },
  access: {
    list: (target: ShareTarget) => request<AccessGrantInfo[]>('GET', `/access${qs(target)}`),
    revoke: (id: string) => request<void>('DELETE', `/access/${id}`),
  },
  /** Collection admins. Only the owner lists, adds and removes them. */
  admins: {
    list: (directoryId: string) => request<AdminInfo[]>('GET', `/admins${qs({ directoryId })}`),
    add: (directoryId: string, email: string) => request<AdminInfo>('POST', '/admins', { directoryId, email }),
    remove: (id: string) => request<void>('DELETE', `/admins/${id}`),
  },
  uploadLinks: {
    create: (videoId: string) => request<UploadLinkInfo>('POST', '/upload-links', { videoId }),
    list: (videoId: string) => request<UploadLinkInfo[]>('GET', `/upload-links${qs({ videoId })}`),
    revoke: (id: string) => request<UploadLinkInfo>('DELETE', `/upload-links/${id}`),
    /** For the person who opened the link: any signed-in user. */
    target: (code: string) => request<UploadLinkTarget>('GET', `/upload-links/code/${encodeURIComponent(code)}`),
    ticket: (code: string, sizeBytes: number) =>
      request<UploadTicket>('POST', `/upload-links/code/${encodeURIComponent(code)}/ticket`, { sizeBytes }),
    complete: (code: string) => request<UploadLinkTarget>('POST', `/upload-links/code/${encodeURIComponent(code)}/complete`),
  },
  redeem: (code: string) => request<RedeemResult>('POST', '/redeem', { code }),
  audit: {
    list: (limit = 50, before?: string) => request<AuditEventInfo[]>('GET', `/audit${qs({ limit, before })}`),
  },
}
