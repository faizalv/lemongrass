<!-- SPDX-License-Identifier: Apache-2.0 -->
<!-- Copyright (C) 2026 The Sankasten Authors -->

<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref } from 'vue'
import { AppNavItem } from '../src'
import ActionsSection from './sections/ActionsSection.vue'
import DisplaySection from './sections/DisplaySection.vue'
import FieldsSection from './sections/FieldsSection.vue'
import OverviewSection from './sections/OverviewSection.vue'
import RulesSection from './sections/RulesSection.vue'
import TokensSection from './sections/TokensSection.vue'
import { DOCS } from './docs'

const groups: { title: string; links: { id: string; label: string }[] }[] = [
  {
    title: 'Foundations',
    links: [
      { id: 'overview', label: 'Overview' },
      { id: 'tokens-color', label: 'Color' },
      { id: 'tokens-layers', label: 'Layers' },
      { id: 'tokens-type', label: 'Type' },
      { id: 'tokens-space', label: 'Spacing and radius' },
      { id: 'tokens-depth', label: 'Shadow and motion' },
      { id: 'tokens-height', label: 'Control height' }
    ]
  },
  {
    title: 'Rules',
    links: [
      { id: 'rules-weight', label: 'Weight' },
      { id: 'rules-sizes', label: 'Sizes' },
      { id: 'rules-labels', label: 'Icon labels' },
      { id: 'rules-copy', label: 'UI copy' },
      { id: 'rules-tokens', label: 'Tokens and layers' }
    ]
  },
  {
    title: 'Components',
    links: Object.keys(DOCS)
      .sort()
      .map((name) => ({ id: name, label: name }))
  }
]

const current = ref(location.hash.slice(1) || 'overview')

function go(id: string): void {
  current.value = id
  document.getElementById(id)?.scrollIntoView({ behavior: 'smooth', block: 'start' })
  history.replaceState(null, '', `#${id}`)
}

function onHash(): void {
  current.value = location.hash.slice(1)
}

onMounted(() => {
  window.addEventListener('hashchange', onHash)
  if (location.hash) document.getElementById(current.value)?.scrollIntoView()
})
onBeforeUnmount(() => window.removeEventListener('hashchange', onHash))
</script>

<template>
  <div class="site">
    <nav class="nav" aria-label="Sections">
      <p class="brand">Sankasten</p>
      <div v-for="group in groups" :key="group.title" class="nav-group">
        <p class="nav-title">{{ group.title }}</p>
        <AppNavItem
          v-for="link in group.links"
          :key="link.id"
          class="nav-link"
          size="sm"
          :label="link.label"
          :active="current === link.id"
          :aria-current="current === link.id ? 'location' : undefined"
          @click="go(link.id)"
        />
      </div>
    </nav>
    <main class="main">
      <div class="content">
        <OverviewSection />
        <TokensSection />
        <RulesSection />
        <ActionsSection />
        <FieldsSection />
        <DisplaySection />
      </div>
    </main>
  </div>
</template>

<style scoped>
.site {
  display: grid;
  grid-template-columns: 240px 1fr;
  height: 100%;
}

.nav {
  display: flex;
  flex-direction: column;
  gap: var(--space-4);
  padding: var(--space-5) var(--space-3);
  overflow-y: auto;
  background: var(--color-surface-1);
  border-right: 1px solid var(--color-border-subtle);
}

.brand {
  padding: 0 var(--space-2);
  color: var(--color-fg-accent);
  font-family: var(--font-display);
  font-size: var(--text-lg);
  font-weight: var(--weight-bold);
}

.nav-group {
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.nav-title {
  padding: 0 var(--space-2) var(--space-1);
  color: var(--color-fg-muted);
  font-size: var(--text-xs);
  font-weight: var(--weight-semibold);
  letter-spacing: var(--tracking-wide);
  text-transform: uppercase;
}

/* In the site's own nav the entries sit flat on the sidebar until hovered or current. */
.nav .nav-link:not(.active) {
  background: transparent;
}

.nav .nav-link:not(.active):hover {
  background: var(--layer-raised);
}

/* The main area fills everything right of the nav and is the scroll container, so its scrollbar
   sits at the window edge; the column inside is centered and capped for reading and demos, with
   side padding that shrinks on a narrow window. min-width: 0 lets the grid cell shrink below the
   column's content width. */
.main {
  min-width: 0;
  overflow-y: auto;
  padding: var(--space-10) clamp(var(--space-4), 4vw, var(--space-10)) var(--space-24);
}

.content {
  display: flex;
  flex-direction: column;
  gap: var(--space-4);
  max-width: 1200px;
  margin: 0 auto;
}
</style>
