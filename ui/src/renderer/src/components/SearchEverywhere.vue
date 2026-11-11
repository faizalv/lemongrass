<!-- SPDX-License-Identifier: MPL-2.0 -->
<!-- Copyright (C) 2026 The Lemongrass Authors -->

<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { AppButton, AppChip, AppEmptyState, AppInput, AppSegmented } from '@sankasten'
import type { AppInputHandle } from '@sankasten'
import TabIcon from './TabIcon.vue'
import MatchName from './treeSearch/MatchName.vue'
import { fileIconOf } from '../fileIcons'
import { fuzzyMatch } from '../treeSearch'
import {
  biblioFilePaths,
  createDoubleShift,
  FILES_LIMIT,
  rankFiles,
  rowsFor,
  shortPath,
  type FileRow,
  type SearchRow,
  type SearchScope
} from '../searchEverywhere'
import { openSearchHit, type ProjectRef } from '../workspace'
import type { BiblioTree, ProjectTextHit } from '../../../preload/types'

const props = defineProps<{
  project?: ProjectRef
  biblioTree: BiblioTree | null
}>()

const FILE_DELAY_MS = 120
const TEXT_DELAY_MS = 150
const TEXT_MIN_QUERY = 2

const SCOPES: { value: SearchScope; label: string }[] = [
  { value: 'all', label: 'All' },
  { value: 'files', label: 'Files' },
  { value: 'text', label: 'Text' }
]

const open = ref(false)
const query = ref('')
const scope = ref<SearchScope>('all')
const files = ref<FileRow[]>([])
const textHits = ref<ProjectTextHit[]>([])
const textTruncated = ref(false)
const textBusy = ref(false)
const cursor = ref(0)
const input = ref<AppInputHandle | null>(null)
const list = ref<HTMLElement | null>(null)
const trigger = ref<HTMLElement | null>(null)

const rows = computed(() => rowsFor(scope.value, files.value, textHits.value))
const selected = computed<SearchRow | undefined>(() => rows.value[cursor.value])

const status = computed(() => {
  if (scope.value === 'files' || query.value.trim().length < TEXT_MIN_QUERY) return ''
  if (textBusy.value) return 'Searching...'
  return textTruncated.value ? `Top ${textHits.value.length} lines` : ''
})

let fileTimer: ReturnType<typeof setTimeout> | undefined
let textTimer: ReturnType<typeof setTimeout> | undefined
let fileSeq = 0
let textSeq = 0

// Each search kind keeps its own sequence, so a slower reply for an older query never replaces a
// newer one. Main also cancels an older text search outright.
async function searchFiles(q: string, project: ProjectRef): Promise<void> {
  const seq = ++fileSeq
  const result = await window.api.projectFiles.search(project.path, q)
  if (seq !== fileSeq) return
  const paths = result.paths.filter((hit) => hit.type === 'file').map((hit) => hit.path)
  files.value = rankFiles(
    [...paths, ...biblioFilePaths(props.biblioTree)],
    q,
    FILES_LIMIT,
    fuzzyMatch
  )
}

async function searchText(q: string, project: ProjectRef): Promise<void> {
  const seq = ++textSeq
  textBusy.value = true
  const result = await window.api.projectFiles.grep(project.path, q)
  if (seq !== textSeq || result.cancelled) return
  textBusy.value = false
  textHits.value = result.hits
  textTruncated.value = result.truncated
}

function schedule(): void {
  clearTimeout(fileTimer)
  clearTimeout(textTimer)
  cursor.value = 0
  const q = query.value
  const project = props.project
  if (!open.value || !project || !q.trim()) {
    fileSeq += 1
    textSeq += 1
    files.value = []
    textHits.value = []
    textTruncated.value = false
    textBusy.value = false
    return
  }
  fileTimer = setTimeout(() => void searchFiles(q, project), FILE_DELAY_MS)
  if (q.trim().length < TEXT_MIN_QUERY) {
    textSeq += 1
    textHits.value = []
    textTruncated.value = false
    textBusy.value = false
    return
  }
  textBusy.value = true
  textTimer = setTimeout(() => void searchText(q, project), TEXT_DELAY_MS)
}

watch([query, () => props.project?.path], schedule)
// A click on a scope takes the focus, so it goes back to the query for the arrow keys.
watch(scope, () => {
  cursor.value = 0
  void nextTick(() => input.value?.focus())
})

// Where the keyboard was before the search opened, such as a terminal, to return to on Escape.
let returnFocus: HTMLElement | null = null

async function show(): Promise<void> {
  if (!props.project) return
  if (!open.value) {
    const active = document.activeElement
    // Focus on the header field opened the search, so returning there would open it again.
    returnFocus = active instanceof HTMLElement && !trigger.value?.contains(active) ? active : null
    open.value = true
    schedule()
  }
  await nextTick()
  input.value?.focus()
  input.value?.select()
}

function hide(restoreFocus = false): void {
  open.value = false
  clearTimeout(fileTimer)
  clearTimeout(textTimer)
  if (restoreFocus) returnFocus?.focus()
  returnFocus = null
}

function activate(row: SearchRow | undefined, split: boolean): void {
  if (!row || !props.project) return
  openSearchHit(props.project, row.path, {
    line: row.kind === 'text' ? row.line : undefined,
    split
  })
  hide()
}

async function moveCursor(step: number): Promise<void> {
  const count = rows.value.length
  if (count === 0) return
  cursor.value = (cursor.value + step + count) % count
  await nextTick()
  list.value?.querySelector(`[data-row="${cursor.value}"]`)?.scrollIntoView({ block: 'nearest' })
}

function cycleScope(step: number): void {
  const at = SCOPES.findIndex((option) => option.value === scope.value)
  scope.value = SCOPES[(at + step + SCOPES.length) % SCOPES.length].value
}

function onKeydown(event: KeyboardEvent): void {
  if (event.key === 'ArrowDown' || event.key === 'ArrowUp') {
    event.preventDefault()
    void moveCursor(event.key === 'ArrowDown' ? 1 : -1)
  } else if (event.key === 'Enter') {
    event.preventDefault()
    activate(selected.value, event.shiftKey)
  } else if (event.key === 'Tab') {
    event.preventDefault()
    cycleScope(event.shiftKey ? -1 : 1)
  } else if (event.key === 'Escape') {
    event.preventDefault()
    hide(true)
  }
}

function rowTitle(row: SearchRow): string {
  return row.kind === 'text' ? `${row.path}:${row.line}` : row.path
}

// Listened for in the capture phase on the window, so a focused terminal or editor never swallows
// the Shift presses first.
const doubleShift = createDoubleShift(() => void show())
const onWindowKeydown = (event: KeyboardEvent): void => doubleShift.keydown(event)
const onWindowKeyup = (event: KeyboardEvent): void => doubleShift.keyup(event)
const onWindowBlur = (): void => doubleShift.reset()

onMounted(() => {
  window.addEventListener('keydown', onWindowKeydown, true)
  window.addEventListener('keyup', onWindowKeyup, true)
  window.addEventListener('blur', onWindowBlur)
})

onBeforeUnmount(() => {
  clearTimeout(fileTimer)
  clearTimeout(textTimer)
  window.removeEventListener('keydown', onWindowKeydown, true)
  window.removeEventListener('keyup', onWindowKeyup, true)
  window.removeEventListener('blur', onWindowBlur)
})
</script>

<template>
  <div v-if="project" ref="trigger" class="search-trigger" @mousedown.prevent="show">
    <AppInput
      :model-value="''"
      class="trigger-field"
      icon="search"
      size="sm"
      readonly
      placeholder="Search files and code"
      aria-label="Search everywhere"
      @focus="show"
    />
    <AppChip class="trigger-hint" tone="quiet" title="Press Shift twice">Shift Shift</AppChip>
  </div>

  <Teleport to="body">
    <template v-if="open">
      <div class="search-backdrop" @mousedown="hide(true)" />
      <div class="search-panel" role="dialog" aria-label="Search everywhere">
        <div class="panel-head">
          <AppSegmented v-model="scope" :options="SCOPES" size="sm" aria-label="Search scope" />
          <span class="panel-status">{{ status }}</span>
        </div>
        <AppInput
          ref="input"
          v-model="query"
          class="panel-query"
          icon="search"
          placeholder="Type a file name or a piece of code"
          spellcheck="false"
          @keydown="onKeydown"
        />
        <div ref="list" class="panel-rows">
          <div
            v-for="(row, index) in rows"
            :key="row.kind === 'text' ? `t:${row.path}:${row.line}` : `f:${row.path}`"
            class="row"
            :class="[row.kind, { selected: index === cursor }]"
            :data-row="index"
            :title="rowTitle(row)"
            @mousemove="cursor = index"
            @click="activate(row, false)"
          >
            <TabIcon
              class="row-icon"
              :shapes="fileIconOf(row.path).shapes"
              :style="{ color: fileIconOf(row.path).color }"
            />
            <template v-if="row.kind === 'file'">
              <MatchName class="row-name" :text="row.name" :ranges="row.nameRanges" />
              <span class="row-dir">{{ row.dir }}</span>
            </template>
            <template v-else>
              <MatchName class="row-code" :text="row.text" :ranges="row.ranges" />
              <span class="row-where">{{ shortPath(row.path) }} {{ row.line }}</span>
            </template>
          </div>
          <AppEmptyState
            v-if="rows.length === 0"
            size="sm"
            :title="
              query.trim()
                ? textBusy
                  ? 'Searching...'
                  : 'No matches.'
                : 'Type to search file names and code.'
            "
            :detail="query.trim() ? '' : 'Tab switches between All, Files and Text.'"
          />
        </div>
        <div class="panel-foot">
          <span class="foot-path">{{ selected ? rowTitle(selected) : '' }}</span>
          <AppButton
            variant="text"
            size="xs"
            :disabled="!selected"
            title="Shift+Enter"
            @click="activate(selected, true)"
          >
            Open in right split
          </AppButton>
        </div>
      </div>
    </template>
  </Teleport>
</template>

<style scoped>
/* Centered on the header and kept out of its window drag region, which would swallow clicks. */
.search-trigger {
  position: absolute;
  top: 50%;
  left: 50%;
  transform: translate(-50%, -50%);
  width: clamp(200px, calc(100% - 820px), 520px);
  display: flex;
  align-items: center;
  cursor: pointer;
  -webkit-app-region: no-drag;
}

.trigger-field {
  flex: 1;
}

/* Too narrow to sit between the title and the window controls; double Shift still opens it. */
@media (max-width: 960px) {
  .search-trigger {
    display: none;
  }
}

.trigger-hint {
  position: absolute;
  right: var(--space-2);
  pointer-events: none;
  font-size: var(--text-xs);
}

.search-backdrop {
  position: fixed;
  inset: 0;
  z-index: 59;
}

.search-panel {
  position: fixed;
  top: 6px;
  left: 50%;
  transform: translateX(-50%);
  z-index: 60;
  width: min(860px, calc(100vw - 48px));
  max-height: min(720px, calc(100vh - 24px));
  display: flex;
  flex-direction: column;
  gap: var(--space-2);
  padding: var(--space-3);
  background: var(--layer-float);
  border: 1px solid var(--color-border-default);
  border-radius: var(--radius-lg);
  box-shadow: var(--shadow-lg);
  -webkit-app-region: no-drag;
}

.panel-head {
  display: flex;
  align-items: center;
  gap: var(--space-3);
}

.panel-status {
  margin-left: auto;
  color: var(--color-fg-muted);
  font-size: var(--text-xs);
}

.panel-rows {
  flex: 1;
  min-height: 0;
  overflow-y: auto;
}

.row {
  display: flex;
  align-items: center;
  gap: var(--space-2);
  min-width: 0;
  padding: 4px var(--space-2);
  border-radius: var(--radius-md);
  color: var(--color-fg-secondary);
  font-size: var(--text-sm);
  cursor: pointer;
}

.row.selected {
  background: var(--color-amber-muted);
  color: var(--color-fg-primary);
}

.row-icon {
  flex-shrink: 0;
}

.row-name {
  flex-shrink: 0;
  max-width: 60%;
  color: var(--color-fg-primary);
}

.row-dir {
  min-width: 0;
  overflow: hidden;
  white-space: nowrap;
  text-overflow: ellipsis;
  color: var(--color-fg-muted);
}

.row-code {
  flex: 1;
  font-family: var(--font-mono);
  font-size: var(--text-xs);
}

.row-where {
  flex-shrink: 0;
  max-width: 40%;
  overflow: hidden;
  white-space: nowrap;
  text-overflow: ellipsis;
  color: var(--color-fg-muted);
  font-size: var(--text-xs);
}

.panel-foot {
  display: flex;
  align-items: center;
  gap: var(--space-3);
  padding-top: var(--space-2);
  border-top: 1px solid var(--color-border-subtle);
}

.foot-path {
  flex: 1;
  min-width: 0;
  overflow: hidden;
  white-space: nowrap;
  text-overflow: ellipsis;
  color: var(--color-fg-muted);
  font-size: var(--text-xs);
}
</style>
