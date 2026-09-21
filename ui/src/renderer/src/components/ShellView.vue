<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref } from 'vue'
import { attachShell, fitShell, type ShellSpec } from '../shellRegistry'

const props = defineProps<{
  spec: ShellSpec
}>()

const containerEl = ref<HTMLDivElement>()
let resizeObserver: ResizeObserver | undefined

onMounted(() => {
  if (!containerEl.value) return
  attachShell(props.spec, containerEl.value)
  resizeObserver = new ResizeObserver(() => fitShell(props.spec.id))
  resizeObserver.observe(containerEl.value)
})

onBeforeUnmount(() => resizeObserver?.disconnect())
</script>

<template>
  <div class="terminal-card fade-in">
    <div ref="containerEl" class="terminal-pane"></div>
  </div>
</template>

<style scoped>
/* FitAddon sizes the terminal off the inner element's own box, without
   accounting for any padding or border on it, so the card chrome lives on
   this wrapper instead. */
.terminal-card {
  flex: 1;
  min-height: 0;
  width: 100%;
  padding: var(--space-3);
  background: var(--color-surface-1);
  border: 1px solid var(--color-border-subtle);
  border-radius: var(--radius-lg);
  overflow: hidden;
}

.terminal-pane {
  height: 100%;
  width: 100%;
}
</style>
