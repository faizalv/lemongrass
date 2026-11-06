// SPDX-License-Identifier: Apache-2.0
// Copyright (C) 2026 The Sankasten Authors

import { fileURLToPath } from 'node:url'
import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'

// Builds the documentation site in site/ into dist/. A relative base keeps the built site working
// from any path it is deployed under, or opened straight from disk.
export default defineConfig({
  root: fileURLToPath(new URL('./site', import.meta.url)),
  base: './',
  plugins: [vue()],
  build: {
    outDir: fileURLToPath(new URL('./dist', import.meta.url)),
    emptyOutDir: true
  }
})
