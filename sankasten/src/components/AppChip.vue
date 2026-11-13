<!-- SPDX-License-Identifier: Apache-2.0 -->
<!-- Copyright (C) 2026 The Sankasten Authors -->

<script setup lang="ts">
import AppIcon from './AppIcon.vue'

// A value in a pill, such as a table name or a path in a list of chips. It is a value, not a
// control, so it keeps one small height of its own instead of the control scale; set chips in a
// wrapping flex row. Attributes and listeners (tabindex, title, @mouseenter) fall through to the
// root, so a caller can make a chip focusable and hang a popover on it.
withDefaults(
  defineProps<{
    // default is surface-2; quiet is surface-3 with secondary text (a read-only value or a global
    // entry); the tones are a muted fill with the tone's text, for a checked state.
    tone?: 'default' | 'quiet' | 'accent' | 'info' | 'success' | 'danger'
    mono?: boolean
    // Muted, struck-through text, for an entry that is listed but switched off.
    struck?: boolean
    // Muted text after the value, such as a scope.
    tag?: string
    // An inline text action after the value, in amber; emits action.
    actionLabel?: string
    // Shows the remove glyph, which emits remove; removeLabel is its title and aria-label.
    removable?: boolean
    removeLabel?: string
  }>(),
  {
    tone: 'default',
    mono: false,
    struck: false,
    tag: undefined,
    actionLabel: undefined,
    removable: false,
    removeLabel: 'Remove'
  }
)

const emit = defineEmits<{ remove: []; action: [] }>()
</script>

<template>
  <span class="app-chip" :class="[`tone-${tone}`, { mono, struck }]">
    <span class="chip-value"><slot /></span>
    <span v-if="tag" class="chip-tag">{{ tag }}</span>
    <button v-if="actionLabel" type="button" class="chip-action" @click="emit('action')">
      {{ actionLabel }}
    </button>
    <button
      v-if="removable"
      type="button"
      class="chip-remove"
      :title="removeLabel"
      :aria-label="removeLabel"
      @click="emit('remove')"
    >
      <AppIcon name="close" :size="10" />
    </button>
  </span>
</template>

<style scoped>
.app-chip {
  box-sizing: border-box;
  display: inline-flex;
  align-items: center;
  gap: var(--space-1);
  max-width: 100%;
  height: 20px;
  padding: 0 var(--space-2);
  border-radius: var(--radius-pill);
  background: var(--color-surface-2);
  color: var(--color-fg-primary);
  font-family: var(--font-body);
  font-size: var(--text-xs);
  line-height: 1;
  white-space: nowrap;
}

.app-chip.mono {
  font-family: var(--font-mono);
}

.app-chip:focus-visible {
  outline: 1px solid var(--color-amber);
  outline-offset: 1px;
}

.tone-quiet {
  background: var(--color-surface-3);
  color: var(--color-fg-secondary);
}

.tone-accent,
.tone-info,
.tone-success,
.tone-danger {
  /* Muted text drops below its contrast floor on a tint, so muted reads as secondary here. */
  --color-fg-muted: var(--color-fg-secondary);
}

.tone-accent {
  background: var(--color-amber-muted);
  color: var(--color-amber);
}

.tone-info {
  background: color-mix(in oklab, var(--color-info) 14%, transparent);
  color: var(--color-info);
}

.tone-success {
  background: var(--color-success-muted);
  color: var(--color-success);
}

.tone-danger {
  background: var(--color-error-muted);
  color: var(--color-error);
}

.chip-value {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
}

.struck .chip-value {
  color: var(--color-fg-muted);
  text-decoration: line-through;
}

.chip-tag {
  flex-shrink: 0;
  color: var(--color-fg-muted);
}

.chip-action {
  flex-shrink: 0;
  padding: 0;
  background: none;
  border: none;
  color: var(--color-amber);
  font: inherit;
  font-family: var(--font-body);
  cursor: pointer;
}

.chip-action:hover {
  text-decoration: underline;
}

.chip-action:focus-visible,
.chip-remove:focus-visible {
  outline: 1px solid var(--color-amber);
  border-radius: var(--radius-sm);
}

/* Muted on a neutral chip; on a toned chip it takes the chip's own color. */
.chip-remove {
  display: inline-flex;
  flex-shrink: 0;
  padding: 0;
  background: none;
  border: none;
  color: var(--color-fg-muted);
  cursor: pointer;
}

.chip-remove:hover {
  color: var(--color-fg-primary);
}

.app-chip:not(.tone-default):not(.tone-quiet) .chip-remove {
  color: inherit;
}
</style>
