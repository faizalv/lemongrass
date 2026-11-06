<!-- SPDX-License-Identifier: Apache-2.0 -->
<!-- Copyright (C) 2026 The Sankasten Authors -->

<script setup lang="ts">
import AppBadge from './AppBadge.vue'
import AppIcon from './AppIcon.vue'

// A sidebar entry card: a full-width raised button that opens a view, a panel or a fold. The
// panel around it (border, padding, margins) stays with the caller.
withDefaults(
  defineProps<{
    label?: string
    // Secondary text after the label, such as a status or a branch; it takes the free width and
    // ellipsizes.
    state?: string
    // The view it opens is showing, or the thing it stands for is on: amber-muted.
    active?: boolean
    // danger overrides active, for an entry that needs attention now (red fill and red count).
    tone?: 'neutral' | 'danger'
    // A count pill on the right, hidden at 0 or when absent.
    count?: number
    countTitle?: string
    // Something needs the user: a red "!" pill after the count, with this text as its title and
    // accessible name.
    alert?: string
    // Set for an entry that folds a section open below it: a trailing chevron that turns while
    // open, and aria-expanded.
    expanded?: boolean
    size?: 'sm' | 'md'
  }>(),
  {
    label: undefined,
    state: undefined,
    active: false,
    tone: 'neutral',
    count: undefined,
    countTitle: undefined,
    alert: undefined,
    expanded: undefined,
    size: 'md'
  }
)
</script>

<!-- Attributes and listeners, @click and title included, fall through to the native button. -->
<template>
  <button
    type="button"
    class="app-nav-item"
    :class="[`size-${size}`, `tone-${tone}`, { active }]"
    :aria-expanded="expanded"
  >
    <span v-if="$slots.icon" class="nav-icon"><slot name="icon" /></span>
    <span class="nav-label"
      ><slot>{{ label }}</slot></span
    >
    <span class="nav-state" :title="state">{{ state }}</span>
    <slot name="trailing" />
    <AppBadge
      v-if="count"
      :tone="tone === 'danger' ? 'danger' : 'accent'"
      :title="countTitle"
      class="nav-count"
    >
      {{ count }}
    </AppBadge>
    <AppBadge
      v-if="alert"
      tone="danger"
      class="nav-alert"
      role="img"
      :aria-label="alert"
      :title="alert"
      >!</AppBadge
    >
    <AppIcon
      v-if="expanded !== undefined"
      class="nav-chevron"
      :class="{ open: expanded }"
      name="chevronRight"
      :size="10"
    />
  </button>
</template>

<style scoped>
/* The height comes from the control scale like every single-line control; the state span is
   always present so trailing parts sit at the right edge whether or not there is a state. */
.app-nav-item {
  --h: var(--control-h-md);
  box-sizing: border-box;
  display: flex;
  align-items: center;
  gap: 6px;
  width: 100%;
  height: var(--h);
  padding: 0 var(--space-3);
  background: var(--layer-raised);
  border: none;
  border-radius: var(--radius-md);
  color: var(--color-fg-primary);
  font-family: var(--font-body);
  font-size: var(--text-xs);
  font-weight: var(--weight-semibold);
  text-align: left;
  cursor: pointer;
  transition:
    background var(--duration-fast) var(--ease-out),
    color var(--duration-fast) var(--ease-out);
}

.size-sm {
  --h: var(--control-h-sm);
  padding: 0 var(--space-2);
}

.app-nav-item:hover {
  background: var(--layer-raised-hover);
}

.app-nav-item.active {
  background: var(--color-amber-muted);
  color: var(--color-fg-accent);
}

.app-nav-item.tone-danger,
.app-nav-item.tone-danger:hover {
  background: var(--color-error-muted);
  color: var(--color-error);
}

.app-nav-item:focus-visible {
  outline: 1px solid var(--color-amber);
  outline-offset: 1px;
}

.nav-icon {
  display: inline-flex;
  flex-shrink: 0;
}

.nav-icon :deep(svg) {
  height: 13px;
  width: auto;
}

.nav-label {
  flex-shrink: 0;
}

.nav-state {
  flex: 1;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  color: var(--color-fg-secondary);
  font-weight: var(--weight-regular);
}

.nav-alert {
  font-weight: var(--weight-bold);
}

.nav-chevron {
  flex-shrink: 0;
  color: var(--color-fg-muted);
  transition: transform var(--duration-fast) var(--ease-out);
}

.nav-chevron.open {
  transform: rotate(90deg);
}
</style>
