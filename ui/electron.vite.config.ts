// SPDX-License-Identifier: AGPL-3.0-only
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
      alias: {
        '@renderer': resolve('src/renderer/src')
      }
    },
    plugins: [vue(), bundledPackages()]
  }
})
