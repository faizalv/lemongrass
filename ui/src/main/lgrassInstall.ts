import { app } from 'electron'
import { existsSync, copyFileSync, chmodSync } from 'fs'
import { join } from 'path'
import { homedir } from 'os'
import { spawn } from 'child_process'

// Maps Node's process.platform + process.arch to the bundled resource
// subdirectory (Go GOOS-GOARCH naming from lgrass/lgrassconf Makefiles).
function resourceDir(): string | null {
  let osName: string
  switch (process.platform) {
    case 'linux':
      osName = 'linux'
      break
    case 'darwin':
      osName = 'darwin'
      break
    default:
      return null
  }
  let arch: string
  switch (process.arch) {
    case 'x64':
      arch = 'amd64'
      break
    case 'arm64':
      arch = 'arm64'
      break
    default:
      return null
  }
  return `${osName}-${arch}`
}

function resolveBundledBin(name: 'lgrass' | 'lgrassconf'): string | null {
  const dir = resourceDir()
  if (!dir) return null
  return join(process.resourcesPath, 'bin', dir, name)
}

function resolveInstallDir(): string {
  const localBin = join(homedir(), '.local', 'bin')
  return existsSync(localBin) ? localBin : '/usr/local/bin'
}

function copyBundled(name: 'lgrass' | 'lgrassconf'): string | null {
  try {
    const bundled = resolveBundledBin(name)
    if (!bundled || !existsSync(bundled)) {
      console.error(`${name}: no bundled binary for this platform/arch, skipping self-install`)
      return null
    }
    const installDir = resolveInstallDir()
    const dest = join(installDir, name)
    copyFileSync(bundled, dest)
    chmodSync(dest, 0o755)
    return dest
  } catch (err) {
    console.error(`${name}: self-install failed, continuing without it:`, err)
    return null
  }
}

// Copies the bundled lgrass binary matching this machine into
// ~/.local/bin (or /usr/local/bin as a fallback), keeping it in
// permanent version lockstep with the running app. Never throws.
//
// Unpackaged (electron-vite dev) has no resourcesPath bundle to copy
// from. `make dev` builds lgrass straight to the install dir instead.
export function installLgrass(): string | null {
  if (!app.isPackaged) {
    const dest = join(resolveInstallDir(), 'lgrass')
    if (!existsSync(dest)) {
      console.error(
        'lgrass: not found at',
        dest,
        '. Run `make dev` from the repo root instead of `npm run dev` directly.'
      )
      return null
    }
    return dest
  }
  return copyBundled('lgrass')
}

// Same pattern for lgrassconf. On packaged launches, also runs
// `lgrassconf install` so the agent config keeper is registered
// (LaunchAgent on macOS, systemd user unit on Linux). Failures are
// logged, never fatal.
export function installLgrassconf(): string | null {
  if (!app.isPackaged) {
    const dest = join(resolveInstallDir(), 'lgrassconf')
    return existsSync(dest) ? dest : null
  }
  const dest = copyBundled('lgrassconf')
  if (!dest) return null
  try {
    const child = spawn(dest, ['install'], {
      detached: true,
      stdio: 'ignore'
    })
    child.unref()
  } catch (err) {
    console.error('lgrassconf: install spawn failed:', err)
  }
  return dest
}
