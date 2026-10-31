// SPDX-License-Identifier: AGPL-3.0-only
// Copyright (C) 2026 The Lemongrass Authors

// Writes <out>/licenses.json, the third-party credits shown in Settings > Credits. It ships inside
// app.asar with the rest of out/, which is what carries the notices that MIT, BSD, Apache and OFL
// ask every distributed copy to include. Run after `electron-vite build`, since it reads the
// bundled-packages.json files that build leaves behind.
//
//   node scripts/licenses.ts [--out <dir>] [--warn]
//
// A license off the accepted list fails the run unless --warn, which make dev uses so a new
// dependency never blocks a dev launch.

import { execFileSync } from 'child_process'
import { existsSync, readdirSync, readFileSync, rmSync, writeFileSync } from 'fs'
import { dirname, join, resolve } from 'path'
import { fileURLToPath } from 'url'
import { detectLicense, licenseAccepted } from './licenseRules.ts'

const BUNDLED_PACKAGES_FILE = 'bundled-packages.json'

type Group = 'fonts' | 'app' | 'tools' | 'electron'

interface Entry {
  group: Group
  name: string
  version: string
  license: string
  homepage?: string
  text: string
}

interface Pkg {
  name: string
  version: string
  dir: string
}

// Packages whose declared license is missing or not an SPDX expression, keyed name@version with
// the license read from their license file. A new version drops out of this map on purpose, so
// its license is looked at again.
const EXCEPTIONS: Record<string, { license: string; reason: string }> = {}

const ui = resolve(dirname(fileURLToPath(import.meta.url)), '..')
const repo = resolve(ui, '..')
const args = process.argv.slice(2)
const out = resolve(ui, args.includes('--out') ? args[args.indexOf('--out') + 1] : 'out')
const warnOnly = args.includes('--warn')

const problems: string[] = []
const entries = [...npmEntries(), ...goEntries(), electronEntry()]
entries.sort((a, b) => a.group.localeCompare(b.group) || a.name.localeCompare(b.name))

for (const e of entries) {
  if (!licenseAccepted(e.license)) {
    problems.push(`${e.name}@${e.version}: license "${e.license}" is not on the accepted list`)
  }
}

writeFileSync(join(out, 'licenses.json'), JSON.stringify(entries))
console.log(`licenses: ${entries.length} components written to ${join(out, 'licenses.json')}`)

if (problems.length > 0) {
  for (const p of problems) console.error(`licenses: ${p}`)
  if (!warnOnly) {
    console.error(
      'licenses: add the license to ACCEPTED in scripts/licenseRules.ts only if it can ship with AGPL-3.0-only, or record the package in EXCEPTIONS in scripts/licenses.ts.'
    )
    process.exit(1)
  }
}

function npmEntries(): Entry[] {
  const pkgs = new Map<string, Pkg>()
  for (const p of [...packagedClosure(), ...bundled()]) pkgs.set(`${p.name}@${p.version}`, p)
  return [...pkgs.values()].map((p) => {
    const manifest = JSON.parse(readFileSync(join(p.dir, 'package.json'), 'utf8'))
    const files = licenseFiles(p.dir)
    const declared = declaredLicense(manifest)
    const exception = EXCEPTIONS[`${p.name}@${p.version}`]
    let license = exception?.license ?? declared
    if (!license) {
      problems.push(`${p.name}@${p.version}: package.json declares no license`)
      license = 'UNKNOWN'
    }
    const text =
      files.length > 0
        ? joinTexts(p.dir, files)
        : `This package ships no license file. Its package.json declares ${license}.`
    return {
      group: p.name.startsWith('@fontsource') ? 'fonts' : 'app',
      name: p.name,
      version: p.version,
      license,
      homepage: homepage(manifest),
      text
    }
  })
}

// What electron-builder copies into app.asar: the closure of ui/package.json dependencies through
// each package's dependencies and optionalDependencies, without peers. scripts/afterPack.mjs
// checks the real asar against the result.
function packagedClosure(): Pkg[] {
  const found = new Map<string, Pkg>()
  const visit = (name: string, from: string, optional: boolean): void => {
    const dir = resolvePackage(name, from)
    if (!dir) {
      if (!optional) problems.push(`${name}: dependency of ${from} is not installed`)
      return
    }
    if (found.has(dir)) return
    const manifest = JSON.parse(readFileSync(join(dir, 'package.json'), 'utf8'))
    found.set(dir, { name: manifest.name, version: manifest.version, dir })
    for (const dep of Object.keys(manifest.dependencies ?? {})) visit(dep, dir, false)
    for (const dep of Object.keys(manifest.optionalDependencies ?? {})) visit(dep, dir, true)
  }
  const root = JSON.parse(readFileSync(join(ui, 'package.json'), 'utf8'))
  for (const dep of Object.keys(root.dependencies ?? {})) visit(dep, ui, false)
  return [...found.values()]
}

// Node's lookup: the nearest node_modules/<name> walking up from the dependent package, stopping
// at the ui root.
function resolvePackage(name: string, from: string): string | null {
  let dir = from
  for (;;) {
    const candidate = join(dir, 'node_modules', name)
    if (existsSync(join(candidate, 'package.json'))) return candidate
    if (dir === ui) return null
    const parent = dirname(dir)
    if (parent === dir) return null
    dir = parent
  }
}

function bundled(): Pkg[] {
  const result: Pkg[] = []
  for (const target of ['main', 'preload', 'renderer']) {
    const file = join(out, target, BUNDLED_PACKAGES_FILE)
    if (!existsSync(file)) {
      problems.push(`${file} is missing; run electron-vite build first`)
      continue
    }
    for (const p of JSON.parse(readFileSync(file, 'utf8')) as Pkg[]) {
      result.push({ ...p, dir: resolve(ui, p.dir) })
    }
    rmSync(file)
  }
  return result
}

function declaredLicense(manifest: Record<string, unknown>): string | null {
  const field = manifest.license ?? manifest.licenses
  if (typeof field === 'string') return field
  if (Array.isArray(field)) {
    const ids = field.map((l) => (typeof l === 'string' ? l : l?.type)).filter(Boolean)
    return ids.length > 0 ? `(${ids.join(' OR ')})` : null
  }
  if (field && typeof field === 'object' && 'type' in field) return String(field.type)
  return null
}

function homepage(manifest: Record<string, unknown>): string | undefined {
  if (typeof manifest.homepage === 'string') return manifest.homepage
  const repository = manifest.repository
  const url =
    typeof repository === 'string'
      ? repository
      : typeof repository === 'object' && repository && 'url' in repository
        ? String(repository.url)
        : undefined
  if (!url) return undefined
  return url
    .replace(/^git\+/, '')
    .replace(/^git:\/\//, 'https://')
    .replace(/^github:/, 'https://github.com/')
    .replace(/^([\w-]+\/[\w.-]+)$/, 'https://github.com/$1')
    .replace(/\.git$/, '')
}

function goEntries(): Entry[] {
  const modules = new Map<string, { path: string; version: string; dir: string }>()
  for (const goos of ['linux', 'darwin']) {
    const listing = execFileSync(
      'go',
      [
        'list',
        '-deps',
        '-f',
        '{{with .Module}}{{if not .Main}}{{.Path}}\t{{.Version}}\t{{if .Replace}}{{.Replace.Dir}}{{else}}{{.Dir}}{{end}}{{end}}{{end}}',
        './cmd/lgrass',
        './cmd/lgrassd',
        './cmd/lgrassconf'
      ],
      { cwd: repo, encoding: 'utf8', env: { ...process.env, GOOS: goos } }
    )
    for (const line of listing.split('\n')) {
      if (!line.trim()) continue
      const [path, version, dir] = line.split('\t')
      modules.set(`${path}@${version}`, { path, version, dir })
    }
  }
  return [...modules.values()].map((m) => {
    if (!m.dir)
      problems.push(`${m.path}@${m.version}: not in the module cache; run go mod download`)
    const files = m.dir ? licenseFiles(m.dir) : []
    const detected =
      files.length > 0 ? detectLicense(readFileSync(join(m.dir, files[0]), 'utf8')) : null
    const exception = EXCEPTIONS[`${m.path}@${m.version}`]
    const license = exception?.license ?? detected ?? 'UNKNOWN'
    if (files.length === 0) problems.push(`${m.path}@${m.version}: no license file`)
    else if (!exception && !detected)
      problems.push(`${m.path}@${m.version}: license text not recognised`)
    return {
      group: 'tools' as const,
      name: m.path,
      version: m.version.replace(/^v/, ''),
      license,
      homepage: `https://pkg.go.dev/${m.path}`,
      text: files.length > 0 ? joinTexts(m.dir, files) : ''
    }
  })
}

// Electron's own MIT notice. Chromium's list is far too large to copy, and Electron already ships
// it beside the executable as LICENSES.chromium.html, which the Credits section opens in place.
function electronEntry(): Entry {
  const dir = join(ui, 'node_modules', 'electron')
  const manifest = JSON.parse(readFileSync(join(dir, 'package.json'), 'utf8'))
  return {
    group: 'electron',
    name: 'Electron',
    version: manifest.version,
    license: declaredLicense(manifest) ?? 'MIT',
    homepage: 'https://www.electronjs.org',
    text: joinTexts(dir, licenseFiles(dir))
  }
}

// License files at a package root, the plain LICENSE/LICENCE/COPYING first so it is the one a
// license is detected from, then extras such as LICENSE-APACHE or LICENSE-3RD-PARTY.
function licenseFiles(dir: string): string[] {
  const matching = readdirSync(dir).filter((f) => /^(licen[cs]e|copying)([._-].*)?$/i.test(f))
  const plain = (f: string): number => (/^(licen[cs]e|copying)(\.(md|txt))?$/i.test(f) ? 0 : 1)
  return matching.sort((a, b) => plain(a) - plain(b) || a.localeCompare(b))
}

function joinTexts(dir: string, files: string[]): string {
  if (files.length === 1) return readFileSync(join(dir, files[0]), 'utf8').trim()
  return files.map((f) => `${f}\n\n${readFileSync(join(dir, f), 'utf8').trim()}`).join('\n\n\n')
}
