<!-- SPDX-License-Identifier: MPL-2.0 -->
<!-- Copyright (C) 2026 The Lemongrass Authors -->

<script setup lang="ts">
import { computed, watch } from 'vue'
import { AppBadge, AppEmptyState, AppSegmented } from '@sankasten'
import type { ConflictThreadState } from '../../conflictThread'
import { useFileDecisions } from '../../conflictFiles'
import {
  countStates,
  generalLine,
  handOverAsked,
  isQueued,
  listLabel,
  listStateOf,
  manualDigest,
  nextStep,
  roundDone,
  type BlockCard,
  type FileDecisions,
  type ListState
} from '../../conflictDecisions'
import {
  clearReasoningFlash,
  draftsOf,
  focusBlock,
  listViewOf,
  manualOf,
  pendingFor,
  reasoningFlashOf,
  resolveByHand,
  setDraft,
  setListView
} from '../../conflictVerdicts'
import { focusChat } from '../../conflictChatStore'
import { commentDraftsOf } from '../../conflictCommentStore'
import type { ChatScope } from '../../conflictChat'
import { tabParts, type NamePart } from '../../tabNames'
import type { ProjectRef } from '../../workspace'
import ConflictDock from './ConflictDock.vue'
import ConflictModeSwitch from './ConflictModeSwitch.vue'
import DecisionWorkspace from './DecisionWorkspace.vue'
import MarkedText from '../MarkedText.vue'

// Decisions: what the human has to decide, across every conflicted file, as a list and a
// workspace. The list has a row per block with its state, the workspace shows the selected block
// large with its one primary action, and the top says whose move it is and holds the one button
// that sends the human's decisions. The conversation itself is never shown: the model's latest
// plain message is one line under the top, and the human answers it in one line.
const props = defineProps<{
  project: ProjectRef
  thread: ConflictThreadState
  files: string[]
}>()

// manual opens a file in Manual, or the file the tab last showed when path is null.
const emit = defineEmits<{ manual: [path: string | null] }>()

const decisions = useFileDecisions(() => props.project)
const drafts = computed(() => draftsOf(props.project.id))

interface Row {
  card: BlockCard
  file: FileDecisions
  state: ListState
  label: string
}

const finished = computed(() => roundDone(props.thread.messages))
const asked = computed(() => handOverAsked(props.thread.messages))

const rows = computed((): Row[] =>
  decisions.value.flatMap((file) =>
    (file.cards ?? []).map((card) => {
      const queued = isQueued(card, drafts.value[card.key], manualOf(props.project.id, card.path))
      const state = listStateOf(card, queued, asked.value, finished.value)
      return { card, file, state, label: listLabel(state, card) }
    })
  )
)

const counts = computed(() => countStates(rows.value.map((row) => row.state)))

// "Needs you" keeps the decisions already made in view until they are sent, so one can be undone.
// The choice, the selection and the width live in the module's per-project state, not here, since
// this view is unmounted whenever the human switches project.
const listView = computed(() => listViewOf(props.project.id))
const filter = computed(
  () => listView.value.filter ?? (counts.value.needs + counts.value.queued > 0 ? 'needs' : 'all')
)
const filterOptions: { value: 'needs' | 'all'; label: string }[] = [
  { value: 'needs', label: 'Needs you' },
  { value: 'all', label: 'All' }
]

const shown = computed(() =>
  filter.value === 'all'
    ? rows.value
    : rows.value.filter((row) => row.state === 'needs' || row.state === 'queued')
)

interface Group {
  path: string
  name: string
  dir: string
  rows: Row[]
}

const groups = computed((): Group[] => {
  const out: Group[] = []
  for (const row of shown.value) {
    let group = out.find((g) => g.path === row.card.path)
    if (!group) {
      const cut = row.card.path.lastIndexOf('/')
      group = {
        path: row.card.path,
        name: row.card.path.slice(cut + 1),
        dir: cut === -1 ? '' : row.card.path.slice(0, cut),
        rows: []
      }
      out.push(group)
    }
    group.rows.push(row)
  }
  return out
})

// The picked block while it is listed, else the first block that needs the human, else the first.
const current = computed(
  (): Row | null =>
    shown.value.find((row) => row.card.key === listView.value.selected) ??
    shown.value.find((row) => row.state === 'needs') ??
    shown.value[0] ??
    null
)

// Coming back from the label on an agent's proposal in Manual: the block must be listed to be
// selected, so a filter that hides it gives way.
const reasoningFlash = computed(() => reasoningFlashOf(props.project.id))
watch(
  reasoningFlash,
  (flash) => {
    if (flash && !shown.value.some((row) => row.card.key === flash.key)) {
      setListView(props.project.id, { filter: 'all' })
    }
  },
  { immediate: true }
)

const TONES: Record<ListState, 'accent' | 'info' | 'success' | 'danger' | 'neutral'> = {
  needs: 'accent',
  queued: 'success',
  model: 'info',
  untouched: 'neutral',
  done: 'success'
}

function blocksText(card: BlockCard): string {
  const blocks = card.blocks
  return blocks.length === 1 ? `Block ${blocks[0]}` : `Blocks ${blocks[0]} to ${blocks.at(-1)}`
}

function select(key: string): void {
  setListView(props.project.id, { selected: key })
}

// Up and Down walk the list, as in any list that has a selection.
function onListKey(event: KeyboardEvent): void {
  if (event.key !== 'ArrowDown' && event.key !== 'ArrowUp') return
  const list = shown.value
  if (list.length === 0) return
  event.preventDefault()
  const at = list.findIndex((row) => row.card.key === current.value?.card.key)
  const next = Math.min(list.length - 1, Math.max(0, at + (event.key === 'ArrowDown' ? 1 : -1)))
  select(list[next].card.key)
}

// After a decision the selection moves to the next block that still needs the human, after the
// current one first and then from the top.
function advance(): void {
  const list = rows.value
  const at = list.findIndex((row) => row.card.key === current.value?.card.key)
  const after = [...list.slice(at + 1), ...list.slice(0, at)]
  const next = after.find((row) => row.state === 'needs')
  if (next) select(next.card.key)
}

const MIN_LIST = 200
const MAX_LIST = 560

// Dragging the handle resizes the list; the pointer is captured so a fast drag over the workspace
// or a code box still follows.
function startResize(event: PointerEvent): void {
  const handle = event.currentTarget as HTMLElement
  const from = event.clientX
  const width = listView.value.width
  handle.setPointerCapture(event.pointerId)
  const move = (e: PointerEvent): void => {
    const next = Math.min(MAX_LIST, Math.max(MIN_LIST, width + e.clientX - from))
    setListView(props.project.id, { width: next })
  }
  const stop = (): void => {
    handle.removeEventListener('pointermove', move)
    handle.removeEventListener('pointerup', stop)
    handle.removeEventListener('pointercancel', stop)
  }
  handle.addEventListener('pointermove', move)
  handle.addEventListener('pointerup', stop)
  handle.addEventListener('pointercancel', stop)
}

// Who holds the conflict, highlighted since its title is the model's own words.
const holder = computed((): NamePart[] => {
  const tabId = props.thread.status?.assignee
  return tabId ? tabParts(tabId) : [{ text: 'an agent' }]
})
const statusParts = computed((): NamePart[] => {
  const step = ` ${nextStep(counts.value)}`
  return finished.value
    ? [...holder.value, { text: ` finished its round.${step}` }]
    : [{ text: 'Handed to ' }, ...holder.value, { text: `.${step}` }]
})

const line = computed(() => generalLine(props.thread.messages))
function byHand(card: BlockCard): void {
  resolveByHand(props.project.id, card)
  emit('manual', card.path)
}

function showInManual(card: BlockCard): void {
  focusBlock(props.project.id, card.path, card.blocks[0])
  emit('manual', card.path)
}

// What the chat box is about: the selected block, or its file when nothing is selected.
const chatScope = computed((): ChatScope | null => {
  if (current.value) return { path: current.value.card.path, block: current.value.card.blocks[0] }
  return props.files[0] ? { path: props.files[0], block: null } : null
})

// A comment queued on the selected block's lines is the question to the agent, so Send asks it.
const commented = computed(() => {
  const card = current.value?.card
  if (!card) return false
  return commentDraftsOf(props.project.id).some(
    (draft) => draft.path === card.path && card.blocks.includes(draft.block)
  )
})

// Files with something to send, and what sending would do with each.
const ready = computed(() =>
  decisions.value.flatMap((file) => {
    if (!file.cards) return []
    // Read so this recomputes when a draft or a hand resolution changes.
    void drafts.value
    void manualOf(props.project.id, file.path)
    const pending = pendingFor(props.project.id, file)
    if (pending.verdicts.length === 0 && !pending.settled) return []
    return [{ file, ...pending }]
  })
)

const noBlocks = computed(() => decisions.value.filter((file) => file.cards === null))
// Lines changed outside every block are staged with the file but no block report covers them.
const outside = computed(() => decisions.value.filter((file) => file.editsOutside > 0))

function fileName(path: string): string {
  return path.slice(path.lastIndexOf('/') + 1)
}

function summary(verdicts: { verdict: string; block: number }[], settled: boolean): string {
  const counted = new Map<string, Set<number>>()
  for (const v of verdicts) {
    if (!counted.has(v.verdict)) counted.set(v.verdict, new Set())
    counted.get(v.verdict)!.add(v.block)
  }
  const words: Record<string, string> = {
    accepted: 'accepted',
    rejected: 'rejected',
    resolved: 'resolved by you',
    answered: 'answered'
  }
  const parts = [...counted].map(([kind, blocks]) => `${blocks.size} ${words[kind] ?? kind}`)
  if (settled) parts.push('stages the file')
  return parts.join(', ')
}

const sendTitle = computed(() =>
  ready.value.length === 0
    ? 'Nothing is decided yet.'
    : ready.value
        .map((item) => `${fileName(item.file.path)}: ${summary(item.verdicts, item.settled)}`)
        .join('\n')
)
</script>

<template>
  <div class="decisions">
    <header class="top">
      <ConflictModeSwitch mode="decisions" @switch="emit('manual', null)" />
      <MarkedText class="status" :parts="statusParts" capitalize />
    </header>

    <p v-if="line" class="general" :title="line.body">
      <span class="general-author">Agent</span>
      {{ line.body }}
    </p>

    <div v-if="rows.length === 0 && noBlocks.length === 0" class="idle">
      <AppEmptyState
        layout="fill"
        :loading="decisions.length < files.length"
        title="Nothing to decide yet."
        detail="Each block shows here as soon as there is one."
      />
      <footer class="idle-row">
        <ConflictDock
          :project="project"
          :scope="chatScope"
          :ready="ready"
          :queued-blocks="counts.queued"
          inline
          :title="sendTitle"
        />
      </footer>
    </div>
    <div v-else class="layout">
      <aside class="list" :style="{ width: `${listView.width}px` }">
        <header class="list-head">
          <span class="list-title">Blocks</span>
          <AppSegmented
            size="sm"
            aria-label="Which blocks to list"
            :options="filterOptions"
            :model-value="filter"
            @update:model-value="setListView(project.id, { filter: $event })"
          />
        </header>

        <div class="list-body" tabindex="0" role="listbox" @keydown="onListKey">
          <p v-if="groups.length === 0" class="list-empty">
            Nothing needs you. Switch to All to see every block.
          </p>
          <section v-for="group in groups" :key="group.path" class="group">
            <h3 class="group-name" :title="group.path">
              <span class="group-file">{{ group.name }}</span>
              <span v-if="group.dir" class="group-dir">{{ group.dir }}</span>
            </h3>
            <button
              v-for="row in group.rows"
              :key="row.card.key"
              type="button"
              class="row"
              :class="[row.state, { selected: row.card.key === current?.card.key }]"
              role="option"
              :aria-selected="row.card.key === current?.card.key"
              @click="select(row.card.key)"
            >
              <span class="dot" />
              <span class="row-name">{{ blocksText(row.card) }}</span>
              <AppBadge size="sm" :tone="TONES[row.state]">{{ row.label }}</AppBadge>
            </button>
          </section>

          <section v-if="noBlocks.length || outside.length" class="group">
            <h3 class="group-name">Other changes</h3>
            <button
              v-for="file in noBlocks"
              :key="`n-${file.path}`"
              type="button"
              class="other"
              :title="`${file.path}: binary, or deleted on one side. Pick a side in Manual.`"
              @click="emit('manual', file.path)"
            >
              <span class="group-file">{{ fileName(file.path) }}</span>
              <span class="other-text">Pick a side in Manual</span>
            </button>
            <button
              v-for="file in outside"
              :key="`o-${file.path}`"
              type="button"
              class="other"
              :title="`${file.path}: ${file.editsOutside} line(s) changed outside the blocks. Send stages them with the file.`"
              @click="emit('manual', file.path)"
            >
              <span class="group-file">{{ fileName(file.path) }}</span>
              <span class="other-text">
                {{ file.editsOutside === 1 ? '1 line' : `${file.editsOutside} lines` }} outside the
                blocks
              </span>
            </button>
          </section>
        </div>
      </aside>

      <div
        class="resize"
        role="separator"
        aria-orientation="vertical"
        title="Drag to resize the list"
        @pointerdown="startResize"
      />
      <DecisionWorkspace
        v-if="current"
        :key="current.card.key"
        :project="project"
        :card="current.card"
        :list-state="current.state"
        :left-ref="current.file.leftRef"
        :right-ref="current.file.rightRef"
        :draft="drafts[current.card.key]"
        :manual-digest="manualDigest(current.card, manualOf(project.id, current.card.path))"
        :flash="reasoningFlash?.key === current.card.key ? reasoningFlash.nonce : 0"
        :commented="commented"
        @flashed="clearReasoningFlash(project.id)"
        @draft="setDraft(project.id, current.card.key, $event)"
        @by-hand="byHand(current.card)"
        @manual="showInManual(current.card)"
        @ask="focusChat(project.id)"
        @decided="advance"
      >
        <template #end>
          <ConflictDock
            :project="project"
            :scope="chatScope"
            :ready="ready"
            :queued-blocks="counts.queued"
            inline
            :title="sendTitle"
          />
        </template>
      </DecisionWorkspace>
      <div v-else class="idle">
        <AppEmptyState
          layout="fill"
          title="Pick a block."
          detail="The blocks that need you are listed on the left."
        />
        <footer class="idle-row">
          <ConflictDock
            :project="project"
            :scope="chatScope"
            :ready="ready"
            :queued-blocks="counts.queued"
            inline
            :title="sendTitle"
          />
        </footer>
      </div>
    </div>
  </div>
</template>

<style scoped>
.decisions {
  position: relative;
  display: flex;
  flex-direction: column;
  height: 100%;
  min-height: 0;
  background: var(--color-surface-0);
}

.top {
  display: flex;
  align-items: center;
  gap: var(--space-3);
  min-width: 0;
  padding: var(--space-2) var(--space-3);
  border-bottom: 1px solid var(--color-border-subtle);
}

.status {
  flex: 1;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-size: var(--text-sm);
  color: var(--color-fg-secondary);
}

.general {
  margin: 0;
  padding: var(--space-2) var(--space-3);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  border-bottom: 1px solid var(--color-border-subtle);
  font-size: var(--text-sm);
  color: var(--color-fg-primary);
}

.general-author {
  margin-right: var(--space-1);
  font-weight: var(--weight-medium);
  color: var(--color-fg-secondary);
}

.layout {
  display: flex;
  flex: 1;
  min-height: 0;
}

/* With no block to work on, Send and Thread still hold the bottom right, as in the action row. */
.idle {
  display: flex;
  flex: 1;
  flex-direction: column;
  min-width: 0;
  min-height: 0;
}

.idle-row {
  display: flex;
  justify-content: flex-end;
  padding: var(--space-3) var(--space-4);
  border-top: 1px solid var(--color-border-subtle);
  background: var(--color-surface-1);
}

.list {
  display: flex;
  flex-direction: column;
  flex-shrink: 0;
  min-height: 0;
  background: var(--color-surface-1);
}

.resize {
  flex-shrink: 0;
  width: 5px;
  margin: 0 -2px;
  z-index: 1;
  border-left: 1px solid var(--color-border-subtle);
  cursor: col-resize;
}

.resize:hover {
  border-left-color: var(--color-fg-accent);
  box-shadow: inset 1px 0 0 var(--color-fg-accent);
}

.list-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: var(--space-2);
  padding: var(--space-2) var(--space-3);
  border-bottom: 1px solid var(--color-border-subtle);
}

.list-title {
  font-size: var(--text-sm);
  font-weight: var(--weight-semibold);
  color: var(--color-fg-secondary);
}

.list-body {
  display: flex;
  flex-direction: column;
  gap: var(--space-3);
  flex: 1;
  min-height: 0;
  overflow-y: auto;
  padding: var(--space-2);
  outline: none;
}

.list-empty {
  margin: 0;
  padding: var(--space-2);
  font-size: var(--text-xs);
  color: var(--color-fg-muted);
}

.group {
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.group-name {
  display: flex;
  align-items: baseline;
  gap: var(--space-2);
  min-width: 0;
  margin: 0;
  padding: var(--space-1) var(--space-2);
  font-size: var(--text-xs);
  font-weight: var(--weight-medium);
  color: var(--color-fg-secondary);
}

.group-file {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-family: var(--font-mono);
  color: var(--color-fg-primary);
}

.group-dir {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  color: var(--color-fg-muted);
}

.row,
.other {
  display: flex;
  align-items: center;
  gap: var(--space-2);
  width: 100%;
  min-width: 0;
  padding: var(--space-1) var(--space-2);
  border: none;
  border-radius: var(--radius-md);
  background: transparent;
  color: var(--color-fg-primary);
  font-family: var(--font-body);
  font-size: var(--text-sm);
  text-align: left;
  cursor: pointer;
}

.row:hover,
.other:hover {
  background: var(--color-surface-2);
}

.row.selected {
  background: var(--color-surface-3);
}

.dot {
  flex-shrink: 0;
  box-sizing: border-box;
  width: 8px;
  height: 8px;
  border-radius: 50%;
  border: 1.5px solid var(--color-fg-muted);
}

.row.needs .dot {
  border-color: var(--color-fg-accent);
  background: var(--color-fg-accent);
}

.row.queued .dot,
.row.done .dot {
  border-color: var(--color-jade);
  background: var(--color-jade);
}

.row.model .dot {
  border-color: var(--color-violet);
  background: var(--color-violet);
}

.row-name {
  flex: 1;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.other {
  justify-content: space-between;
  font-size: var(--text-xs);
}

.other-text {
  flex-shrink: 0;
  color: var(--color-fg-muted);
}

.error {
  margin: 0;
  padding: var(--space-1) var(--space-3);
  font-size: var(--text-xs);
  color: var(--color-error);
}
</style>
