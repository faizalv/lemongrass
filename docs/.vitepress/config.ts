// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

import { defineConfig } from 'vitepress'

// Light and dark come from Sankasten's data-theme attribute, not from VitePress's .dark class, so the tokens switch
// with the toggle. This runs in the head before first paint and mirrors VitePress's own appearance script, which reads
// the same storage key and falls back to the configured default, so a light-mode visitor never sees a dark flash.
const themeSync = `(() => {
  const stored = localStorage.getItem('vitepress-theme-appearance') || 'dark'
  const dark = stored === 'dark' || (stored === 'auto' && matchMedia('(prefers-color-scheme: dark)').matches)
  if (!dark) document.documentElement.dataset.theme = 'light'
})()`

const features = [
  { text: 'Databases and APIs', link: '/features/connectors' },
  { text: 'Workgroups', link: '/features/workgroups' },
  { text: 'The guard', link: '/features/guard' },
  { text: 'Asking the human', link: '/features/ask' },
  { text: 'Notification center', link: '/features/notifications' },
  { text: 'Orientation', link: '/features/orientation' },
  { text: 'Claude accounts', link: '/features/accounts' },
  { text: 'Sharing with other agents', link: '/features/node' },
  { text: 'Terminals', link: '/features/terminals' }
]

const development = [
  { text: 'How it fits together', link: '/development/architecture' },
  { text: 'Build', link: '/development/build' },
  { text: 'Develop', link: '/development/develop' }
]

export default defineConfig({
  title: 'Lemongrass',
  description:
    'The agent centered IDE for Claude Code, Codex and Antigravity. Agents that know your project, quality work out of the box, and full capability with a safety check on every step.',
  lang: 'en-US',
  cleanUrls: true,
  appearance: 'dark',
  head: [
    ['link', { rel: 'icon', type: 'image/svg+xml', href: '/logo.svg' }],
    ['script', {}, themeSync]
  ],
  themeConfig: {
    logo: '/logo.svg',
    nav: [
      { text: 'Install', link: '/install' },
      { text: 'Features', link: '/features/connectors' },
      { text: 'Roadmap', link: '/roadmap' },
      { text: 'Development', link: '/development/architecture' }
    ],
    sidebar: [
      { text: 'Get started', items: [{ text: 'Install and uninstall', link: '/install' }] },
      { text: 'Features', items: features },
      { text: 'Roadmap', link: '/roadmap' },
      { text: 'Development', items: development }
    ],
    socialLinks: [{ icon: 'github', link: 'https://github.com/faizalv/lemongrass' }],
    search: { provider: 'local' },
    outline: [2, 3]
  }
})
