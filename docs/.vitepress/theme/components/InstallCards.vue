<!-- SPDX-License-Identifier: MPL-2.0 -->
<!-- Copyright (C) 2026 The Lemongrass Authors -->

<script setup lang="ts">
import { computed, ref } from 'vue'
import MarkIcon from './MarkIcon.vue'
import { APPLE_MARK, FEDORA_MARK, WINDOWS_MARK } from './osMarks'

const RELEASES = 'https://github.com/faizalv/lemongrass/releases'

const platforms = [
  {
    id: 'fedora',
    name: 'Fedora',
    mark: FEDORA_MARK,
    text: `Install the rpm from the <a href="${RELEASES}">latest release</a>. The app updates itself from Settings, Updates.`
  },
  {
    id: 'macos',
    name: 'macOS',
    mark: APPLE_MARK,
    text: 'Run this in Terminal. It installs the build for your chip and opens the app with no warning.',
    command: 'curl -fsSL https://raw.githubusercontent.com/faizalv/lemongrass/main/scripts/install-mac.sh | bash'
  },
  {
    id: 'windows',
    name: 'Windows',
    mark: WINDOWS_MARK,
    text: `Download <code>lemongrass-&lt;version&gt;-setup.exe</code> from the <a href="${RELEASES}">latest release</a> and run it. It is not code signed yet, so Windows asks once: click More info, then Run anyway.`
  }
]

// One platform open at a time. The open card moves to the front of the grid and spans the full row, so the others
// slide down beneath it instead of sitting beside a tall card.
const openId = ref<string | null>(null)
const ordered = computed(() =>
  openId.value ? [...platforms].sort((a, b) => Number(b.id === openId.value) - Number(a.id === openId.value)) : platforms
)

function toggle(id: string): void {
  openId.value = openId.value === id ? null : id
}
</script>

<template>
  <section id="install" class="install">
    <h2 class="title">Install</h2>
    <TransitionGroup tag="div" name="card" class="cards">
      <div v-for="p in ordered" :key="p.id" class="card" :class="{ open: openId === p.id }">
        <button
          class="head"
          type="button"
          :aria-expanded="openId === p.id"
          :aria-controls="`install-${p.id}`"
          @click="toggle(p.id)"
        >
          <MarkIcon class="os" :d="p.mark" :box="24" :label="p.name" />
          <span class="name">{{ p.name }}</span>
          <svg class="chevron" viewBox="0 0 16 16" aria-hidden="true">
            <path d="M4 6l4 4l4-4" />
          </svg>
        </button>
        <div v-show="openId === p.id" :id="`install-${p.id}`" class="body">
          <p v-html="p.text" />
          <pre v-if="p.command"><code>{{ p.command }}</code></pre>
          <p class="more">
            Full steps, verifying a download and uninstalling are on the <a href="/install">install page</a>.
          </p>
        </div>
      </div>
    </TransitionGroup>
  </section>
</template>

<style scoped>
.install {
  /* The Download buttons jump here, and the fixed nav bar would otherwise cover the heading. */
  scroll-margin-top: calc(var(--vp-nav-height) + var(--space-6));
  padding: var(--space-12) 0;
}

.title {
  margin: 0 0 var(--space-6);
  font-size: var(--text-2xl);
  font-weight: var(--weight-bold);
  text-align: center;
  color: var(--color-fg-primary);
}

.cards {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: var(--space-4);
  align-items: start;
}

.card.open {
  grid-column: 1 / -1;
}

.card-move {
  transition: transform var(--duration-slow) var(--ease-default);
}

@media (prefers-reduced-motion: reduce) {
  .card-move {
    transition: none;
  }
}

.card {
  background: var(--layer-card);
  border: 1px solid var(--color-border-default);
  border-radius: var(--radius-xl);
  transition: border-color var(--duration-fast) var(--ease-default);
}

.card:hover,
.card.open {
  border-color: var(--color-border-strong);
}

.head {
  display: flex;
  align-items: center;
  gap: var(--space-3);
  width: 100%;
  padding: var(--space-5) var(--space-6);
  font: inherit;
  color: var(--color-fg-primary);
  background: none;
  border: 0;
  cursor: pointer;
  text-align: left;
}

.head:focus-visible {
  outline: 2px solid var(--color-amber);
  outline-offset: -2px;
  border-radius: var(--radius-xl);
}

.os {
  font-size: 1.5rem;
}

.name {
  flex: 1;
  font-size: var(--text-lg);
  font-weight: var(--weight-semibold);
}

.chevron {
  width: 16px;
  height: 16px;
  fill: none;
  stroke: var(--color-fg-muted);
  stroke-width: 1.6;
  stroke-linecap: round;
  stroke-linejoin: round;
  transition: transform var(--duration-base) var(--ease-default);
}

.open .chevron {
  transform: rotate(180deg);
}

.body {
  padding: 0 var(--space-6) var(--space-5);
  font-size: var(--text-sm);
  line-height: var(--leading-relaxed);
  color: var(--color-fg-secondary);
}

.body p {
  margin: 0 0 var(--space-3);
}

.body :deep(a) {
  color: var(--color-amber);
  text-decoration: underline;
  text-underline-offset: 2px;
}

.body :deep(code) {
  font-family: var(--font-mono);
  font-size: 0.9em;
  padding: 0.1em 0.35em;
  border-radius: var(--radius-sm);
  background: var(--layer-raised);
  color: var(--color-fg-primary);
}

.body pre {
  margin: 0 0 var(--space-3);
  padding: var(--space-3) var(--space-4);
  overflow-x: auto;
  border-radius: var(--radius-md);
  background: var(--layer-raised);
}

.body pre code {
  padding: 0;
  background: none;
  white-space: pre;
}

.more {
  color: var(--color-fg-muted);
}

@media (max-width: 768px) {
  .cards {
    grid-template-columns: 1fr;
  }
}
</style>
