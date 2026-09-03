import { existsSync, copyFileSync, chmodSync, mkdirSync, readFileSync, writeFileSync } from 'fs'
import { join } from 'path'
import { homedir } from 'os'

const HOOK_EVENTS = ['SessionStart', 'SessionEnd', 'PreToolUse', 'PostToolUse'] as const
type HookEvent = (typeof HOOK_EVENTS)[number]

const hookMatcher: Partial<Record<HookEvent, string>> = {
  PreToolUse: 'Write|Edit'
  // PostToolUse/SessionStart/SessionEnd omit matcher entirely.
}

// Maps Node's process.arch to the bundled resource subdirectory, which
// uses Go's GOARCH naming (set by lgrass/Makefile) rather than Node's.
function archResourceDir(): string | null {
  switch (process.arch) {
    case 'x64':
      return 'linux-amd64'
    case 'arm64':
      return 'linux-arm64'
    default:
      return null
  }
}

function resolveBundledLgrassPath(): string | null {
  const dir = archResourceDir()
  if (!dir) return null
  return join(process.resourcesPath, 'bin', dir, 'lgrass')
}

function resolveInstallDir(): string {
  const localBin = join(homedir(), '.local', 'bin')
  return existsSync(localBin) ? localBin : '/usr/local/bin'
}

// Copies the bundled lgrass binary matching this machine's arch into
// ~/.local/bin (or /usr/local/bin as a fallback), keeping it in
// permanent version lockstep with the running app. Never throws.
// Returns null on any failure (missing bundled binary, unwritable
// install dir) so a launch never fails over this.
export function installLgrass(): string | null {
  try {
    const bundled = resolveBundledLgrassPath()
    if (!bundled || !existsSync(bundled)) {
      console.error('lgrass: no bundled binary for this arch, skipping self-install')
      return null
    }
    const installDir = resolveInstallDir()
    const dest = join(installDir, 'lgrass')
    copyFileSync(bundled, dest)
    chmodSync(dest, 0o755)
    return dest
  } catch (err) {
    console.error('lgrass: self-install failed, continuing without it:', err)
    return null
  }
}

function settingsPath(): string {
  return join(homedir(), '.claude', 'settings.json')
}

interface HookHandler {
  type: 'command'
  command: string
}
interface HookMatcherGroup {
  matcher?: string
  hooks: HookHandler[]
}
type HooksSection = Partial<Record<HookEvent, HookMatcherGroup[]>>
interface ClaudeSettings {
  hooks?: HooksSection
  [key: string]: unknown
}

function replaceAt<T>(arr: T[], index: number, value: T): T[] {
  const copy = arr.slice()
  copy[index] = value
  return copy
}

function readSettings(): ClaudeSettings {
  try {
    return JSON.parse(readFileSync(settingsPath(), 'utf-8'))
  } catch {
    return {}
  }
}

// Registers `lgrass hook <event>` for each event in ~/.claude/settings.json,
// user-level, all projects. This is safe globally since lgrass's own hook
// command already no-ops for any cwd that isn't a registered lemongrass
// project. Idempotent: appends an entry for an event only if no existing
// entry's command already references it, and updates it in place if the
// install path has changed since the last launch (e.g. ~/.local/bin
// appearing after a prior fallback to /usr/local/bin), so the registered
// command never drifts stale. Repeated launches never pile up duplicates
// and never touch unrelated existing settings.
export function mergeLgrassHooks(lgrassPath: string): void {
  try {
    const settings = readSettings()
    const hooks: HooksSection = settings.hooks ?? {}

    for (const event of HOOK_EVENTS) {
      const command = `${lgrassPath} hook ${event}`
      const matcher = hookMatcher[event]
      const entry: HookMatcherGroup = matcher
        ? { matcher, hooks: [{ type: 'command', command }] }
        : { hooks: [{ type: 'command', command }] }

      const group = hooks[event] ?? []
      const existingIndex = group.findIndex((g) =>
        g.hooks.some((h) => h.command.includes(`hook ${event}`))
      )
      hooks[event] =
        existingIndex === -1 ? [...group, entry] : replaceAt(group, existingIndex, entry)
    }

    settings.hooks = hooks
    mkdirSync(join(homedir(), '.claude'), { recursive: true })
    writeFileSync(settingsPath(), JSON.stringify(settings, null, 2) + '\n')
  } catch (err) {
    console.error('lgrass: failed to register hooks in settings.json, continuing without it:', err)
  }
}
