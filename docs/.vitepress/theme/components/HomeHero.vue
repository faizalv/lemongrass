<!-- SPDX-License-Identifier: MPL-2.0 -->
<!-- Copyright (C) 2026 The Lemongrass Authors -->

<script setup lang="ts">
import { VPButton } from 'vitepress/theme'
import MarkIcon from './MarkIcon.vue'
import { VENDOR_MARKS } from './vendorMarks'

const vendors = [
  { id: 'claude', label: 'Claude Code' },
  { id: 'codex', label: 'Codex' },
  { id: 'agy', label: 'Antigravity' }
]
</script>

<template>
  <section class="hero">
    <img class="logo" src="/logo.svg" alt="" width="72" height="72" />
    <p class="name">Lemongrass</p>
    <h1 class="tagline">
      The agent centered IDE for
      <span class="vendors">
        <MarkIcon
          v-for="v in vendors"
          :key="v.id"
          :d="VENDOR_MARKS[v.id].d"
          :box="VENDOR_MARKS[v.id].box"
          :label="v.label"
        />
      </span>
    </h1>
    <p class="sub">
      Agents that know your project. Quality work out of the box. Full capability, with a safety check on every step.
    </p>
    <div class="actions">
      <VPButton size="big" theme="brand" text="Download" href="#install" />
      <VPButton size="big" theme="alt" text="GitHub" href="https://github.com/faizalv/lemongrass" />
    </div>
  </section>
</template>

<style scoped>
.hero {
  display: flex;
  flex-direction: column;
  align-items: center;
  text-align: center;
  padding: var(--space-16) 0 var(--space-12);
}

.logo {
  width: 72px;
  height: 72px;
  margin-bottom: var(--space-4);
}

.name {
  margin: 0;
  font-family: var(--font-display);
  font-size: var(--text-2xl);
  font-weight: var(--weight-bold);
  letter-spacing: var(--tracking-snug);
  color: var(--color-amber);
}

.tagline {
  margin: var(--space-4) 0 0;
  max-width: 18ch;
  font-size: clamp(2.25rem, 6vw, 3.5rem);
  line-height: var(--leading-tight);
  font-weight: var(--weight-bold);
  letter-spacing: var(--tracking-tight);
  color: var(--color-fg-primary);
}

.vendors {
  display: inline-flex;
  gap: 0.3em;
  vertical-align: -0.08em;
  margin-left: 0.15em;
  font-size: 0.85em;
}

.sub {
  margin: var(--space-6) 0 0;
  max-width: 40rem;
  font-size: var(--text-lg);
  line-height: var(--leading-relaxed);
  color: var(--color-fg-secondary);
}

.actions {
  display: flex;
  flex-wrap: wrap;
  justify-content: center;
  gap: var(--space-3);
  margin-top: var(--space-8);
}
</style>
