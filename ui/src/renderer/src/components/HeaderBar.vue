<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref } from 'vue'

// Aliased so the template can reach it -- a bare `window` identifier in
// a Vue template resolves against the component instance, not globalThis.
const controls = window.api.windowControls

const isMaximized = ref(false)
let unsubscribe: (() => void) | undefined

onMounted(async () => {
  isMaximized.value = await controls.isMaximized()
  unsubscribe = controls.onMaximizedChange((maximized) => {
    isMaximized.value = maximized
  })
})

onBeforeUnmount(() => unsubscribe?.())
</script>

<template>
  <div class="header-bar">
    <div class="drag-region">
      <slot name="title" />
    </div>
    <div class="actions">
      <slot name="actions" />
    </div>
    <div class="window-controls">
      <button class="control" title="Minimize" @click="controls.minimize()">
        <svg width="10" height="10" viewBox="0 0 10 10">
          <line x1="1" y1="8" x2="9" y2="8" stroke="currentColor" stroke-width="1" />
        </svg>
      </button>
      <button
        class="control"
        :title="isMaximized ? 'Restore' : 'Maximize'"
        @click="controls.toggleMaximize()"
      >
        <svg v-if="!isMaximized" width="10" height="10" viewBox="0 0 10 10">
          <rect
            x="1"
            y="1"
            width="8"
            height="8"
            fill="none"
            stroke="currentColor"
            stroke-width="1"
          />
        </svg>
        <svg v-else width="10" height="10" viewBox="0 0 10 10">
          <rect
            x="0.5"
            y="2.5"
            width="6"
            height="6"
            fill="none"
            stroke="currentColor"
            stroke-width="1"
          />
          <path d="M2.5 2.5V0.5H9.5V7.5H7.5" fill="none" stroke="currentColor" stroke-width="1" />
        </svg>
      </button>
      <button class="control close" title="Close" @click="controls.close()">
        <svg width="10" height="10" viewBox="0 0 10 10">
          <line x1="1" y1="1" x2="9" y2="9" stroke="currentColor" stroke-width="1" />
          <line x1="9" y1="1" x2="1" y2="9" stroke="currentColor" stroke-width="1" />
        </svg>
      </button>
    </div>
  </div>
</template>

<style scoped>
.header-bar {
  display: flex;
  align-items: stretch;
  height: 46px;
  flex-shrink: 0;
  background: var(--color-surface-1);
  -webkit-app-region: drag;
}

.drag-region {
  flex: 1;
  min-width: 0;
  display: flex;
  align-items: center;
  padding: 0 var(--space-4);
}

.actions {
  display: flex;
  align-items: center;
  padding: 0 var(--space-2);
  -webkit-app-region: no-drag;
}

.window-controls {
  display: flex;
  align-items: center;
  gap: var(--space-1);
  padding: 0 var(--space-2);
  -webkit-app-region: no-drag;
}

.control {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 30px;
  height: 30px;
  background: transparent;
  border: none;
  border-radius: var(--radius-pill);
  color: var(--color-fg-secondary);
  cursor: pointer;
  transition: background var(--duration-fast) var(--ease-out);
}

.control:hover {
  background: var(--color-surface-3);
  color: var(--color-fg-primary);
}

.control.close:hover {
  background: var(--color-error);
  color: var(--color-white);
}
</style>
