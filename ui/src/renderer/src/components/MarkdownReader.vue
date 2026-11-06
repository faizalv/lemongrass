<!-- SPDX-License-Identifier: AGPL-3.0-only -->
<!-- Copyright (C) 2026 The Lemongrass Authors -->

<script setup lang="ts">
import { ref } from 'vue'
import { classifyReaderLink, headingAnchor } from '../readerMarkdown'

// Rendered markdown shown read-only, shared by biblio docs and project files. `docPath` is relative
// to the root that `open` resolves paths against: biblio/ for a doc, the project for a file.
const props = defineProps<{
  html: string
  docPath: string
  open: (path: string) => void
}>()

const reader = ref<HTMLElement | null>(null)

function scrollToAnchor(anchor: string): void {
  const body = reader.value
  if (!body || !anchor) return
  const find = (name: string): Element | null =>
    body.querySelector(`[data-anchor="${CSS.escape(name)}"]`)
  const heading = find(anchor) ?? find(headingAnchor(anchor))
  heading?.scrollIntoView({ block: 'start' })
}

// An anchor rendered with v-html would otherwise navigate the whole app window away.
function onClick(event: MouseEvent): void {
  const anchor = (event.target as HTMLElement).closest('a')
  if (!anchor) return
  event.preventDefault()
  if (event.type === 'auxclick' && event.button !== 1) return
  const link = classifyReaderLink(props.docPath, anchor.getAttribute('href') ?? '')
  if (link.kind === 'external') window.open(link.url)
  else if (link.kind === 'anchor') scrollToAnchor(link.anchor)
  else if (link.kind === 'doc') {
    if (link.path === props.docPath) scrollToAnchor(link.anchor)
    else props.open(link.path)
  }
}
</script>

<template>
  <div ref="reader" class="reader" @click="onClick" @auxclick="onClick">
    <!-- eslint-disable-next-line vue/no-v-html -->
    <div class="markdown-body" v-html="html" />
  </div>
</template>

<style scoped>
.reader {
  flex: 1;
  min-height: 0;
  overflow-y: auto;
  padding: var(--space-4) var(--space-6);
}

.markdown-body {
  width: 100%;
  color: var(--color-fg-primary);
  font-family: var(--font-body);
  font-size: var(--text-sm);
  line-height: var(--leading-relaxed);
}

.markdown-body :deep(h1),
.markdown-body :deep(h2),
.markdown-body :deep(h3),
.markdown-body :deep(h4),
.markdown-body :deep(h5),
.markdown-body :deep(h6) {
  font-family: var(--font-display);
  font-weight: var(--weight-bold);
  line-height: var(--leading-tight);
  margin-top: var(--space-6);
  margin-bottom: var(--space-3);
}

.markdown-body :deep(h1:first-child),
.markdown-body :deep(h2:first-child),
.markdown-body :deep(h3:first-child),
.markdown-body :deep(h4:first-child),
.markdown-body :deep(h5:first-child),
.markdown-body :deep(h6:first-child) {
  margin-top: 0;
}

.markdown-body :deep(h1) {
  font-size: var(--text-xl);
}
.markdown-body :deep(h2) {
  font-size: var(--text-lg);
}
.markdown-body :deep(h3) {
  font-size: var(--text-md);
}
.markdown-body :deep(h4) {
  font-size: var(--text-base);
}
.markdown-body :deep(h5),
.markdown-body :deep(h6) {
  font-size: var(--text-sm);
}
.markdown-body :deep(h6) {
  color: var(--color-fg-secondary);
}

.markdown-body :deep(p) {
  margin-bottom: var(--space-4);
}

.markdown-body :deep(ul),
.markdown-body :deep(ol) {
  margin-bottom: var(--space-4);
  padding-left: var(--space-6);
}

.markdown-body :deep(li) {
  margin-bottom: var(--space-1);
}

.markdown-body :deep(li > ul),
.markdown-body :deep(li > ol) {
  margin-top: var(--space-1);
  margin-bottom: 0;
}

.markdown-body :deep(li:has(> .md-check)),
.markdown-body :deep(li:has(> p:first-child > .md-check)) {
  list-style: none;
}

.markdown-body :deep(.md-check) {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 14px;
  height: 14px;
  margin-left: calc(-1 * var(--space-5));
  margin-right: var(--space-1);
  vertical-align: -2px;
  background: var(--color-surface-1);
  border: 1px solid var(--color-border-strong);
  border-radius: var(--radius-sm);
  color: var(--color-black);
}

.markdown-body :deep(.md-check svg) {
  opacity: 0;
}

.markdown-body :deep(.md-check.checked) {
  background: var(--color-amber);
  border-color: var(--color-amber);
}

.markdown-body :deep(.md-check.checked svg) {
  opacity: 1;
}

.markdown-body :deep(table) {
  display: block;
  max-width: 100%;
  width: max-content;
  overflow-x: auto;
  border-collapse: collapse;
  margin-bottom: var(--space-4);
}

.markdown-body :deep(th),
.markdown-body :deep(td) {
  padding: var(--space-1) var(--space-3);
  border: 1px solid var(--color-border-subtle);
  text-align: left;
  vertical-align: top;
}

.markdown-body :deep(th) {
  background: var(--layer-raised);
  font-weight: var(--weight-semibold);
}

.markdown-body :deep(th[align='center']),
.markdown-body :deep(td[align='center']) {
  text-align: center;
}

.markdown-body :deep(th[align='right']),
.markdown-body :deep(td[align='right']) {
  text-align: right;
}

.markdown-body :deep(tbody tr:nth-child(even)) {
  background: var(--layer-card);
}

.markdown-body :deep(img) {
  max-width: 100%;
  border-radius: var(--radius-md);
}

.markdown-body :deep(.md-image-missing) {
  color: var(--color-fg-muted);
  font-style: italic;
}

.markdown-body :deep(del) {
  color: var(--color-fg-secondary);
}

.markdown-body :deep(a) {
  color: var(--color-fg-accent);
}

.markdown-body :deep(strong) {
  font-weight: var(--weight-semibold);
}

.markdown-body :deep(code) {
  font-family: var(--font-mono);
  font-size: 0.9em;
  background: var(--color-surface-2);
  padding: 0.15em 0.4em;
  border-radius: var(--radius-sm);
}

.markdown-body :deep(pre) {
  background: var(--color-surface-2);
  border-radius: var(--radius-lg);
  padding: var(--space-4);
  overflow-x: auto;
  margin-bottom: var(--space-4);
}

.markdown-body :deep(pre code) {
  background: none;
  padding: 0;
}

.markdown-body :deep(blockquote) {
  border-left: 2px solid var(--color-border-default);
  padding-left: var(--space-4);
  color: var(--color-fg-secondary);
  margin-bottom: var(--space-4);
}

.markdown-body :deep(hr) {
  border: none;
  border-top: 1px solid var(--color-border-subtle);
  margin: var(--space-6) 0;
}
</style>
