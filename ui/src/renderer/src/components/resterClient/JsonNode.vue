<!-- SPDX-License-Identifier: AGPL-3.0-only -->
<!-- Copyright (C) 2026 The Lemongrass Authors -->

<script setup lang="ts">
import { computed, ref } from 'vue'

defineOptions({ name: 'JsonNode' })

const props = defineProps<{
  value: unknown
  name?: string
  depth: number
  last: boolean
}>()

// A large array would render thousands of rows at once, so children show in pages.
const PAGE = 200

const isArray = computed(() => Array.isArray(props.value))
const isBranch = computed(() => props.value !== null && typeof props.value === 'object')
const open = ref(props.depth < 2)
const shown = ref(PAGE)

const entries = computed<[string, unknown][]>(() => {
  if (!isBranch.value) return []
  return isArray.value
    ? (props.value as unknown[]).map((v, i) => [String(i), v])
    : Object.entries(props.value as Record<string, unknown>)
})

const leafClass = computed(() => {
  const v = props.value
  if (v === null) return 'null'
  return typeof v
})

const leafText = computed(() => JSON.stringify(props.value))
const opener = computed(() => (isArray.value ? '[' : '{'))
const closer = computed(() => (isArray.value ? ']' : '}'))
const comma = computed(() => (props.last ? '' : ','))
</script>

<template>
  <div class="json-node">
    <template v-if="isBranch">
      <button type="button" class="branch" @click="open = !open">
        <svg
          class="caret"
          :class="{ open }"
          width="8"
          height="8"
          viewBox="0 0 10 10"
          fill="none"
          stroke="currentColor"
          stroke-width="1.6"
          stroke-linecap="round"
          stroke-linejoin="round"
        >
          <path d="M3.5 2L6.5 5L3.5 8" />
        </svg>
        <template v-if="name !== undefined">
          <span class="key">"{{ name }}"</span><span class="punct">: </span>
        </template>
        <span class="punct">{{ opener }}</span>
        <template v-if="!open">
          <span class="summary">
            {{ entries.length }}
            {{
              isArray
                ? entries.length === 1
                  ? 'item'
                  : 'items'
                : entries.length === 1
                  ? 'key'
                  : 'keys'
            }}
          </span>
          <span class="punct">{{ closer }}{{ comma }}</span>
        </template>
      </button>
      <template v-if="open">
        <div class="children">
          <JsonNode
            v-for="([key, child], i) in entries.slice(0, shown)"
            :key="key"
            :name="isArray ? undefined : key"
            :value="child"
            :depth="depth + 1"
            :last="i === entries.length - 1"
          />
          <button v-if="entries.length > shown" type="button" class="more" @click="shown += PAGE">
            Show {{ Math.min(PAGE, entries.length - shown) }} more of {{ entries.length - shown }}
          </button>
        </div>
        <div class="punct closer">{{ closer }}{{ comma }}</div>
      </template>
    </template>
    <div v-else class="leaf">
      <template v-if="name !== undefined">
        <span class="key">"{{ name }}"</span><span class="punct">: </span>
      </template>
      <span :class="leafClass">{{ leafText }}</span
      ><span class="punct">{{ comma }}</span>
    </div>
  </div>
</template>

<style scoped>
.json-node {
  font-family: var(--font-mono);
  font-size: var(--text-xs);
  line-height: 1.6;
}

.branch {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  padding: 0;
  background: transparent;
  border: none;
  color: inherit;
  font: inherit;
  cursor: pointer;
  text-align: left;
}

.caret {
  flex-shrink: 0;
  margin-left: -12px;
  color: var(--color-fg-muted);
  transition: transform var(--duration-fast) var(--ease-out);
}

.caret.open {
  transform: rotate(90deg);
}

.children {
  padding-left: var(--space-4);
}

.leaf,
.closer {
  white-space: pre-wrap;
  word-break: break-all;
}

.key {
  color: var(--color-sapphire);
}

.punct {
  color: var(--color-fg-muted);
}

.summary {
  padding: 0 var(--space-1);
  border-radius: var(--radius-sm);
  background: var(--color-surface-3);
  color: var(--color-fg-secondary);
}

.string {
  color: var(--color-jade);
}

.number {
  color: var(--color-amber);
}

.boolean,
.null {
  color: var(--color-violet);
}

.more {
  padding: 0;
  background: transparent;
  border: none;
  color: var(--color-fg-accent);
  font: inherit;
  cursor: pointer;
}
</style>
