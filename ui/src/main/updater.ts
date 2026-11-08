// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

import { app, BrowserWindow, ipcMain, net } from 'electron'
import { createHash } from 'crypto'
import {
  createReadStream,
  createWriteStream,
  existsSync,
  mkdirSync,
  readdirSync,
  readFileSync,
  renameSync,
  rmSync,
  writeFileSync
} from 'fs'
import { homedir } from 'os'
import { dirname, join } from 'path'
import { Readable, Transform } from 'stream'
import { pipeline } from 'stream/promises'
import type { ReadableStream as NodeReadableStream } from 'stream/web'
import type { UpdateState } from '../preload/types'
import { isNewer, newestRelease, sha256For, versionFromTag } from './updateVersion'
import { nextCheckDelay, parseCheckCache, type CheckCache, type Release } from './updateCheckCache'
import type { UpdatePlatform } from './updatePlatform'
import { rpmPlatform } from './updaterRpm'
import { macPlatform } from './updaterMac'
import { winPlatform } from './updaterWin'

// Releases are built by .github/workflows/release.yml. The repo is public, so the check needs no token,
// and the unauthenticated limit of 60 requests an hour is far above one check every few hours.
const LATEST_URL = 'https://api.github.com/repos/faizalv/lemongrass/releases/latest'
const LIST_URL = 'https://api.github.com/repos/faizalv/lemongrass/releases?per_page=30'
// releases/latest skips prereleases. A tester sets this to update from one release candidate to the
// next before a stable tag exists.
const PRERELEASE = process.env.LEMONGRASS_UPDATE_PRERELEASE === '1'
const SUMS_ASSET = 'SHA256SUMS'
const CHECK_INTERVAL_MS = 60 * 60 * 1000
const UPDATES_DIR = join(homedir(), '.lemongrass', 'updates')
// Outside UPDATES_DIR, since a download removes every other entry there.
const CHECK_CACHE = join(homedir(), '.lemongrass', 'update-check.json')

interface ReleaseAsset {
  name: string
  browser_download_url: string
  size: number
}

interface ReleaseBody {
  tag_name?: string
  draft?: boolean
  assets?: ReleaseAsset[]
}

const platforms: Partial<Record<NodeJS.Platform, UpdatePlatform>> = {
  linux: rpmPlatform,
  darwin: macPlatform,
  win32: winPlatform
}
const platform: UpdatePlatform | null = platforms[process.platform] ?? null

let state: UpdateState = {
  phase: 'idle',
  current: app.getVersion(),
  latest: null,
  available: false,
  checkedAt: null,
  downloaded: false,
  progress: null,
  message: null
}
let release: Release | null = null
let nextCheck: NodeJS.Timeout | null = null
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
    'authorizing',
    'installing',
    'installed'
  ].includes(state.phase)
}

function messageOf(err: unknown): string {
  return err instanceof Error ? err.message : String(err)
}

async function unsupportedReason(): Promise<string | null> {
  if (!app.isPackaged) return 'This is a source build. Updates run only in the installed app.'
  if (!platform) return 'Updates are published for Linux, macOS and Windows only.'
  return platform.unsupportedReason()
}

async function fetchOk(url: string, headers?: Record<string, string>): Promise<Response> {
  const response = await net.fetch(url, { headers })
  if (!response.ok) throw new Error(`updater: ${url} answered ${response.status}`)
  return response
}

async function fetchRelease(): Promise<ReleaseBody> {
  const headers = { Accept: 'application/vnd.github+json' }
  if (!PRERELEASE) return (await (await fetchOk(LATEST_URL, headers)).json()) as ReleaseBody
  const list = (await (await fetchOk(LIST_URL, headers)).json()) as ReleaseBody[]
  const newest = newestRelease(list)
  if (!newest) throw new Error('updater: no published release found')
  return newest
}

function isDownloaded(rel: Release | null): boolean {
  return !!rel && existsSync(join(UPDATES_DIR, rel.version, rel.assetName))
}

function apply(result: CheckCache): void {
  if (!platform) return
  release = result.release
  const newer = isNewer(result.version, state.current)
  set({
    phase: 'idle',
    latest: result.version,
    available: newer && release !== null,
    checkedAt: result.checkedAt,
    downloaded: newer && isDownloaded(release),
    message:
      newer && !release
        ? `Release ${result.version} has no ${platform.assetName(result.version)} or SHA256SUMS attached yet.`
        : null
  })
}

function loadCache(): CheckCache | null {
  try {
    return parseCheckCache(readFileSync(CHECK_CACHE, 'utf8'), PRERELEASE, Date.now())
  } catch {
    return null
  }
}

// Written to a temporary file and renamed, so a crash mid-write leaves the previous cache, never half a file.
function saveCache(result: CheckCache): void {
  try {
    mkdirSync(dirname(CHECK_CACHE), { recursive: true })
    const temp = `${CHECK_CACHE}.tmp`
    writeFileSync(temp, JSON.stringify(result))
    renameSync(temp, CHECK_CACHE)
  } catch {
    // A missing cache only means the next launch checks at once.
  }
}

function scheduleCheck(delay: number): void {
  if (nextCheck) clearTimeout(nextCheck)
  nextCheck = setTimeout(() => void check(), delay)
}

// Every check sets the next one an hour later, whether it was scheduled, asked for with Check now or skipped
// while an update is busy. A failed check leaves the cached result in place.
async function check(): Promise<void> {
  if (!platform || state.phase === 'unsupported') return
  scheduleCheck(CHECK_INTERVAL_MS)
  if (busy()) return
  set({ phase: 'checking', message: null })
  try {
    const body = await fetchRelease()
    const version = versionFromTag(body.tag_name ?? '')
    const assetName = platform.assetName(version)
    const asset = body.assets?.find((a) => a.name === assetName)
    const sums = body.assets?.find((a) => a.name === SUMS_ASSET)
    const result: CheckCache = {
      checkedAt: Date.now(),
      prerelease: PRERELEASE,
      version,
      release:
        asset && sums
          ? {
              version,
              assetName,
              assetUrl: asset.browser_download_url,
              assetSize: asset.size,
              sumsUrl: sums.browser_download_url
            }
          : null
    }
    saveCache(result)
    apply(result)
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

// Downloads into ~/.lemongrass/updates/<version>/ and returns the verified package. A verified package
// left by a failed install is reused, and every other version's folder is removed, since each is large.
async function download(rel: Release): Promise<string> {
  if (existsSync(UPDATES_DIR)) {
    for (const entry of readdirSync(UPDATES_DIR)) {
      if (entry !== rel.version) rmSync(join(UPDATES_DIR, entry), { recursive: true, force: true })
    }
  }
  const dir = join(UPDATES_DIR, rel.version)
  mkdirSync(dir, { recursive: true })

  const want = sha256For(await (await fetchOk(rel.sumsUrl)).text(), rel.assetName)
  if (!want) throw new Error(`updater: SHA256SUMS has no line for ${rel.assetName}`)

  const file = join(dir, rel.assetName)
  if (existsSync(file)) {
    set({ phase: 'verifying', progress: null })
    if ((await hashFile(file)) === want) return file
    rmSync(file, { force: true })
  }

  set({ phase: 'downloading', progress: 0 })
  const response = await fetchOk(rel.assetUrl)
  if (!response.body) throw new Error(`updater: the ${rel.assetName} download returned no body`)
  const total = Number(response.headers.get('content-length')) || rel.assetSize
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
    throw new Error(`updater: the downloaded ${rel.assetName} does not match SHA256SUMS`)
  }
  renameSync(part, file)
  return file
}

async function install(): Promise<void> {
  if (!release || !state.available || busy() || !platform) return
  const rel = release
  set({ message: null })
  try {
    const file = await download(rel)
    const target = platform
    set({ phase: 'authorizing', progress: null })
    const message = await target.install(file, rel.version, () =>
      set({ phase: 'installing', message: target.installingMessage })
    )
    if (!platform.keepAfterInstall) {
      rmSync(join(UPDATES_DIR, rel.version), { recursive: true, force: true })
    }
    set({ phase: 'installed', available: false, message })
  } catch (err) {
    set({ phase: 'failed', progress: null, downloaded: isDownloaded(rel), message: messageOf(err) })
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
    if (state.phase !== 'installed' || !platform) return
    platform.restart()
  })

  // A macOS or Windows update stays staged in its version's folder until it is applied after quit, so
  // the folder named for the version now running holds the update that was just applied.
  rmSync(join(UPDATES_DIR, state.current), { recursive: true, force: true })

  void unsupportedReason().then((reason) => {
    if (reason) {
      set({ phase: 'unsupported', message: reason })
      return
    }
    const cached = loadCache()
    if (cached) apply(cached)
    scheduleCheck(nextCheckDelay(cached?.checkedAt ?? null, Date.now(), CHECK_INTERVAL_MS))
  })
}
