// SPDX-License-Identifier: AGPL-3.0-only
// Copyright (C) 2026 The Lemongrass Authors

import { app, BrowserWindow, ipcMain, net } from 'electron'
import { execFile } from 'child_process'
import { createHash } from 'crypto'
import {
  createReadStream,
  createWriteStream,
  existsSync,
  mkdirSync,
  readdirSync,
  renameSync,
  rmSync
} from 'fs'
import { homedir } from 'os'
import { join } from 'path'
import { Readable, Transform } from 'stream'
import { pipeline } from 'stream/promises'
import type { ReadableStream as NodeReadableStream } from 'stream/web'
import type { UpdateState } from '../preload/types'
import { isNewer, rpmAssetName, sha256For, versionFromTag } from './updateVersion'

// Releases are built by .github/workflows/release.yml. The repo is public, so the check needs no token,
// and the unauthenticated limit of 60 requests an hour is far above one check every few hours.
const LATEST_URL = 'https://api.github.com/repos/faizalv/lemongrass/releases/latest'
const SUMS_ASSET = 'SHA256SUMS'
const FIRST_CHECK_DELAY_MS = 30_000
const CHECK_INTERVAL_MS = 6 * 60 * 60 * 1000
const UPDATES_DIR = join(homedir(), '.lemongrass', 'updates')
const RPM = '/usr/bin/rpm'

interface Release {
  version: string
  rpmName: string
  rpmUrl: string
  rpmSize: number
  sumsUrl: string
}

interface ReleaseAsset {
  name: string
  browser_download_url: string
  size: number
}

let state: UpdateState = {
  phase: 'idle',
  current: app.getVersion(),
  latest: null,
  available: false,
  checkedAt: null,
  progress: null,
  message: null
}
let release: Release | null = null
let currentWindow: () => BrowserWindow | undefined = () => undefined

function set(patch: Partial<UpdateState>): void {
  state = { ...state, ...patch }
  const window = currentWindow()
  if (window && !window.isDestroyed()) window.webContents.send('update:state-changed', state)
}

function busy(): boolean {
  return [
    'unsupported',
    'checking',
    'downloading',
    'verifying',
    'installing',
    'installed'
  ].includes(state.phase)
}

function messageOf(err: unknown): string {
  return err instanceof Error ? err.message : String(err)
}

function run(command: string, args: string[]): Promise<void> {
  return new Promise((resolve, reject) => {
    execFile(command, args, { encoding: 'utf8' }, (failure, _stdout, stderr) => {
      if (!failure) return resolve()
      reject(Object.assign(new Error(stderr.trim() || failure.message), { code: failure.code }))
    })
  })
}

// Updating replaces the rpm's files under /opt, so only a copy that rpm itself installed can do it.
async function unsupportedReason(): Promise<string | null> {
  if (!app.isPackaged) return 'This is a source build. Updates run only in the installed app.'
  if (process.platform !== 'linux' || process.arch !== 'x64') {
    return 'Updates are published for Linux x86_64 only.'
  }
  try {
    await run(RPM, ['-q', 'lemongrass'])
  } catch {
    return 'This copy was not installed from the rpm, so it cannot update itself.'
  }
  return null
}

async function fetchOk(url: string, headers?: Record<string, string>): Promise<Response> {
  const response = await net.fetch(url, { headers })
  if (!response.ok) throw new Error(`updater: ${url} answered ${response.status}`)
  return response
}

async function check(): Promise<void> {
  if (busy()) return
  set({ phase: 'checking', message: null })
  try {
    const response = await fetchOk(LATEST_URL, { Accept: 'application/vnd.github+json' })
    const body = (await response.json()) as { tag_name?: string; assets?: ReleaseAsset[] }
    const version = versionFromTag(body.tag_name ?? '')
    const rpmName = rpmAssetName(version)
    const rpm = body.assets?.find((asset) => asset.name === rpmName)
    const sums = body.assets?.find((asset) => asset.name === SUMS_ASSET)
    release =
      rpm && sums
        ? {
            version,
            rpmName,
            rpmUrl: rpm.browser_download_url,
            rpmSize: rpm.size,
            sumsUrl: sums.browser_download_url
          }
        : null
    const newer = isNewer(version, state.current)
    set({
      phase: 'idle',
      latest: version,
      available: newer && release !== null,
      checkedAt: Date.now(),
      message:
        newer && !release ? `Release ${version} has no rpm or SHA256SUMS attached yet.` : null
    })
  } catch (err) {
    set({ phase: 'failed', message: messageOf(err) })
  }
}

function hashFile(path: string): Promise<string> {
  return new Promise((resolve, reject) => {
    const hash = createHash('sha256')
    createReadStream(path)
      .on('data', (chunk) => hash.update(chunk))
      .on('error', reject)
      .on('end', () => resolve(hash.digest('hex')))
  })
}

// Downloads into ~/.lemongrass/updates/<version>/ and returns the verified rpm. A verified rpm left by
// a failed install is reused, and every other version's folder is removed, since each rpm is large.
async function download(rel: Release): Promise<string> {
  if (existsSync(UPDATES_DIR)) {
    for (const entry of readdirSync(UPDATES_DIR)) {
      if (entry !== rel.version) rmSync(join(UPDATES_DIR, entry), { recursive: true, force: true })
    }
  }
  const dir = join(UPDATES_DIR, rel.version)
  mkdirSync(dir, { recursive: true })

  const want = sha256For(await (await fetchOk(rel.sumsUrl)).text(), rel.rpmName)
  if (!want) throw new Error(`updater: SHA256SUMS has no line for ${rel.rpmName}`)

  const file = join(dir, rel.rpmName)
  if (existsSync(file)) {
    set({ phase: 'verifying', progress: null })
    if ((await hashFile(file)) === want) return file
    rmSync(file, { force: true })
  }

  set({ phase: 'downloading', progress: 0 })
  const response = await fetchOk(rel.rpmUrl)
  if (!response.body) throw new Error('updater: the rpm download returned no body')
  const total = Number(response.headers.get('content-length')) || rel.rpmSize
  const hash = createHash('sha256')
  let received = 0
  let reported = 0
  const meter = new Transform({
    transform(chunk: Buffer, _encoding, done): void {
      hash.update(chunk)
      received += chunk.length
      const progress = total > 0 ? Math.min(received / total, 1) : 0
      if (progress - reported >= 0.01) {
        reported = progress
        set({ progress })
      }
      done(null, chunk)
    }
  })
  const part = `${file}.part`
  await pipeline(
    Readable.fromWeb(response.body as unknown as NodeReadableStream),
    meter,
    createWriteStream(part)
  )

  set({ phase: 'verifying', progress: null })
  if (hash.digest('hex') !== want) {
    rmSync(part, { force: true })
    throw new Error('updater: the downloaded rpm does not match SHA256SUMS')
  }
  renameSync(part, file)
  return file
}

// pkexec shows the desktop's polkit password prompt. It exits 126 when the prompt is dismissed.
async function installRpm(file: string): Promise<void> {
  try {
    await run('pkexec', [RPM, '-U', file])
  } catch (err) {
    if ((err as { code?: unknown }).code === 126) {
      throw new Error('updater: the password prompt was dismissed, nothing was installed')
    }
    throw err
  }
}

async function install(): Promise<void> {
  if (!release || !state.available || busy()) return
  const rel = release
  set({ message: null })
  try {
    const file = await download(rel)
    set({ phase: 'installing', progress: null })
    await installRpm(file)
    rmSync(join(UPDATES_DIR, rel.version), { recursive: true, force: true })
    set({ phase: 'installed', available: false })
  } catch (err) {
    set({ phase: 'failed', progress: null, message: messageOf(err) })
  }
}

export function registerUpdateHandlers(getWindow: () => BrowserWindow | undefined): void {
  currentWindow = getWindow
  ipcMain.handle('update:state', (): UpdateState => state)
  ipcMain.handle('update:check', async (): Promise<UpdateState> => {
    await check()
    return state
  })
  ipcMain.handle('update:install', (): void => {
    void install()
  })
  ipcMain.handle('update:restart', (): void => {
    if (state.phase !== 'installed') return
    app.relaunch()
    app.quit()
  })

  void unsupportedReason().then((reason) => {
    if (reason) {
      set({ phase: 'unsupported', message: reason })
      return
    }
    setTimeout(() => void check(), FIRST_CHECK_DELAY_MS)
    setInterval(() => void check(), CHECK_INTERVAL_MS)
  })
}
