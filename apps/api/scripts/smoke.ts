/**
 * End-to-end smoke test against a running API with DEV_AUTH_BYPASS=true.
 * Usage: npx tsx scripts/smoke.ts [http://localhost:3000]
 * Exercises: role switch, folders, QR share, redeem as a second user, pruned library, revoke, audit.
 * Vimeo calls are not exercised (no token needed).
 */

export {}

const base = (process.argv[2] ?? 'http://localhost:3000').replace(/\/$/, '')
const stamp = Date.now()
const creator = `smoke-creator-${stamp}@example.com`
const viewer = `smoke-viewer-${stamp}@example.com`

let failures = 0

async function call<T>(user: string, method: string, path: string, body?: unknown): Promise<T> {
  const headers: Record<string, string> = { 'x-dev-user': user }
  if (body !== undefined) headers['Content-Type'] = 'application/json'
  const res = await fetch(`${base}${path}`, {
    method,
    headers,
    body: body === undefined ? undefined : JSON.stringify(body),
  })
  const text = await res.text()
  if (!res.ok) throw new Error(`${method} ${path} -> ${res.status} ${text}`)
  return (text ? JSON.parse(text) : undefined) as T
}

function check(label: string, ok: boolean, detail?: unknown) {
  if (ok) {
    console.log(`  ok   ${label}`)
  } else {
    failures++
    console.log(`  FAIL ${label}`, detail === undefined ? '' : JSON.stringify(detail))
  }
}

async function expectError(label: string, status: number, fn: () => Promise<unknown>) {
  try {
    await fn()
    check(label, false, 'expected an error')
  } catch (err) {
    check(label, String(err).includes(` ${status} `), String(err))
  }
}

type Me = { id: string; role: string; rootDirectoryId: string | null }
type Dir = { id: string }
type Qr = { id: string; code: string; payload: string }
type Tree = { root: { id: string; children: { id: string; name: string; children: unknown[] }[] } }
type Grant = { id: string; user: { id: string } }
type Audit = { type: string }[]

console.log(`Smoke test against ${base}`)

const health = await fetch(`${base}/health`).then((r) => r.json())
check('health', (health as { ok: boolean }).ok === true)

let me = await call<Me>(creator, 'GET', '/me')
check('new user is a viewer', me.role === 'viewer')

me = await call<Me>(creator, 'PATCH', '/me', { role: 'creator', name: 'Smoke Creator' })
check('becomes creator with a root directory', me.role === 'creator' && !!me.rootDirectoryId)
const rootId = me.rootDirectoryId as string

const season = await call<Dir>(creator, 'POST', '/directories', { name: 'Season 1', parentId: rootId })
const week = await call<Dir>(creator, 'POST', '/directories', { name: 'Week 1', parentId: season.id })
const other = await call<Dir>(creator, 'POST', '/directories', { name: 'Private', parentId: rootId })
check('created three directories', !!season.id && !!week.id && !!other.id)

await expectError('cannot move a directory into its own subtree', 400, () =>
  call(creator, 'PATCH', `/directories/${season.id}`, { parentId: week.id }),
)
await expectError('cannot delete the root', 400, () => call(creator, 'DELETE', `/directories/${rootId}?mode=delete`))

const qr = await call<Qr>(creator, 'POST', '/share', { directoryId: season.id })
check('QR code created with payload', qr.payload.startsWith('wcsw://redeem/'))

await expectError('viewer has no access before redeeming', 403, () => call(viewer, 'GET', `/library/${me.id}`))
await expectError('creator cannot redeem own code', 400, () => call(creator, 'POST', '/redeem', { code: qr.payload }))

const redeemed = await call<{ added: boolean }>(viewer, 'POST', '/redeem', { code: qr.payload })
check('viewer redeems code', redeemed.added === true)
const again = await call<{ added: boolean }>(viewer, 'POST', '/redeem', { code: qr.code })
check('second redeem is idempotent', again.added === false)

const lib = await call<{ id: string }[]>(viewer, 'GET', '/library')
check('creator appears in viewer library', lib.some((c) => c.id === me.id))

const tree = await call<Tree>(viewer, 'GET', `/library/${me.id}`)
const names = tree.root.children.map((c) => c.name)
check('viewer sees Season 1 but not Private', names.includes('Season 1') && !names.includes('Private'), names)
const seasonNode = tree.root.children.find((c) => c.name === 'Season 1')
check('viewer sees Week 1 inside Season 1 (inherited)', (seasonNode?.children.length ?? 0) === 1)

const grants = await call<Grant[]>(creator, 'GET', `/access?directoryId=${season.id}`)
check('creator sees one grant on Season 1', grants.length === 1)

await call(creator, 'DELETE', `/access/${grants[0]!.id}`)
await expectError('viewer loses access after revoke', 403, () => call(viewer, 'GET', `/library/${me.id}`))

const revokedQr = await call<{ revokedAt: string | null }>(creator, 'DELETE', `/share/${qr.id}`)
check('QR code revoked', revokedQr.revokedAt !== null)
await expectError('revoked code cannot be redeemed', 404, () => call(viewer, 'POST', '/redeem', { code: qr.code }))

await call(creator, 'DELETE', `/directories/${season.id}?mode=delete`)
const afterDelete = await call<Tree>(creator, 'GET', `/library/${me.id}`)
check('Season 1 deleted', !afterDelete.root.children.some((c) => c.name === 'Season 1'))

const audit = await call<Audit>(creator, 'GET', '/audit')
const types = new Set(audit.map((a) => a.type))
check(
  'audit log has the expected event types',
  ['qr.created', 'access.granted', 'access.revoked', 'qr.revoked', 'directory.deleted'].every((t) => types.has(t)),
  [...types],
)

const back = await call<Me>(creator, 'PATCH', '/me', { role: 'viewer' })
check('creator with empty collection can become viewer', back.role === 'viewer' && back.rootDirectoryId === null)

console.log(failures === 0 ? '\nAll checks passed.' : `\n${failures} check(s) failed.`)
process.exit(failures === 0 ? 0 : 1)
