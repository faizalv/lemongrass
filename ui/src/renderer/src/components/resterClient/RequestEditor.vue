<!-- SPDX-License-Identifier: AGPL-3.0-only -->
<!-- Copyright (C) 2026 The Lemongrass Authors -->

<script setup lang="ts">
import { ref } from 'vue'
import AppButton from '../AppButton.vue'
import AppSegmented from '../AppSegmented.vue'
import PairRows from './PairRows.vue'
import type { ResterClientBodyKind, ResterClientRequest } from '../../../../preload/types'

// Edited in place; the parent owns the request.
const request = defineModel<ResterClientRequest>({ required: true })

const section = ref<'body' | 'headers'>('body')
const formatError = ref('')

const sections = [
  { value: 'body' as const, label: 'Body' },
  { value: 'headers' as const, label: 'Headers' }
]

const bodyKinds: { value: ResterClientBodyKind; label: string }[] = [
  { value: 'none', label: 'None' },
  { value: 'json', label: 'JSON' },
  { value: 'form', label: 'Form' }
]

function formatJson(): void {
  formatError.value = ''
  if (!request.value.json.trim()) return
  try {
    request.value.json = JSON.stringify(JSON.parse(request.value.json), null, 2)
  } catch (err) {
    formatError.value = err instanceof Error ? err.message : String(err)
  }
}
</script>

<template>
  <div class="editor">
    <AppSegmented v-model="section" :options="sections" aria-label="Request part" />

    <template v-if="section === 'body'">
      <AppSegmented v-model="request.bodyKind" :options="bodyKinds" aria-label="Body kind" />
      <p v-if="request.bodyKind === 'none'" class="hint">This request sends no body.</p>
      <template v-else-if="request.bodyKind === 'json'">
        <textarea
          v-model="request.json"
          class="json-input"
          spellcheck="false"
          placeholder="{ }"
          @input="formatError = ''"
        />
        <div class="json-actions">
          <AppButton @click="formatJson">Format</AppButton>
          <span v-if="formatError" class="error-text">{{ formatError }}</span>
        </div>
      </template>
      <PairRows v-else v-model="request.form" add-label="Field" name-placeholder="field" />
    </template>

    <template v-else>
      <p class="hint">
        The rester's own headers and the user's token are added when the request is sent.
      </p>
      <PairRows v-model="request.headers" add-label="Header" name-placeholder="header" />
    </template>
  </div>
</template>

<style scoped>
.editor {
  display: flex;
  flex-direction: column;
  gap: var(--space-3);
  min-height: 0;
  flex: 1;
}

.json-input {
  flex: 1;
  min-height: 220px;
  resize: none;
  font-family: var(--font-mono);
  font-size: var(--text-xs);
  line-height: 1.6;
  tab-size: 2;
}

.json-actions {
  display: flex;
  align-items: center;
  gap: var(--space-3);
}
</style>
