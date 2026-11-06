<!-- SPDX-License-Identifier: Apache-2.0 -->
<!-- Copyright (C) 2026 The Sankasten Authors -->

<script setup lang="ts">
// A choice in a grid: a glyph, a label and one line of detail, such as an agent in a picker or a
// tool with its status. md is a square tile with a large glyph; sm is a compact row with the glyph
// beside the text, for a grid that keeps growing. Selected draws the amber border and fill.
withDefaults(
  defineProps<{
    label: string
    detail?: string
    // Detail as code, such as a command, in the mono face.
    mono?: boolean
    // Colors the detail line, for a status that is on (success) or needs the user (danger).
    tone?: 'neutral' | 'accent' | 'success' | 'danger'
    selected?: boolean
    size?: 'sm' | 'md'
  }>(),
  { detail: undefined, mono: false, tone: 'neutral', selected: false, size: 'md' }
)

defineOptions({ inheritAttrs: false })
</script>

<!-- Attributes and listeners go to the inner button, so a control in the corner slot is not
     nested in it and its clicks do not reach the pad. -->
<template>
  <div class="app-pad" :class="[`size-${size}`, { selected, 'has-corner': $slots.corner }]">
    <button type="button" class="pad-main" v-bind="$attrs">
      <span v-if="$slots.icon" class="pad-icon"><slot name="icon" /></span>
      <span class="pad-text">
        <span class="pad-label">{{ label }}</span>
        <span v-if="detail" class="pad-detail" :class="[`tone-${tone}`, { mono }]">{{
          detail
        }}</span>
      </span>
    </button>
    <span v-if="$slots.corner" class="pad-corner"><slot name="corner" /></span>
  </div>
</template>

<style scoped>
.app-pad {
  position: relative;
  min-width: 0;
}

.pad-main {
  box-sizing: border-box;
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: var(--space-1);
  width: 100%;
  height: 100%;
  min-width: 0;
  padding: var(--space-4) var(--space-2) var(--space-3);
  background: transparent;
  border: 1px solid var(--color-border-default);
  border-radius: var(--radius-md);
  color: var(--color-fg-muted);
  font-family: var(--font-body);
  font-size: var(--text-sm);
  cursor: pointer;
  transition:
    background var(--duration-fast) var(--ease-out),
    border-color var(--duration-fast) var(--ease-out),
    color var(--duration-fast) var(--ease-out);
}

.pad-main:hover {
  background: var(--color-surface-2);
  color: var(--color-fg-primary);
}

.pad-main:focus-visible {
  outline: 1px solid var(--color-amber);
  outline-offset: 1px;
}

.selected .pad-main {
  border-color: var(--color-amber);
  background: var(--color-amber-muted);
  color: var(--color-fg-primary);
}

.pad-icon {
  display: inline-flex;
  margin-bottom: var(--space-1);
}

.pad-icon :deep(svg) {
  height: 24px;
  width: auto;
}

.pad-label {
  max-width: 100%;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  color: var(--color-fg-secondary);
  font-weight: var(--weight-medium);
}

.pad-main:hover .pad-label,
.selected .pad-label {
  color: var(--color-fg-primary);
}

.pad-detail {
  max-width: 100%;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  color: var(--color-fg-muted);
  font-size: var(--text-xs);
}

.pad-detail.mono {
  font-family: var(--font-mono);
}

.pad-detail.tone-accent {
  color: var(--color-fg-accent);
}

.pad-detail.tone-success {
  color: var(--color-success);
}

.pad-detail.tone-danger {
  color: var(--color-error);
}

.pad-corner {
  position: absolute;
  top: var(--space-1);
  right: var(--space-1);
}
.pad-text {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: var(--space-1);
  min-width: 0;
  max-width: 100%;
}

.size-sm .pad-main {
  flex-direction: row;
  justify-content: flex-start;
  gap: var(--space-2);
  padding: var(--space-2) var(--space-3);
  text-align: left;
}

/* Room for the corner control, centered on the right edge of a compact pad. */
.size-sm.has-corner .pad-main {
  padding-right: var(--space-8);
}

.size-sm .pad-icon {
  margin-bottom: 0;
}

.size-sm .pad-icon :deep(svg) {
  height: 16px;
}

.size-sm .pad-text {
  align-items: flex-start;
  gap: 0;
}

.size-sm .pad-label {
  font-size: var(--text-xs);
}

.size-sm .pad-corner {
  top: 50%;
  transform: translateY(-50%);
}
</style>
