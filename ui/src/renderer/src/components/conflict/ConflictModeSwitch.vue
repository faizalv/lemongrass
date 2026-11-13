<!-- SPDX-License-Identifier: MPL-2.0 -->
<!-- Copyright (C) 2026 The Lemongrass Authors -->

<script setup lang="ts">
import { AppSegmented } from '@sankasten'
import type { ConflictMode } from '../../conflictVerdicts'

// The conflict tab's two views while a model holds the conflict: Decisions, what the human has to
// decide across every file, and Manual, the three panes on one file.
const props = defineProps<{ mode: ConflictMode }>()
const emit = defineEmits<{ switch: [mode: ConflictMode] }>()

const options: { value: ConflictMode; label: string }[] = [
  { value: 'decisions', label: 'Decisions' },
  { value: 'manual', label: 'Manual' }
]

function pick(mode: ConflictMode): void {
  if (mode !== props.mode) emit('switch', mode)
}
</script>

<template>
  <AppSegmented
    class="mode-switch"
    size="sm"
    :options="options"
    :model-value="mode"
    @update:model-value="pick"
  />
</template>

<style scoped>
.mode-switch {
  flex-shrink: 0;
}
</style>
