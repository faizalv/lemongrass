<!-- SPDX-License-Identifier: AGPL-3.0-only -->
<!-- Copyright (C) 2026 The Lemongrass Authors -->

<script setup lang="ts">
import { computed, inject, nextTick, ref, watch } from 'vue'
import { pathId, type PathSegment } from '../../responseSearch'
import { treeSearchKey } from './searchContext'

defineOptions({ name: 'JsonNode' })

const props = defineProps<{
  value: unknown
  name?: string
  depth: number
  last: boolean
  path: PathSegment[]
  // Set under a matched branch, whose contents the filter keeps whole.
  keepAll?: boolean
}>()

// A large array would render thousands of rows at once, so children show in pages.
const PAGE = 200

const isArray = computed(() => Array.isArray(props.value))
const isBranch = computed(() => props.value !== null && typeof props.value === 'object')
const search = inject(treeSearchKey, null)
const id = computed(() => pathId(props.path))
const match = computed(() => (search?.active ? search.byPath.get(id.value) : undefined))
const current = computed(() => !!search?.active && search.currentId === id.value)
const onMatchPath = computed(() => !!search?.active && search.ancestors.has(id.value))

const userOpen = ref(props.depth < 2)
// A click during a search wins over the search's own opening until the next search.
const override = ref<boolean | null>(null)
const open = computed(() => override.value ?? (onMatchPath.value || userOpen.value))
const shown = ref(PAGE)
const row = ref<HTMLElement | null>(null)

watch(
  () => search?.version,
  () => (override.value = null)
)

// Immediate, because the current match is often a node the search has just opened and mounted.
watch(
  current,
  async (isCurrent) => {
    if (!isCurrent) return
    await nextTick()
    row.value?.scrollIntoView({ block: 'center' })
  },
  { immediate: true }
)

function toggle(): void {
  if (search?.active) override.value = !open.value
  else userOpen.value = !userOpen.value
}

interface Child {
  key: string
  value: unknown
  path: PathSegment[]
  index: number
}

const entries = computed<Child[]>(() => {
  if (!isBranch.value) return []
  const pairs: [PathSegment, unknown][] = isArray.value
    ? (props.value as unknown[]).map((v, i) => [i, v])
    : Object.entries(props.value as Record<string, unknown>)
  return pairs.map(([segment, value], index) => ({
    key: String(segment),
    value,
    path: [...props.path, segment],
    index
  }))
})

function onPath(child: Child): boolean {
  const childId = pathId(child.path)
  return !!search && (search.ancestors.has(childId) || search.byPath.has(childId))
}

const keepChildren = computed(() => !!props.keepAll || !!match.value)

// The filter keeps only children on a match's path, and paging always reaches the last of those.
const visible = computed(() => {
  if (!search?.active) return entries.value.slice(0, shown.value)
  if (search.filter && !keepChildren.value)
    return entries.value.filter(onPath).slice(0, shown.value)
  let needed = 0
  for (const child of entries.value) if (onPath(child)) needed = child.index + 1
  return entries.value.slice(0, Math.max(shown.value, needed))
})

const hiddenCount = computed(() => {
  const total =
    search?.active && search.filter && !keepChildren.value
      ? entries.value.filter(onPath).length
      : entries.value.length
  return total - visible.value.length
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
      <button ref="row" type="button" class="branch" :class="{ current }" @click="toggle">
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
          <span class="key" :class="{ hit: match?.key }">"{{ name }}"</span
          ><span class="punct">: </span>
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
            v-for="child in visible"
            :key="child.key"
            :name="isArray ? undefined : child.key"
            :value="child.value"
            :depth="depth + 1"
            :last="child.index === entries.length - 1"
            :path="child.path"
            :keep-all="keepChildren"
          />
          <button v-if="hiddenCount > 0" type="button" class="more" @click="shown += PAGE">
            Show {{ Math.min(PAGE, hiddenCount) }} more of {{ hiddenCount }}
          </button>
        </div>
        <div class="punct closer">{{ closer }}{{ comma }}</div>
      </template>
    </template>
    <div v-else ref="row" class="leaf" :class="{ current }">
      <template v-if="name !== undefined">
        <span class="key" :class="{ hit: match?.key }">"{{ name }}"</span
        ><span class="punct">: </span>
      </template>
      <span :class="[leafClass, { hit: match?.value }]">{{ leafText }}</span
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
.hit {
  border-radius: var(--radius-sm);
  background: var(--color-amber-muted);
  box-shadow: 0 0 0 1px var(--color-amber-muted);
}

.current .hit {
  background: var(--color-amber);
  box-shadow: 0 0 0 1px var(--color-amber);
  color: var(--color-black);
}
</style>
