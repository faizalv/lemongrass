// SPDX-License-Identifier: AGPL-3.0-only
// Copyright (C) 2026 The Lemongrass Authors

import { Marked } from 'marked'

const SCHEME = /^[a-z][a-z0-9+.-]*:/i

const CHECK_MARK =
  '<svg width="10" height="10" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="3.5" stroke-linecap="round" stroke-linejoin="round"><polyline points="20 6 9 17 4 12" /></svg>'

function escapeHtml(text: string): string {
  return text
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;')
    .replace(/'/g, '&#39;')
}

// Resolves a relative link or image path against the doc it appears in, both relative to biblio/.
// Null when it climbs above biblio/, which no reader request is allowed to reach.
export function resolveDocRelative(docPath: string, target: string): string | null {
  let decoded: string
  try {
    decoded = decodeURIComponent(target)
  } catch {
    decoded = target
  }
  const segments = docPath.split('/').slice(0, -1)
  for (const part of decoded.split('/')) {
    if (part === '' || part === '.') continue
    if (part === '..') {
      if (!segments.length) return null
      segments.pop()
    } else segments.push(part)
  }
  return segments.length ? segments.join('/') : null
}

// GitHub's heading anchor form, so a `#some-heading` link written for GitHub lands here too.
export function headingAnchor(text: string): string {
  return text
    .trim()
    .toLowerCase()
    .replace(/[^\p{L}\p{N}\s_-]/gu, '')
    .replace(/\s/g, '-')
}

export type ReaderLink =
  | { kind: 'external'; url: string }
  | { kind: 'anchor'; anchor: string }
  | { kind: 'doc'; path: string; anchor: string }
  | { kind: 'none' }

// Decides what a click on a reader link does. Nothing here may navigate the app window itself.
export function classifyReaderLink(docPath: string, href: string): ReaderLink {
  if (/^(https?|mailto):/i.test(href)) return { kind: 'external', url: href }
  if (SCHEME.test(href) || href.startsWith('/')) return { kind: 'none' }
  const hashAt = href.indexOf('#')
  const pathPart = (hashAt === -1 ? href : href.slice(0, hashAt)).split('?')[0]
  const anchor = hashAt === -1 ? '' : decodeAnchor(href.slice(hashAt + 1))
  if (!pathPart) return anchor ? { kind: 'anchor', anchor } : { kind: 'none' }
  const path = resolveDocRelative(docPath, pathPart)
  if (!path) return { kind: 'none' }
  return { kind: 'doc', path, anchor }
}

function decodeAnchor(anchor: string): string {
  try {
    return decodeURIComponent(anchor)
  } catch {
    return anchor
  }
}

// Builds the address of an image at a root-relative path. Each root has its own scheme, and each
// scheme's handler refuses anything outside that root.
export type ImageUrl = (path: string) => string

export function biblioImageUrl(projectPath: string): ImageUrl {
  return (path) => `biblio-image://project/?${new URLSearchParams({ p: projectPath, f: path })}`
}

export function projectImageUrl(projectPath: string): ImageUrl {
  return (path) => `project-file://project/?${new URLSearchParams({ p: projectPath, f: path })}`
}

// Renders a read-only markdown file whose `docPath` is relative to a root (biblio/ or the project).
// Relative images resolve against the file's own folder and load through `imageUrl`, task
// checkboxes are drawn rather than native inputs, and headings carry a data-anchor so in-doc links
// scroll without putting ids into the shared window document.
export function renderReaderMarkdown(source: string, docPath: string, imageUrl: ImageUrl): string {
  const anchorsSeen = new Map<string, number>()
  const marked = new Marked({
    gfm: true,
    renderer: {
      checkbox: ({ checked }) =>
        `<span class="md-check${checked ? ' checked' : ''}" role="img" aria-label="${checked ? 'done' : 'not done'}">${CHECK_MARK}</span> `,
      heading({ tokens, depth, text }) {
        const base = headingAnchor(text)
        const count = anchorsSeen.get(base) ?? 0
        anchorsSeen.set(base, count + 1)
        const anchor = count ? `${base}-${count}` : base
        return `<h${depth} data-anchor="${escapeHtml(anchor)}">${this.parser.parseInline(tokens)}</h${depth}>\n`
      },
      image({ href, title, text, tokens }) {
        const alt = tokens ? this.parser.parseInline(tokens, this.parser.textRenderer) : text
        let src = href
        if (!SCHEME.test(href) && !href.startsWith('/')) {
          const path = resolveDocRelative(docPath, href.split(/[?#]/)[0])
          if (!path) return `<span class="md-image-missing">${escapeHtml(alt || href)}</span>`
          src = imageUrl(path)
        }
        const titleAttribute = title ? ` title="${escapeHtml(title)}"` : ''
        return `<img src="${escapeHtml(src)}" alt="${escapeHtml(alt)}"${titleAttribute}>`
      }
    }
  })
  return marked.parse(source, { async: false })
}
