import { env } from './env.js'
import { HttpError } from './errors.js'

const BASE = 'https://api.vimeo.com'

export interface VimeoVideo {
  uri: string
  name: string
  link: string
  duration: number
  status: string
  upload?: { approach: string; upload_link?: string; status?: string }
  transcode?: { status: string }
}

async function call<T>(method: string, path: string, body?: unknown): Promise<T> {
  if (!env.vimeoAccessToken) {
    throw new HttpError(503, 'vimeo_not_configured', 'VIMEO_ACCESS_TOKEN is not set')
  }
  const res = await fetch(`${BASE}${path}`, {
    method,
    headers: {
      Authorization: `bearer ${env.vimeoAccessToken}`,
      Accept: 'application/vnd.vimeo.*+json;version=3.4',
      'Content-Type': 'application/json',
    },
    body: body === undefined ? undefined : JSON.stringify(body),
  })
  const text = await res.text()
  if (!res.ok) {
    throw new HttpError(502, 'vimeo_error', `Vimeo ${method} ${path} failed (${res.status}): ${text.slice(0, 300)}`)
  }
  return (text ? JSON.parse(text) : undefined) as T
}

/** "/videos/123456" -> "123456" */
export function vimeoIdFromUri(uri: string): string {
  return uri.split('/').pop() ?? uri
}

/** "https://vimeo.com/123456/abcdef1234" -> "abcdef1234"; null when the link has no hash. */
export function hashFromLink(link: string): string | null {
  const parts = new URL(link).pathname.split('/').filter(Boolean)
  return parts.length >= 2 ? (parts[1] ?? null) : null
}

function privacyBody() {
  if (env.vimeoPrivacy === 'disable') {
    return {
      privacy: { view: 'disable', embed: 'whitelist', download: false, add: false, comments: 'nobody' },
      embed_domains: env.vimeoEmbedDomains,
    }
  }
  return {
    privacy: { view: 'unlisted', embed: 'public', download: false, add: false, comments: 'nobody' },
  }
}

/** Creates a tus upload. Returns the Vimeo id, unlisted hash, and the one-time upload link for the device. */
export async function createTusUpload(name: string, sizeBytes: number) {
  const v = await call<VimeoVideo>('POST', '/me/videos', {
    name,
    upload: { approach: 'tus', size: String(sizeBytes) },
    ...privacyBody(),
  })
  if (!v.upload?.upload_link) {
    throw new HttpError(502, 'vimeo_error', 'Vimeo did not return an upload link')
  }
  return {
    vimeoId: vimeoIdFromUri(v.uri),
    hash: hashFromLink(v.link),
    uploadLink: v.upload.upload_link,
  }
}

export async function getVideo(vimeoId: string) {
  return call<VimeoVideo>(
    'GET',
    `/videos/${vimeoId}?fields=uri,name,link,duration,status,upload.status,transcode.status`,
  )
}

export async function renameVideo(vimeoId: string, name: string) {
  await call('PATCH', `/videos/${vimeoId}`, { name })
}

export async function deleteVideo(vimeoId: string) {
  await call('DELETE', `/videos/${vimeoId}`)
}

export function embedUrl(vimeoId: string, hash: string | null): string {
  const url = new URL(`https://player.vimeo.com/video/${vimeoId}`)
  if (hash) url.searchParams.set('h', hash)
  url.searchParams.set('dnt', '1')
  return url.toString()
}
