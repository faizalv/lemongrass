<!-- SPDX-License-Identifier: Apache-2.0 -->
<!-- Copyright (C) 2026 The Sankasten Authors -->

<script setup lang="ts">
import { computed, inject } from 'vue'
import { capsuleHue } from '../lib/capsuleHue'
import { MOSAIC_KEY } from './mosaic'

// A named item in an AppMosaic, on a neutral surface so the shared buttons sit on it as on any
// card. At rest it shows only its label; clicking the label expands it to the full width with the
// default slot under the label. The dot's hue comes from the content, so one name always gets one
// color without tinting the surface the buttons sit on.
const props = withDefaults(
  defineProps<{
    // Shown at rest, and the identity the mosaic tracks for expansion.
    label: string
    // What the dot's hue is computed from, when it should differ from the label.
    hueKey?: string
  }>(),
  { hueKey: undefined }
)

const mosaic = inject(MOSAIC_KEY, null)

const hue = computed(() => capsuleHue(props.hueKey ?? props.label))
const expanded = computed(() => mosaic?.expanded.value === props.label)

function onKey(event: KeyboardEvent): void {
  if (event.key === 'Escape' && expanded.value) mosaic?.toggle(props.label)
}
</script>

<template>
  <div class="app-capsule" :class="{ expanded }" @keydown="onKey">
    <!-- Only the label toggles, so a click in the details, such as selecting an address, never
         folds the capsule. -->
    <button
      type="button"
      class="capsule-head"
      :aria-expanded="expanded"
      :title="label"
      @click="mosaic?.toggle(label)"
    >
      <span class="capsule-dot" :class="`hue-${hue}`" aria-hidden="true" />
      <span class="capsule-label">{{ label }}</span>
    </button>
    <div v-if="expanded" class="capsule-detail">
      <slot />
    </div>
  </div>
</template>

<style scoped>
.app-capsule {
  box-sizing: border-box;
  flex: 1 1 auto;
  min-width: 0;
  display: flex;
  flex-direction: column;
  border: 1px solid var(--color-border-subtle);
  border-radius: var(--radius-pill);
  background: var(--color-surface-2);
  color: var(--color-fg-primary);
  font-family: var(--font-body);
  font-size: var(--text-xs);
  transition: border-color var(--duration-fast) var(--ease-out);
}

.app-capsule:hover {
  border-color: var(--color-border-strong);
}

.app-capsule.expanded {
  flex-basis: 100%;
  border-color: var(--color-border-default);
  border-radius: var(--radius-lg);
}

.capsule-head {
  display: flex;
  align-items: center;
  gap: var(--space-2);
  min-width: 0;
  padding: 5px var(--space-3);
  background: none;
  border: none;
  border-radius: inherit;
  color: inherit;
  font: inherit;
  font-weight: var(--weight-semibold);
  line-height: 1.3;
  text-align: left;
  cursor: pointer;
}

.capsule-head:focus-visible {
  outline: 1px solid var(--color-amber);
  outline-offset: 1px;
}

.capsule-dot {
  flex-shrink: 0;
  width: 6px;
  height: 6px;
  border-radius: var(--radius-pill);
  background: var(--capsule-hue);
}

.capsule-label {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.capsule-detail {
  display: flex;
  flex-direction: column;
  gap: var(--space-2);
  padding: 0 var(--space-3) var(--space-2);
}

.hue-amber {
  --capsule-hue: var(--color-amber);
}

.hue-coral {
  --capsule-hue: var(--color-coral);
}

.hue-fuchsia {
  --capsule-hue: var(--color-fuchsia);
}

.hue-jade {
  --capsule-hue: var(--color-jade);
}

.hue-sapphire {
  --capsule-hue: var(--color-sapphire);
}

.hue-teal {
  --capsule-hue: var(--color-teal);
}

.hue-violet {
  --capsule-hue: var(--color-violet);
}
</style>
