<!-- SPDX-License-Identifier: MPL-2.0 -->
<!-- Copyright (C) 2026 The Lemongrass Authors -->

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { AppNavItem } from '@sankasten'
import NodeModal from './NodeModal.vue'

type Phase = 'checking' | 'connecting' | 'shared' | 'off'

const phase = ref<Phase>('checking')
const open = ref(false)
let timer: ReturnType<typeof setTimeout> | undefined

const view = computed(() => {
  switch (phase.value) {
    case 'checking':
      return { text: 'Checking', dot: 'Checking' }
    case 'connecting':
      return { text: 'Connecting', dot: 'Connecting' }
    case 'shared':
      return { text: 'Shared', dot: 'Connected' }
    default:
      return { text: 'Off', dot: 'Disconnected' }
  }
})

// Polls quickly while ngrok is connecting, which takes seconds, so Connecting does not linger.
async function refresh(): Promise<void> {
  clearTimeout(timer)
  try {
    const st = (await window.api.node.state()).status
    phase.value = st.running ? 'shared' : st.connecting ? 'connecting' : 'off'
  } catch {
    phase.value = 'off'
  }
  timer = setTimeout(refresh, phase.value === 'connecting' ? 2_000 : 15_000)
}

function close(): void {
  open.value = false
  void refresh()
}

onMounted(() => void refresh())
onBeforeUnmount(() => clearTimeout(timer))
</script>

<template>
  <div class="node-panel">
    <AppNavItem size="sm" label="Node" :state="view.text" @click="open = true">
      <template #icon>
        <svg
          viewBox="0 0 14 14"
          fill="none"
          stroke="currentColor"
          stroke-width="1.2"
          stroke-linecap="round"
        >
          <circle cx="7" cy="7" r="2" />
          <circle cx="2.5" cy="3" r="1.2" />
          <circle cx="11.5" cy="3" r="1.2" />
          <circle cx="7" cy="12" r="1.2" />
          <path d="M3.5 3.8L5.5 5.8M10.5 3.8L8.5 5.8M7 9v1.8" />
        </svg>
      </template>
      <template #trailing><span class="node-dot" :class="phase" :title="view.dot" /></template>
    </AppNavItem>
    <NodeModal v-if="open" @close="close" />
  </div>
</template>

<style scoped>
.node-panel {
  flex-shrink: 0;
  padding: var(--space-2);
  border-top: 1px solid var(--color-border-default);
}

.node-dot {
  flex-shrink: 0;
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: var(--color-error);
}

.node-dot.shared {
  background: var(--color-success);
}

.node-dot.checking,
.node-dot.connecting {
  background: var(--color-fg-muted);
  animation: node-pulse 1.2s ease-in-out infinite;
}

@keyframes node-pulse {
  50% {
    opacity: 0.3;
  }
}
</style>
