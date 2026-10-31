// SPDX-License-Identifier: AGPL-3.0-only
// Copyright (C) 2026 The Lemongrass Authors

// electron-builder afterPack hook. scripts/licenses.ts predicts which npm packages land in
// app.asar by walking the dependency tree itself; this fails the package when the real asar holds
// a package that out/licenses.json does not credit, so the two can never drift apart unnoticed.
// @electron/asar comes in with electron-builder, which is why it is not a direct devDependency.

import { join } from 'path'
import { extractFile, listPackage } from '@electron/asar'

// Plain JavaScript so electron-builder can import it without a TypeScript loader.
// eslint-disable-next-line @typescript-eslint/explicit-function-return-type
export default async function afterPack(context) {
  const resources =
    context.electronPlatformName === 'darwin'
      ? join(
          context.appOutDir,
          `${context.packager.appInfo.productFilename}.app`,
          'Contents',
          'Resources'
        )
      : join(context.appOutDir, 'resources')
  const archive = join(resources, 'app.asar')

  const files = listPackage(archive, { isPack: false })
  if (!files.includes('/out/licenses.json')) {
    throw new Error(
      'afterPack: app.asar has no out/licenses.json; run scripts/licenses.ts after electron-vite build'
    )
  }
  const credited = new Set(
    JSON.parse(extractFile(archive, 'out/licenses.json').toString('utf8')).map(
      (e) => `${e.name}@${e.version}`
    )
  )

  const missing = []
  for (const file of files) {
    if (!/\/node_modules\/(@[^/]+\/)?[^/@][^/]*\/package\.json$/.test(file)) continue
    const manifest = JSON.parse(extractFile(archive, file.slice(1)).toString('utf8'))
    // A package.json without a name marks a subfolder of a package, such as a dist/ with its own
    // "type", not a package of its own.
    if (!manifest.name || !manifest.version) continue
    const key = `${manifest.name}@${manifest.version}`
    if (!credited.has(key)) missing.push(key)
  }
  if (missing.length > 0) {
    throw new Error(
      `afterPack: app.asar ships packages that out/licenses.json does not credit: ${missing.join(', ')}`
    )
  }
  console.log(`afterPack: every package in app.asar is credited (${credited.size} entries)`)
}
