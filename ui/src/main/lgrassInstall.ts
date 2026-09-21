import { app } from 'electron'
import { existsSync, copyFileSync, chmodSync } from 'fs'
import { join } from 'path'
import { homedir } from 'os'

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
//
// Unpackaged (electron-vite dev) has no resourcesPath bundle to copy
// from. `make dev` builds lgrass straight to the install dir instead,
// before launching Electron. This just resolves the path `make dev`
// already installed to, rather than copying anything itself.
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
