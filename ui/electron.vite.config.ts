// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

import { resolve } from 'path'
import { defineConfig } from 'electron-vite'
import vue from '@vitejs/plugin-vue'
import { bundledPackages } from './scripts/bundledPackages'

export default defineConfig({
  main: { plugins: [bundledPackages()] },
  preload: { plugins: [bundledPackages()] },
  renderer: {
    resolve: {
      // The design system is imported from source in ../sankasten. Deduping vue makes the package's
      // components use the app's own Vue instance instead of one resolved from sankasten/node_modules.
      alias: [
        { find: '@renderer', replacement: resolve('src/renderer/src') },
        { find: /^@sankasten$/, replacement: resolve('../sankasten/src/index.ts') },
        { find: /^@sankasten\//, replacement: resolve('../sankasten/src') + '/' }
      ],
      dedupe: ['vue']
    },
    plugins: [vue(), bundledPackages()]
  }
})
