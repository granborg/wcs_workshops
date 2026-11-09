import 'dotenv/config'

function str(name: string, fallback?: string): string {
  const v = process.env[name]
  if (v !== undefined && v !== '') return v
  if (fallback !== undefined) return fallback
  throw new Error(`Missing required environment variable ${name}`)
}

function bool(name: string, fallback: boolean): boolean {
  const v = process.env[name]
  if (v === undefined || v === '') return fallback
  return v === 'true' || v === '1'
}

function list(name: string, fallback = ''): string[] {
  return (process.env[name] ?? fallback)
    .split(',')
    .map((s) => s.trim())
    .filter(Boolean)
}

export const env = {
  port: Number(str('PORT', '3000')),
  host: str('HOST', '0.0.0.0'),
  publicApiUrl: str('PUBLIC_API_URL', 'http://localhost:3000'),
  corsOrigins: list('CORS_ORIGINS', 'http://localhost:5173,capacitor://localhost,http://localhost'),
  devAuthBypass: bool('DEV_AUTH_BYPASS', false),
  vimeoAccessToken: process.env.VIMEO_ACCESS_TOKEN ?? '',
  vimeoPrivacy: (process.env.VIMEO_PRIVACY === 'disable' ? 'disable' : 'unlisted') as 'unlisted' | 'disable',
  vimeoEmbedDomains: list('VIMEO_EMBED_DOMAINS'),
  uploadDir: str('UPLOAD_DIR', './uploads'),
  isProduction: process.env.NODE_ENV === 'production',
}

if (env.isProduction && env.devAuthBypass) {
  throw new Error('DEV_AUTH_BYPASS must not be enabled in production')
}
