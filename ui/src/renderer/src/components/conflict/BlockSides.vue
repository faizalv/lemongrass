<!-- SPDX-License-Identifier: MPL-2.0 -->
<!-- Copyright (C) 2026 The Lemongrass Authors -->

<script setup lang="ts">
import { computed, ref } from 'vue'
import { AppButton } from '@sankasten'
import type { BlockHold } from '../../conflictDecisions'
import type { GitConflictChunk } from '../../../../preload/types'

// A block at a glance: both sides side by side and, once something is written, the result under
// them. The side the result equals is outlined, so a take reads without comparing lines.
const props = defineProps<{
  sides: Extract<GitConflictChunk, { kind: 'conflict' }>
  hold: BlockHold
  leftRef: string
  rightRef: string
}>()

// Past this many lines a box is cut until the human opens it.
const SHORT = 8
const full = ref(false)

const taken = computed(() => (props.hold.kind === 'written' ? (props.hold.sides ?? []) : []))
const result = computed(() => (props.hold.kind === 'written' ? props.hold.lines : null))
const longest = computed(() =>
  Math.max(props.sides.left.length, props.sides.right.length, result.value?.length ?? 0)
)

function shown(lines: string[]): string {
  const cut = full.value ? lines : lines.slice(0, SHORT)
  return cut.map((line) => line.replace(/\r$/, '')).join('\n')
}

function more(lines: string[]): number {
  return full.value ? 0 : Math.max(0, lines.length - SHORT)
}
</script>

<template>
  <div class="block-sides">
    <div class="sides">
      <section class="box" :class="{ taken: taken.includes('left') }">
        <header class="box-head" :title="leftRef">{{ leftRef }}</header>
        <pre v-if="sides.left.length" class="code">{{ shown(sides.left) }}</pre>
        <p v-else class="empty">No lines</p>
        <p v-if="more(sides.left)" class="cut">{{ more(sides.left) }} more lines</p>
      </section>
      <section class="box" :class="{ taken: taken.includes('right') }">
        <header class="box-head" :title="rightRef">{{ rightRef }}</header>
        <pre v-if="sides.right.length" class="code">{{ shown(sides.right) }}</pre>
        <p v-else class="empty">No lines</p>
        <p v-if="more(sides.right)" class="cut">{{ more(sides.right) }} more lines</p>
      </section>
    </div>
    <section v-if="result" class="box result">
      <header class="box-head">Result</header>
      <pre v-if="result.length" class="code">{{ shown(result) }}</pre>
      <p v-else class="empty">The block is removed</p>
      <p v-if="more(result)" class="cut">{{ more(result) }} more lines</p>
    </section>
    <AppButton v-if="longest > SHORT" variant="text" size="xs" class="toggle" @click="full = !full">
      {{ full ? 'Show less' : 'Show every line' }}
    </AppButton>
  </div>
</template>

<style scoped>
.block-sides {
  display: flex;
  flex-direction: column;
  gap: var(--space-2);
  min-width: 0;
}

.sides {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: var(--space-2);
}

.box {
  min-width: 0;
  border: 1px solid var(--color-border-subtle);
  border-radius: var(--radius-sm);
  background: var(--color-surface-0);
}

/* The side the result equals, outlined in the take color the three panes use. */
.box.taken {
  border-color: var(--color-jade);
}

.box.result {
  background: var(--color-surface-1);
}

.box-head {
  padding: 2px var(--space-2);
  border-bottom: 1px solid var(--color-border-subtle);
  font-size: var(--text-xs);
  color: var(--color-fg-secondary);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.code {
  margin: 0;
  padding: var(--space-1) var(--space-2);
  overflow-x: auto;
  font-family: var(--font-mono);
  font-size: var(--text-xs);
  line-height: 1.5;
  color: var(--color-fg-primary);
}

.empty,
.cut {
  margin: 0;
  padding: var(--space-1) var(--space-2);
  font-size: var(--text-xs);
  font-style: italic;
  color: var(--color-fg-muted);
}

.toggle {
  align-self: flex-start;
}
</style>
