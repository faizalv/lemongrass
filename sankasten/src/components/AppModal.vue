<!-- SPDX-License-Identifier: Apache-2.0 -->
<!-- Copyright (C) 2026 The Sankasten Authors -->

<script setup lang="ts">
import { onBeforeUnmount, onMounted, provide, useId } from 'vue'
import { modalActionsKey } from './modalActions'

withDefaults(
  defineProps<{
    title?: string
    size?: 'sm' | 'md' | 'lg'
    scope?: 'app' | 'container'
    flush?: boolean
    label?: string
  }>(),
  { title: undefined, size: 'sm', scope: 'container', flush: false, label: undefined }
)

const emit = defineEmits<{
  close: []
}>()

function onKeydown(event: KeyboardEvent): void {
  if (event.key === 'Escape') emit('close')
}

// AppModalActions inside the body teleports its buttons here, so actions owned by a child component
// stay in the fixed footer instead of scrolling away with the body.
const actionsId = `sk-modal-actions-${useId()}`
provide(modalActionsKey, `#${CSS.escape(actionsId)}`)

onMounted(() => window.addEventListener('keydown', onKeydown))
onBeforeUnmount(() => window.removeEventListener('keydown', onKeydown))
</script>

<template>
  <div class="modal-overlay" :class="scope" @click.self="emit('close')">
    <div
      class="modal-dialog"
      :class="[size, { flush }]"
      role="dialog"
      aria-modal="true"
      :aria-label="label ?? title"
    >
      <button class="modal-close" aria-label="Close" @click="emit('close')">
        <svg
          width="12"
          height="12"
          viewBox="0 0 12 12"
          fill="none"
          stroke="currentColor"
          stroke-width="1.3"
          stroke-linecap="round"
        >
          <path d="M2.5 2.5l7 7M9.5 2.5l-7 7" />
        </svg>
      </button>
      <h3 v-if="title" class="modal-title">{{ title }}</h3>
      <div class="modal-body">
        <slot />
      </div>
      <div v-if="$slots.footer" class="modal-footer">
        <slot name="footer" />
      </div>
      <div :id="actionsId" class="modal-footer modal-actions" />
    </div>
  </div>
</template>

<style scoped>
.modal-overlay {
  z-index: 60;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: var(--space-8);
  background: var(--color-bg-overlay);
}

/* A short window, such as a laptop at 125% scaling, keeps more of its height for the dialog. */
@media (max-height: 720px) {
  .modal-overlay {
    padding: var(--space-4);
  }
}

.modal-overlay.container {
  position: absolute;
  inset: 0;
}

.modal-overlay.app {
  position: fixed;
  inset: 0;
  z-index: 900;
}

.modal-dialog {
  position: relative;
  display: flex;
  flex-direction: column;
  gap: var(--space-3);
  width: 100%;
  min-height: 0;
  max-height: 100%;
  padding: var(--space-6);
  overflow: hidden;
  background: var(--color-surface-1);
  border-radius: var(--radius-xl);
  box-shadow: var(--shadow-xl);
}

.modal-dialog.sm {
  max-width: 420px;
}

.modal-dialog.md {
  max-width: 640px;
}

.modal-dialog.lg {
  max-width: 1000px;
  height: min(680px, 100%);
}

.modal-dialog.flush {
  gap: 0;
  padding: 0;
}

.modal-close {
  position: absolute;
  top: var(--space-3);
  right: var(--space-3);
  z-index: 2;
  display: flex;
  padding: var(--space-2);
  background: transparent;
  border: none;
  border-radius: var(--radius-pill);
  color: var(--color-fg-secondary);
  cursor: pointer;
  transition:
    background var(--duration-fast) var(--ease-out),
    color var(--duration-fast) var(--ease-out);
}

.modal-close:hover {
  background: var(--color-surface-2);
  color: var(--color-fg-primary);
}

.modal-title {
  flex-shrink: 0;
  padding-right: var(--space-6);
  overflow-wrap: anywhere;
  color: var(--color-fg-primary);
  font-family: var(--font-display);
  font-size: var(--text-md);
  font-weight: var(--weight-semibold);
}

/* The dialog never grows past the window: the title and footer stay put and the body scrolls between
   them. The padding, taken back by the margin, keeps focus rings at the body's edges from being cut. */
.modal-body {
  display: flex;
  flex-direction: column;
  gap: var(--space-3);
  min-height: 0;
  margin: calc(var(--space-1) * -1);
  padding: var(--space-1);
  overflow-y: auto;
  color: var(--color-fg-secondary);
  font-size: var(--text-sm);
  line-height: var(--leading-relaxed);
}

.modal-dialog.lg .modal-body {
  flex: 1;
}

/* A large or flush dialog lays out its own scroll areas inside a body of fixed height. */
.modal-dialog.lg .modal-body,
.modal-dialog.flush .modal-body {
  margin: 0;
  padding: 0;
  overflow: hidden;
}

.modal-dialog.flush .modal-body {
  flex: 1;
  gap: 0;
}

.modal-footer {
  display: flex;
  flex-shrink: 0;
  justify-content: flex-end;
  gap: var(--space-2);
}

.modal-actions:empty {
  display: none;
}
</style>
