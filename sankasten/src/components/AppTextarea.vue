<!-- SPDX-License-Identifier: Apache-2.0 -->
<!-- Copyright (C) 2026 The Sankasten Authors -->

<script setup lang="ts">
import { ref } from 'vue'

// The textarea is the root, so id, rows, placeholder, disabled and listeners fall through, and a
// caller's class can still set layout such as flex or min-height.
withDefaults(
  defineProps<{
    size?: 'sm' | 'md'
    resize?: 'vertical' | 'none'
    mono?: boolean
  }>(),
  { size: 'md', resize: 'vertical', mono: false }
)

const model = defineModel<string>({ default: '' })
// The height the human dragged the resize handle to, in px, so a caller can keep it across a
// remount. Null leaves the height to rows and the caller's class.
const height = defineModel<number | null>('height', { default: null })
const element = ref<HTMLTextAreaElement | null>(null)

// Only a drag the human made is reported: the handle belongs to the textarea, so a press on it
// starts here, and a height that changed by the release is the new size. Layout changes never
// write the model.
function onPointerDown(): void {
  const before = element.value?.offsetHeight
  window.addEventListener(
    'pointerup',
    () => {
      const after = element.value?.offsetHeight
      if (after && after !== before) height.value = after
    },
    { once: true }
  )
}

defineExpose({ focus: () => element.value?.focus() })
</script>

<template>
  <textarea
    ref="element"
    v-model="model"
    class="app-textarea"
    :class="[`app-textarea--${size}`, { 'app-textarea--fixed': resize === 'none', mono }]"
    :style="height ? { height: `${height}px` } : undefined"
    @pointerdown="onPointerDown"
  />
</template>

<style scoped>
.app-textarea {
  box-sizing: border-box;
  width: 100%;
  padding: var(--space-2) var(--space-3);
  background: var(--layer-input);
  border: 1px solid var(--color-border-default);
  border-radius: var(--radius-md);
  color: var(--color-fg-primary);
  font-family: var(--font-body);
  line-height: 1.55;
  resize: vertical;
}

.app-textarea--sm {
  font-size: var(--text-xs);
}

.app-textarea--md {
  font-size: var(--text-sm);
}

.app-textarea--fixed {
  resize: none;
}

.app-textarea.mono {
  font-family: var(--font-mono);
}

.app-textarea::placeholder {
  color: var(--color-fg-muted);
}

.app-textarea:hover {
  border-color: var(--color-border-strong);
}

.app-textarea:focus {
  outline: none;
  border-color: var(--color-amber);
}

.app-textarea:disabled {
  color: var(--color-fg-disabled);
  cursor: not-allowed;
}
</style>
