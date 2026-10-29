<!-- SPDX-License-Identifier: AGPL-3.0-only -->
<!-- Copyright (C) 2026 The Lemongrass Authors -->

<script setup lang="ts">
import { computed, ref } from 'vue'
import AppSegmented from '../AppSegmented.vue'
import JsonNode from './JsonNode.vue'
import type { ResterClientDownload, ResterClientResult } from '../../../../preload/types'

const props = defineProps<{
  result: ResterClientResult | null
  download: ResterClientDownload | null
  error: string
  sending: boolean
}>()

const view = ref<'tree' | 'raw' | 'headers'>('tree')

const parsed = computed<{ ok: boolean; value: unknown }>(() => {
  const body = props.result?.body ?? ''
  if (!body.trim()) return { ok: false, value: null }
  try {
    return { ok: true, value: JSON.parse(body) }
  } catch {
    return { ok: false, value: null }
  }
})

const views = computed(() =>
  parsed.value.ok
    ? [
        { value: 'tree' as const, label: 'Tree' },
        { value: 'raw' as const, label: 'Raw' },
        { value: 'headers' as const, label: 'Headers' }
      ]
    : [
        { value: 'raw' as const, label: 'Body' },
        { value: 'headers' as const, label: 'Headers' }
      ]
)

const activeView = computed(() => (view.value === 'tree' && !parsed.value.ok ? 'raw' : view.value))

const rawText = computed(() =>
  parsed.value.ok ? JSON.stringify(parsed.value.value, null, 2) : (props.result?.body ?? '')
)

const headerRows = computed(() =>
  Object.entries(props.result?.headers ?? {}).flatMap(([name, values]) =>
    values.map((value) => ({ name, value }))
  )
)

function statusClass(status: number): string {
  if (status >= 200 && status < 300) return 'ok'
  if (status >= 300 && status < 400) return 'redirect'
  if (status >= 400 && status < 500) return 'client'
  return 'server'
}

function formatSize(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`
  return `${(bytes / 1024 / 1024).toFixed(1)} MB`
}
</script>

<template>
  <div class="response">
    <p v-if="sending" class="hint">Sending...</p>
    <p v-else-if="error" class="error-text">{{ error }}</p>

    <template v-else-if="download && download.kind === 'saved'">
      <div class="meta">
        <span class="status" :class="statusClass(download.status)">{{ download.status }}</span>
        <span>{{ download.durationMs }} ms</span>
        <span>{{ formatSize(download.bytes) }}</span>
      </div>
      <p class="hint">Saved to {{ download.path }}</p>
    </template>
    <p v-else-if="download && download.kind === 'cancelled'" class="hint">Download cancelled.</p>

    <template v-else-if="result">
      <div class="meta">
        <span class="status" :class="statusClass(result.status)">{{ result.status }}</span>
        <span>{{ result.durationMs }} ms</span>
        <span>{{ formatSize(result.size) }}</span>
      </div>
      <p v-if="result.truncated" class="hint">
        Only the first {{ formatSize(result.size) }} is shown. Use Send and download for the whole
        body.
      </p>
      <AppSegmented
        :model-value="activeView"
        :options="views"
        aria-label="Response view"
        @update:model-value="view = $event"
      />
      <div class="body">
        <template v-if="activeView === 'headers'">
          <div v-for="(row, i) in headerRows" :key="i" class="header-row">
            <span class="header-name">{{ row.name }}</span>
            <span class="header-value">{{ row.value }}</span>
          </div>
        </template>
        <p v-else-if="result.binary" class="hint">
          The body is binary. Use Send and download to save it.
        </p>
        <p v-else-if="!result.body" class="hint">The response has no body.</p>
        <div v-else-if="activeView === 'tree'" class="tree">
          <JsonNode :value="parsed.value" :depth="0" :last="true" />
        </div>
        <pre v-else class="raw">{{ rawText }}</pre>
      </div>
    </template>

    <p v-else class="hint">Send a request to see its response here.</p>
  </div>
</template>

<style scoped>
.response {
  display: flex;
  flex-direction: column;
  gap: var(--space-3);
  min-height: 0;
  flex: 1;
}

.meta {
  display: flex;
  align-items: center;
  gap: var(--space-3);
  color: var(--color-fg-secondary);
  font-size: var(--text-xs);
}

.status {
  padding: 2px var(--space-2);
  border-radius: var(--radius-pill);
  font-weight: var(--weight-semibold);
}

.status.ok {
  background: var(--color-success-muted);
  color: var(--color-success);
}

.status.redirect {
  background: var(--color-sapphire-muted);
  color: var(--color-sapphire);
}

.status.client {
  background: var(--color-amber-muted);
  color: var(--color-amber);
}

.status.server {
  background: var(--color-error-muted);
  color: var(--color-error);
}

.body {
  flex: 1;
  min-height: 0;
  overflow: auto;
  padding: var(--space-3);
  background: var(--color-surface-0);
  border: 1px solid var(--color-border-default);
  border-radius: var(--radius-md);
}

.tree {
  padding-left: var(--space-3);
}

.raw {
  margin: 0;
  font-family: var(--font-mono);
  font-size: var(--text-xs);
  line-height: 1.6;
  white-space: pre-wrap;
  word-break: break-all;
  color: var(--color-fg-primary);
}

.header-row {
  display: grid;
  grid-template-columns: minmax(120px, 1fr) minmax(0, 2fr);
  gap: var(--space-3);
  padding: 2px 0;
  font-family: var(--font-mono);
  font-size: var(--text-xs);
}

.header-name {
  color: var(--color-sapphire);
}

.header-value {
  color: var(--color-fg-primary);
  word-break: break-all;
}
</style>
