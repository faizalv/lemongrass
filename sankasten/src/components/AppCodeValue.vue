<!-- SPDX-License-Identifier: Apache-2.0 -->
<!-- Copyright (C) 2026 The Sankasten Authors -->

<script setup lang="ts">
import { onBeforeUnmount, ref } from 'vue'
import AppIcon from './AppIcon.vue'

// A value that is typed or sent as written, such as an endpoint, a path or a selector, shown as an
// inset code pill. Clicking copies it, so a value read here can be pasted where it is used. A long
// value wraps inside its pill instead of being cut, since the whole value is what is looked up.
const props = defineProps<{ value: string }>()

const copied = ref(false)
let timer: ReturnType<typeof setTimeout> | undefined

async function copy(): Promise<void> {
  try {
    await navigator.clipboard.writeText(props.value)
  } catch {
    return
  }
  copied.value = true
  clearTimeout(timer)
  timer = setTimeout(() => (copied.value = false), 1200)
}

onBeforeUnmount(() => clearTimeout(timer))
</script>

<template>
  <button
    type="button"
    class="code-value"
    :class="{ copied }"
    :title="copied ? 'Copied' : `${value}\nClick to copy`"
    :aria-label="`Copy ${value}`"
    @click="copy"
  >
    <span class="text">{{ value }}</span>
    <AppIcon class="mark" :name="copied ? 'check' : 'copy'" :size="11" />
  </button>
</template>

<style scoped>
.code-value {
  box-sizing: border-box;
  display: inline-flex;
  align-items: center;
  gap: var(--space-2);
  max-width: 100%;
  padding: 1px var(--space-2);
  background: var(--color-surface-0);
  border: 1px solid var(--color-border-subtle);
  border-radius: var(--radius-sm);
  color: var(--color-fg-primary);
  font-family: var(--font-mono);
  font-size: var(--text-xs);
  line-height: 1.5;
  text-align: left;
  cursor: pointer;
  transition: border-color var(--duration-fast) var(--ease-out);
}

.code-value:hover {
  border-color: var(--color-border-strong);
}

.code-value:focus-visible {
  outline: 1px solid var(--color-amber);
  outline-offset: 1px;
}

.text {
  min-width: 0;
  overflow-wrap: anywhere;
}

/* The copy glyph shows only while the value is pointed at, the check once it has been copied. */
.mark {
  flex-shrink: 0;
  color: var(--color-fg-muted);
  opacity: 0;
  transition: opacity var(--duration-fast) var(--ease-out);
}

.code-value:hover .mark,
.code-value:focus-visible .mark {
  opacity: 1;
}

.code-value.copied {
  border-color: var(--color-success);
}

.code-value.copied .mark {
  color: var(--color-success);
  opacity: 1;
}
</style>
