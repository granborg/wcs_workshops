// Builds the web app, embeds it in the Go API, and compiles one binary to apps/api/bin/.
import { cpSync, mkdirSync, rmSync, writeFileSync } from 'node:fs'
import { execSync } from 'node:child_process'

const run = (cmd, cwd = '.') => execSync(cmd, { stdio: 'inherit', cwd })

run('npm run build -w apps/app')

const dist = 'apps/api/web/dist'
rmSync(dist, { recursive: true, force: true })
mkdirSync(dist, { recursive: true })
cpSync('apps/app/dist', dist, { recursive: true })
writeFileSync(`${dist}/.gitkeep`, '')

run('go build -o bin/ ./cmd/api', 'apps/api')
console.log('\nBuilt apps/api/bin/api (plus .exe on Windows). It serves the web app at / and the API at /api.')
