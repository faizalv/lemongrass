<!-- SPDX-License-Identifier: AGPL-3.0-only -->
<!-- Copyright (C) 2026 The Lemongrass Authors -->

<script setup lang="ts">
import { computed, ref } from 'vue'
import AppSegmented from '../AppSegmented.vue'
import JsonNode from './JsonNode.vue'
import { statusClass } from '../../resterClientCall'
import type { ResterClientDownload, ResterClientResult } from '../../../../preload/types'

const props = defineProps<{
  result: ResterClientResult | null
  download: ResterClientDownload | null
  error: string
  sending: boolean
  // Set when the response comes from history and its body was cut when it was stored.
  bodyCut: boolean
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

function formatSize(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`
  return `${(bytes / 1024 / 1024).toFixed(1)} MB`
}
</script>

<template>
  <section class="box">
    <header class="box-head">
      <span class="box-title">Response</span>
      <template v-if="!sending && !error && result">
        <span class="status" :class="statusClass(result.status)">{{ result.status }}</span>
        <span class="meta">{{ result.durationMs }} ms</span>
        <span class="meta">{{ formatSize(result.size) }}</span>
        <AppSegmented
          class="views"
          :model-value="activeView"
          :options="views"
          aria-label="Response view"
          @update:model-value="view = $event"
        />
      </template>
      <template v-else-if="!sending && !error && download?.kind === 'saved'">
        <span class="status" :class="statusClass(download.status)">{{ download.status }}</span>
        <span class="meta">{{ download.durationMs }} ms</span>
        <span class="meta">{{ formatSize(download.bytes) }}</span>
      </template>
    </header>

    <div class="box-body">
      <p v-if="sending" class="box-placeholder">Sending...</p>
      <p v-else-if="error" class="error-text">{{ error }}</p>
      <p v-else-if="download?.kind === 'saved'" class="box-placeholder">
        Saved to {{ download.path }}
      </p>
      <p v-else-if="download?.kind === 'cancelled'" class="box-placeholder">Download cancelled.</p>

      <template v-else-if="result">
        <p v-if="result.truncated" class="hint">
          Only the first {{ formatSize(result.size) }} came back inline. Use Send and download for
          the whole body.
        </p>
        <p v-if="bodyCut" class="hint">This saved response was cut when it went into history.</p>
        <template v-if="activeView === 'headers'">
          <div v-for="(row, i) in headerRows" :key="i" class="header-row">
            <span class="header-name">{{ row.name }}</span>
            <span class="header-value">{{ row.value }}</span>
          </div>
        </template>
        <p v-else-if="result.binary" class="box-placeholder">
          The body is binary. Use Send and download to save it.
        </p>
        <p v-else-if="!result.body" class="box-placeholder">The response has no body.</p>
        <div v-else-if="activeView === 'tree'" class="tree">
          <JsonNode :value="parsed.value" :depth="0" :last="true" />
        </div>
        <pre v-else class="raw">{{ rawText }}</pre>
      </template>

      <p v-else class="box-placeholder">Send the request to see its response here.</p>
    </div>
  </section>
</template>

<style scoped>
.meta {
  color: var(--color-fg-secondary);
  font-size: var(--text-xs);
}

.views {
  margin-left: auto;
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
