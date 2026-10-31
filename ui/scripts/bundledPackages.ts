// SPDX-License-Identifier: AGPL-3.0-only
// Copyright (C) 2026 The Lemongrass Authors

import { readFileSync } from 'fs'
import { relative } from 'path'
import type { Plugin } from 'vite'

export const BUNDLED_PACKAGES_FILE = 'bundled-packages.json'

export interface BundledPackage {
  name: string
  version: string
  dir: string
}

// The packages vite inlined into one build target, whichever of dependencies or devDependencies
// declares them, for scripts/licenses.ts to credit. Vue is the case this exists for: a
// devDependency that still ships inside the renderer bundle.
export function bundledPackages(): Plugin {
  const root = process.cwd()
  return {
    name: 'lemongrass-bundled-packages',
    apply: 'build',
    generateBundle(_options, bundle) {
      const found = new Map<string, BundledPackage>()
      for (const chunk of Object.values(bundle)) {
        if (chunk.type !== 'chunk') continue
        for (const id of Object.keys(chunk.modules)) {
          const dir = packageDir(id)
          if (!dir || found.has(dir)) continue
          const pkg = JSON.parse(readFileSync(`${dir}/package.json`, 'utf8'))
          found.set(dir, { name: pkg.name, version: pkg.version, dir: relative(root, dir) })
        }
      }
      this.emitFile({
        type: 'asset',
        fileName: BUNDLED_PACKAGES_FILE,
        source: JSON.stringify([...found.values()], null, 2)
      })
    }
  }
}

// The innermost node_modules entry a module path belongs to, so a nested copy of a package is
// credited at its own version.
function packageDir(id: string): string | null {
  const path = id.replace(/^\0/, '').split('?')[0]
  const match = /^(.*\/node_modules\/(?:@[^/]+\/)?[^/]+)\//.exec(path)
  return match ? match[1] : null
}
