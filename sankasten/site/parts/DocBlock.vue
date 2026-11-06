<!-- SPDX-License-Identifier: Apache-2.0 -->
<!-- Copyright (C) 2026 The Sankasten Authors -->

<script setup lang="ts">
import { DOCS, type PropDoc } from '../docs'

// One component's reference: its summary, the live demo in the slot, then its API tables.
const props = defineProps<{ name: string }>()

const doc = DOCS[props.name]

const tables: { title: string; key: 'props' | 'events' | 'slots' }[] = [
  { title: 'Props', key: 'props' },
  { title: 'Events', key: 'events' },
  { title: 'Slots', key: 'slots' }
]

function rows(key: 'props' | 'events' | 'slots'): PropDoc[] {
  return doc[key] ?? []
}
</script>

<template>
  <article :id="name" class="doc-block">
    <h3 class="doc-name">{{ name }}</h3>
    <p class="doc-summary">{{ doc.summary }}</p>
    <div class="doc-demo"><slot /></div>
    <template v-for="table in tables" :key="table.key">
      <table v-if="rows(table.key).length" class="doc-table">
        <thead>
          <tr>
            <th>{{ table.title }}</th>
            <th>Type</th>
            <th>Default</th>
            <th>Use</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="row in rows(table.key)" :key="row.name">
            <td>
              <code>{{ row.name }}</code>
            </td>
            <td class="mono">{{ row.type }}</td>
            <td class="mono">{{ row.default ?? '' }}</td>
            <td>{{ row.doc }}</td>
          </tr>
        </tbody>
      </table>
    </template>
    <p v-for="note in doc.notes ?? []" :key="note" class="doc-note">{{ note }}</p>
  </article>
</template>

<style scoped>
.doc-block {
  padding: var(--space-6);
  background: var(--layer-card);
  border: 1px solid var(--color-border-subtle);
  border-radius: var(--radius-lg);
  scroll-margin-top: var(--space-6);
}

.doc-name {
  font-size: var(--text-lg);
}

.doc-summary {
  margin-top: var(--space-1);
  color: var(--color-fg-secondary);
}

.doc-demo {
  display: flex;
  flex-direction: column;
  gap: var(--space-4);
  margin: var(--space-4) 0;
  padding: var(--space-5);
  background: var(--layer-page);
  border: 1px solid var(--color-border-subtle);
  border-radius: var(--radius-md);
}

.doc-table {
  width: 100%;
  margin-top: var(--space-3);
  border-collapse: collapse;
  font-size: var(--text-xs);
}

.doc-table th {
  padding: var(--space-1) var(--space-2);
  color: var(--color-fg-muted);
  font-weight: var(--weight-semibold);
  text-align: left;
  border-bottom: 1px solid var(--color-border-default);
}

.doc-table td {
  padding: var(--space-2);
  vertical-align: top;
  color: var(--color-fg-secondary);
  border-bottom: 1px solid var(--color-border-subtle);
}

.doc-table td:first-child {
  white-space: nowrap;
}

.mono {
  font-family: var(--font-mono);
  color: var(--color-fg-muted);
}

.doc-note {
  margin-top: var(--space-3);
  color: var(--color-fg-muted);
  font-size: var(--text-xs);
}
</style>
