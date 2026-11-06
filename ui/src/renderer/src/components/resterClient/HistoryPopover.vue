<!-- SPDX-License-Identifier: MPL-2.0 -->
<!-- Copyright (C) 2026 The Lemongrass Authors -->

<script setup lang="ts">
import { AppButton, AppConfirmButton, AppIcon, placeFloat, placementStyle } from '@sankasten'
import { nextTick, onBeforeUnmount, onMounted, ref } from 'vue'
import RunHistory from './RunHistory.vue'
import { formatAge } from '../../relativeAge'
import { ageNow } from '../../ageClock'
import { noteVaultActivity } from '../../vaultSession'
import type { ResterClientRun } from '../../../../preload/types'

defineProps<{
  // When the shown response came back.
  at: string
  runs: ResterClientRun[]
  runId: string | null
}>()

const emit = defineEmits<{
  pick: [run: ResterClientRun]
  remove: [runId: string]
  clear: []
}>()

// The panel renders in body because the box head it opens from scrolls sideways and clips.
const POPOVER_WIDTH = 320
const POPOVER_GAP = 8

const open = ref(false)
const rootRef = ref<HTMLElement | null>(null)
const popoverRef = ref<HTMLElement | null>(null)
const popoverStyle = ref<Record<string, string>>({})

// Right-aligned to the trigger, like the menu it was before it moved to body.
function place(): void {
  if (!open.value || !rootRef.value || !popoverRef.value) return
  const trigger = rootRef.value.getBoundingClientRect()
  const anchor = new DOMRect(
    trigger.right - POPOVER_WIDTH,
    trigger.top,
    POPOVER_WIDTH,
    trigger.height
  )
  const placement = placeFloat(anchor, POPOVER_WIDTH, popoverRef.value.offsetHeight, POPOVER_GAP)
  popoverStyle.value = placementStyle(placement)
}

async function toggle(): Promise<void> {
  open.value = !open.value
  if (!open.value) return
  await nextTick()
  place()
}

function pick(run: ResterClientRun): void {
  emit('pick', run)
  open.value = false
}

function onDocumentClick(event: MouseEvent): void {
  const target = event.target as Node
  if (!open.value || rootRef.value?.contains(target) || popoverRef.value?.contains(target)) return
  open.value = false
}

function onDocumentKeydown(event: KeyboardEvent): void {
  if (open.value && event.key === 'Escape') open.value = false
}

onMounted(() => {
  document.addEventListener('click', onDocumentClick, true)
  document.addEventListener('keydown', onDocumentKeydown)
  document.addEventListener('scroll', place, true)
  window.addEventListener('resize', place)
})
onBeforeUnmount(() => {
  document.removeEventListener('click', onDocumentClick, true)
  document.removeEventListener('keydown', onDocumentKeydown)
  document.removeEventListener('scroll', place, true)
  window.removeEventListener('resize', place)
})
</script>

<template>
  <div ref="rootRef" class="history">
    <AppButton
      variant="text"
      size="sm"
      class="age"
      :class="{ open }"
      :pressed="open"
      :title="`${new Date(at).toLocaleString()}. Click for this request's history.`"
      @click="toggle"
    >
      {{ formatAge(at, ageNow) }}
      <AppIcon class="caret" name="chevronDown" :size="9" />
    </AppButton>
    <Teleport to="body">
      <div
        v-if="open"
        ref="popoverRef"
        class="popover rester-float"
        :style="popoverStyle"
        @click.capture="noteVaultActivity"
        @keydown.capture="noteVaultActivity"
      >
        <div class="popover-head">
          <span class="popover-title">
            {{ runs.length }} {{ runs.length === 1 ? 'run' : 'runs' }}
          </span>
          <AppConfirmButton
            v-if="runs.length > 0"
            variant="ghost"
            size="sm"
            confirm-label="Click again to clear"
            @confirm="emit('clear')"
          >
            Clear
          </AppConfirmButton>
        </div>
        <div class="popover-list">
          <RunHistory
            :runs="runs"
            :selected-run-id="runId"
            @pick="pick"
            @remove="emit('remove', $event)"
          />
        </div>
      </div>
    </Teleport>
  </div>
</template>

<style scoped>
.history {
  position: relative;
}

.caret {
  transition: transform var(--duration-fast) var(--ease-out);
}

.age.open .caret {
  transform: rotate(180deg);
}

.popover {
  position: fixed;
  z-index: 80;
  width: 320px;
  display: flex;
  flex-direction: column;
  background: var(--layer-float);
  border: 1px solid var(--color-border-default);
  border-radius: var(--radius-lg);
  box-shadow: var(--shadow-xl);
  overflow: hidden;
}

.popover-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: var(--space-2) var(--space-3);
  border-bottom: 1px solid var(--color-border-default);
}

.popover-title {
  color: var(--color-fg-muted);
  font-size: var(--text-xs);
}

.popover-list {
  min-height: 0;
  max-height: 360px;
  overflow-y: auto;
  padding: var(--space-2);
}
</style>
