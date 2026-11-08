<!-- SPDX-License-Identifier: Apache-2.0 -->
<!-- Copyright (C) 2026 The Sankasten Authors -->

<script setup lang="ts">
// A framed message inside a surface that asks the reader to know or do something, such as a
// missing grant or an unlinked config. It is not for status, which is a capsule or plain text, and
// not for an empty view, which is AppEmptyState.
import { computed } from 'vue'
import AppIcon from './AppIcon.vue'
import type { AppIconName } from '../icons/appIcons'

const props = withDefaults(
  defineProps<{
    tone?: 'warning' | 'info' | 'danger'
    title?: string
  }>(),
  { tone: 'warning', title: '' }
)

const icon = computed((): AppIconName => (props.tone === 'info' ? 'info' : 'warning'))
</script>

<template>
  <div
    class="app-banner"
    :class="[`tone-${tone}`, { 'has-title': title || $slots.title }]"
    :role="tone === 'danger' ? 'alert' : 'note'"
  >
    <AppIcon class="banner-icon" :name="icon" :size="14" />
    <div class="banner-body">
      <p v-if="title || $slots.title" class="banner-title">
        <slot name="title">{{ title }}</slot>
      </p>
      <div v-if="$slots.default" class="banner-text"><slot /></div>
    </div>
    <div v-if="$slots.actions" class="banner-actions"><slot name="actions" /></div>
  </div>
</template>

<style scoped>
.app-banner {
  box-sizing: border-box;
  display: flex;
  align-items: flex-start;
  gap: var(--space-3);
  width: 100%;
  padding: var(--space-2) var(--space-3);
  border: 1px solid;
  border-radius: var(--radius-md);
  color: var(--color-fg-primary);
  font-family: var(--font-body);
  font-size: var(--text-sm);
  line-height: var(--leading-snug);
}

.tone-warning {
  border-color: var(--color-amber-dim);
  background: var(--color-amber-muted);
}

.tone-info {
  border-color: color-mix(in srgb, var(--color-info) 45%, transparent);
  background: color-mix(in srgb, var(--color-info) 12%, transparent);
}

.tone-danger {
  border-color: color-mix(in srgb, var(--color-error) 45%, transparent);
  background: var(--color-error-muted);
}

/* Nudged to sit on the first line of text, title or not. */
.banner-icon {
  flex-shrink: 0;
  margin-top: 2px;
}

.tone-warning .banner-icon {
  color: var(--color-warning);
}

.tone-info .banner-icon {
  color: var(--color-info);
}

.tone-danger .banner-icon {
  color: var(--color-error);
}

.banner-body {
  display: flex;
  flex: 1;
  flex-direction: column;
  gap: var(--space-1);
  min-width: 0;
}

.banner-title {
  margin: 0;
  font-weight: var(--weight-medium);
}

.banner-text {
  color: var(--color-fg-secondary);
  font-size: var(--text-xs);
  line-height: var(--leading-relaxed);
  user-select: text;
}

/* Without a title the text is the message itself, so it reads at the title's weight. */
.app-banner:not(.has-title) .banner-text {
  color: var(--color-fg-primary);
  font-size: var(--text-sm);
  line-height: var(--leading-snug);
}

.banner-actions {
  display: flex;
  flex-shrink: 0;
  align-items: center;
  gap: var(--space-2);
  align-self: center;
}
</style>
