<!-- SPDX-License-Identifier: AGPL-3.0-only -->
<!-- Copyright (C) 2026 The Lemongrass Authors -->

<script setup lang="ts">
import AppButton from '../AppButton.vue'
import AppInput from '../AppInput.vue'
import AppSegmented from '../AppSegmented.vue'
import FilePick from './FilePick.vue'
import type { ResterClientPart, ResterClientPickedFile } from '../../../../preload/types'

// Edited in place; the parent owns the array.
const parts = defineModel<ResterClientPart[]>({ required: true })

const kinds = [
  { value: 'text' as const, label: 'Text' },
  { value: 'file' as const, label: 'File' }
]

function add(): void {
  parts.value.push({ name: '', kind: 'text', value: '', path: '', size: 0, contentType: '' })
}

function picked(part: ResterClientPart, file: ResterClientPickedFile): void {
  part.path = file.path
  part.size = file.size
}
</script>

<template>
  <div class="parts">
    <p class="hint">Sent as multipart form data: text fields first, then files in row order.</p>
    <div v-for="(part, i) in parts" :key="i" class="part">
      <AppInput v-model="part.name" type="text" placeholder="field" spellcheck="false" />
      <AppSegmented v-model="part.kind" :options="kinds" aria-label="Field kind" />
      <AppInput
        v-if="part.kind === 'text'"
        v-model="part.value"
        class="value"
        type="text"
        placeholder="value"
        spellcheck="false"
      />
      <div v-else class="value file">
        <FilePick :path="part.path" :size="part.size" @picked="picked(part, $event)" />
        <AppInput
          v-model="part.contentType"
          class="type"
          type="text"
          placeholder="type from extension"
          spellcheck="false"
        />
      </div>
      <button type="button" class="remove" title="Remove" @click="parts.splice(i, 1)">×</button>
    </div>
    <AppButton class="add" @click="add">+ Field</AppButton>
  </div>
</template>

<style scoped>
.parts {
  display: flex;
  flex-direction: column;
  gap: var(--space-2);
}

.part {
  display: grid;
  grid-template-columns: minmax(0, 2fr) auto minmax(0, 4fr) auto;
  gap: var(--space-2);
  align-items: center;
}

.file {
  display: grid;
  grid-template-columns: minmax(0, 1fr) minmax(0, 150px);
  gap: var(--space-2);
  align-items: center;
}

.remove {
  width: 24px;
  height: 24px;
  padding: 0;
  background: transparent;
  border: none;
  border-radius: var(--radius-sm);
  color: var(--color-fg-muted);
  font-size: var(--text-md);
  cursor: pointer;
}

.remove:hover {
  background: var(--color-surface-3);
  color: var(--color-fg-primary);
}

.add {
  align-self: flex-start;
}
</style>
