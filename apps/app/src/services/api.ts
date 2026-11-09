import type {
  AccessGrantInfo,
  ApiError,
  AuditEventInfo,
  CreatorSummary,
  LibraryTree,
  Me,
  PlaybackInfo,
  QrCodeInfo,
  RedeemResult,
  UploadTicket,
  UserRole,
  VideoSummary,
} from '@wcs/shared'
import { authService } from './auth'

const BASE = (import.meta.env.VITE_API_URL ?? 'http://localhost:3000').replace(/\/$/, '')

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

async function request<T>(method: string, path: string, body?: unknown, form?: FormData): Promise<T> {
  const headers: Record<string, string> = { ...(await authService.getHeaders()) }
  const init: RequestInit = { method, headers }
  if (form) {
    init.body = form
  } else if (body !== undefined) {
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

export interface DirectoryRecord {
  id: string
  ownerId: string
  parentId: string | null
  name: string
  createdAt: string
}

export type ShareTarget = { directoryId: string; videoId?: undefined } | { videoId: string; directoryId?: undefined }

export const api = {
  me: {
    get: () => request<Me>('GET', '/me'),
    patch: (patch: { name?: string; role?: UserRole }) => request<Me>('PATCH', '/me', patch),
    setBanner: (file: File) => {
      const form = new FormData()
      form.append('file', file)
      return request<Me>('PUT', '/me/banner', undefined, form)
    },
    clearBanner: () => request<Me>('DELETE', '/me/banner'),
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
    ticket: (title: string, directoryId: string, sizeBytes: number) =>
      request<UploadTicket>('POST', '/videos/upload-ticket', { title, directoryId, sizeBytes }),
    complete: (id: string) => request<VideoSummary>('POST', `/videos/${id}/complete`),
    get: (id: string) => request<VideoSummary>('GET', `/videos/${id}`),
    playback: (id: string) => request<PlaybackInfo>('GET', `/videos/${id}/playback`),
    rename: (id: string, title: string) => request<VideoSummary>('PATCH', `/videos/${id}`, { title }),
    move: (id: string, directoryId: string) => request<VideoSummary>('PATCH', `/videos/${id}`, { directoryId }),
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
  redeem: (code: string) => request<RedeemResult>('POST', '/redeem', { code }),
  audit: {
    list: (limit = 50, before?: string) => request<AuditEventInfo[]>('GET', `/audit${qs({ limit, before })}`),
  },
  devices: {
    register: (token: string, platform: 'ios' | 'android' | 'web') =>
      request<void>('POST', '/devices', { token, platform }),
    remove: (token: string) => request<void>('DELETE', `/devices/${encodeURIComponent(token)}`),
  },
}
