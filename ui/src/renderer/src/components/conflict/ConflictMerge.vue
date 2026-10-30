<!-- SPDX-License-Identifier: AGPL-3.0-only -->
<!-- Copyright (C) 2026 The Lemongrass Authors -->

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, shallowRef, watch } from 'vue'
import type { EditorState, Extension, Text } from '@codemirror/state'
import { EditorView } from '@codemirror/view'
import { applyConflict } from '../../git'
import { conflictCommentExtension, refreshConflictComments } from '../../conflictAnnotations'
import {
  applyText,
  buildMiddle,
  diskSegments,
  fromDisk,
  isCrlf,
  locateChunks,
  mapScroll,
  middleRanges,
  splitLines,
  stripChunks,
  takeText,
  toDisk,
  type ConflictTake
} from '../../conflictModel'
import {
  chunkAtCursor,
  chunkLines,
  middleChunks,
  middleState,
  replaceChunk,
  showCurrent,
  sideState
} from '../../conflictEditor'
import type { ProjectRef } from '../../workspace'
import type {
  ConflictPane,
  GitConflictChunk,
  GitConflictFile,
  GitConflictStage,
  GitLineRange
} from '../../../../preload/types'
import AppButton from '../AppButton.vue'
import AppConfirmButton from '../AppConfirmButton.vue'
import AppModal from '../AppModal.vue'

type ConflictFile = Extract<GitConflictFile, { status: 'ok' }>

const props = defineProps<{
  project: ProjectRef
  path: string
  file: ConflictFile
}>()

interface Loaded {
  chunks: GitConflictChunk[]
  finalNewline: boolean
  crlf: boolean
  doc: Text
  left: GitLineRange[]
  right: GitLineRange[]
}

const leftHost = ref<HTMLElement | null>(null)
const middleHost = ref<HTMLElement | null>(null)
const rightHost = ref<HTMLElement | null>(null)
const loaded = shallowRef<Loaded | null>(null)
const conflictIndexes = computed(() =>
  (loaded.value?.chunks ?? []).flatMap((chunk, index) => (chunk.kind === 'conflict' ? [index] : []))
)
const unresolved = ref(0)
const dirty = ref(false)
const current = ref<number | null>(null)
const applying = ref(false)
const error = ref<string | null>(null)
// A newer working file that arrived while the middle had unsaved edits.
const pendingReload = shallowRef<ConflictFile | null>(null)
let workingSeen = props.file.working
let source: { file: ConflictFile; fromWorking: boolean } = { file: props.file, fromWorking: false }
let left: EditorView | null = null
let middle: EditorView | null = null
let right: EditorView | null = null

const position = computed(() => {
  const at = current.value === null ? -1 : conflictIndexes.value.indexOf(current.value)
  return at === -1 ? null : at + 1
})

const applyBlocked = computed((): string | null => {
  if (applying.value) return 'Applying'
  if (unresolved.value > 0) return 'Resolve every conflict first'
  return null
})

function stageText(stage: GitConflictStage): string {
  return stage.state === 'text' ? stage.text.replace(/\r\n/g, '\n') : ''
}

// The middle starts from git's merge of the stages. After the file changed on disk it starts from
// the file instead, whose remaining markers become its conflicts.
function chunksOf(
  file: ConflictFile,
  fromWorking: boolean
): {
  chunks: GitConflictChunk[]
  finalNewline: boolean
} {
  if (!fromWorking || file.working === null) {
    return { chunks: file.chunks ?? [], finalNewline: file.finalNewline }
  }
  const finalNewline = file.working.endsWith('\n')
  if (file.workingChunks) return { chunks: file.workingChunks, finalNewline }
  const lines = splitLines(file.working)
  return {
    chunks: [{ kind: 'clean', lines, disk: { start: 1, end: lines.length } }],
    finalNewline
  }
}

function middleLineRanges(state: EditorState): GitLineRange[] {
  const chunks = loaded.value?.chunks ?? []
  const conflicts = state.field(middleChunks).map((chunk) => chunkLines(state.doc, chunk))
  return middleRanges(chunks, conflicts, state.doc.lines)
}

function rangesOf(pane: ConflictPane): GitLineRange[] {
  if (pane === 'middle') return middle ? middleLineRanges(middle.state) : []
  return loaded.value?.[pane] ?? []
}

function segmentsOf(pane: ConflictPane): ReturnType<typeof diskSegments> {
  return diskSegments(loaded.value?.chunks ?? [], rangesOf(pane), pane)
}

function reviewExtension(pane: ConflictPane): Extension {
  return conflictCommentExtension({
    project: props.project,
    path: props.path,
    pane,
    operation: props.file.operation,
    leftRef: props.file.leftRef,
    rightRef: props.file.rightRef,
    toDisk: (fromLine, toLine) => toDisk(segmentsOf(pane), fromLine, toLine),
    fromDisk: (startLine, endLine) => fromDisk(segmentsOf(pane), startLine, endLine),
    chunks: () =>
      conflictIndexes.value
        .map((index) => rangesOf(pane)[index])
        .filter((range) => range && range.end >= range.start)
        .map((range) => ({ fromLine: range.start, toLine: range.end }))
  })
}

function onMiddleUpdate(state: EditorState): void {
  const at = chunkAtCursor(state)
  if (at !== null) current.value = at
  unresolved.value = state.field(middleChunks).filter((chunk) => !chunk.resolved).length
  dirty.value = loaded.value !== null && !state.doc.eq(loaded.value.doc)
}

function load(file: ConflictFile, fromWorking: boolean): void {
  source = { file, fromWorking }
  const raw = chunksOf(file, fromWorking)
  const crlf = isCrlf(raw.chunks)
  const chunks = stripChunks(raw.chunks)
  const built = buildMiddle(chunks)
  const leftText = stageText(file.left)
  const rightText = stageText(file.right)
  const empty = chunks.map(() => ({ start: 1, end: 0 }))
  const located = {
    left: locateChunks(chunks, 'left', leftText) ?? empty,
    right: locateChunks(chunks, 'right', rightText) ?? empty
  }
  const sides = (pane: 'left' | 'right'): { index: number; range: GitLineRange }[] =>
    chunks.flatMap((chunk, index) =>
      chunk.kind === 'conflict' ? [{ index, range: located[pane][index] }] : []
    )
  const middleDoc = middleState(built.text, built.chunks, onMiddleUpdate, [
    reviewExtension('middle')
  ])
  loaded.value = {
    chunks,
    finalNewline: raw.finalNewline,
    crlf,
    doc: middleDoc.doc,
    left: located.left,
    right: located.right
  }
  const leftDoc = sideState(leftText, sides('left'), [reviewExtension('left')])
  const rightDoc = sideState(rightText, sides('right'), [reviewExtension('right')])
  if (left && middle && right) {
    left.setState(leftDoc)
    middle.setState(middleDoc)
    right.setState(rightDoc)
    for (const view of [left, middle, right]) refreshConflictComments(view)
  } else {
    left = new EditorView({ state: leftDoc, parent: leftHost.value! })
    middle = new EditorView({ state: middleDoc, parent: middleHost.value! })
    right = new EditorView({ state: rightDoc, parent: rightHost.value! })
    for (const view of [left, middle, right]) {
      view.scrollDOM.addEventListener('scroll', () => syncScroll(view))
    }
  }
  current.value = null
  error.value = null
  onMiddleUpdate(middleDoc)
  if (built.chunks.length > 0) goTo(built.chunks[0].index)
}

watch(current, (index) => {
  for (const view of [left, right]) if (view) showCurrent(view, index)
  // The middle's own listener sets current, and it may not dispatch while it is updating.
  queueMicrotask(() => middle && showCurrent(middle, index))
})

function goTo(index: number): void {
  if (!middle) return
  const chunk = middle.state.field(middleChunks).find((c) => c.index === index)
  if (!chunk) return
  current.value = index
  middle.dispatch({
    selection: { anchor: chunk.from },
    effects: EditorView.scrollIntoView(chunk.from, { y: 'center' })
  })
}

function step(direction: 1 | -1): void {
  const list = conflictIndexes.value
  if (list.length === 0) return
  const at = current.value === null ? -1 : list.indexOf(current.value)
  const next = at === -1 ? (direction === 1 ? 0 : list.length - 1) : at + direction
  goTo(list[(next + list.length) % list.length])
}

function take(side: ConflictTake): void {
  const index = current.value ?? conflictIndexes.value[0]
  const chunk = loaded.value?.chunks[index]
  if (!middle || !chunk) return
  replaceChunk(middle, index, takeText(chunk, side), true)
  middle.focus()
}

function reset(): void {
  load(source.file, source.fromWorking)
}

async function apply(): Promise<void> {
  if (!middle || !loaded.value || applyBlocked.value) return
  applying.value = true
  const content = applyText(
    middle.state.doc.toString(),
    loaded.value.finalNewline,
    loaded.value.crlf
  )
  error.value = await applyConflict(props.project, props.path, content)
  if (!error.value) dirty.value = false
  applying.value = false
}

// A model or a shell changed the working file. With nothing unsaved in the middle it reloads at
// once, otherwise the human chooses, so unsaved edits never silently replace the model's.
watch(
  () => props.file,
  (file) => {
    if (file.working === workingSeen) return
    if (dirty.value) {
      pendingReload.value = file
      return
    }
    workingSeen = file.working
    load(file, true)
  }
)

function reloadPending(): void {
  const file = pendingReload.value
  pendingReload.value = null
  if (!file) return
  workingSeen = file.working
  load(file, true)
}

function keepEdits(): void {
  workingSeen = pendingReload.value?.working ?? workingSeen
  pendingReload.value = null
}

// Scroll position as a fractional line boundary, 0 being the top of the first line.
function scrollLine(view: EditorView): number {
  const top = view.scrollDOM.scrollTop - view.documentPadding.top
  const block = view.lineBlockAtHeight(Math.max(0, top))
  const line = view.state.doc.lineAt(block.from).number
  const fraction = block.height > 0 ? (top - block.top) / block.height : 0
  return line - 1 + Math.min(1, Math.max(0, fraction))
}

function scrollTopOf(view: EditorView, position: number): number {
  const doc = view.state.doc
  const line = Math.min(doc.lines, Math.max(1, Math.floor(position) + 1))
  const block = view.lineBlockAt(doc.line(line).from)
  return block.top + (position - (line - 1)) * block.height + view.documentPadding.top
}

// Panes moved by the sync skip their own next scroll event, so the sync never echoes back.
const synced = new Set<EditorView>()

function syncScroll(from: EditorView): void {
  if (synced.delete(from)) return
  const paneOf = (view: EditorView): ConflictPane =>
    view === left ? 'left' : view === right ? 'right' : 'middle'
  const position = scrollLine(from)
  for (const to of [left, middle, right]) {
    if (!to || to === from) continue
    const mapped = mapScroll(
      rangesOf(paneOf(from)),
      rangesOf(paneOf(to)),
      from.state.doc.lines,
      to.state.doc.lines,
      position
    )
    const before = to.scrollDOM.scrollTop
    to.scrollDOM.scrollTop = scrollTopOf(to, mapped)
    if (Math.abs(to.scrollDOM.scrollTop - before) >= 1) synced.add(to)
  }
}

onMounted(() => load(props.file, false))

onBeforeUnmount(() => {
  for (const view of [left, middle, right]) view?.destroy()
})
</script>

<template>
  <div class="conflict-merge">
    <div class="toolbar">
      <AppButton title="Previous conflict" :disabled="!conflictIndexes.length" @click="step(-1)">
        Previous
      </AppButton>
      <AppButton title="Next conflict" :disabled="!conflictIndexes.length" @click="step(1)">
        Next
      </AppButton>
      <span class="count">
        <template v-if="conflictIndexes.length === 0">No conflicts left</template>
        <template v-else>
          <template v-if="position"
            >Conflict {{ position }} of {{ conflictIndexes.length }}.</template
          >
          {{ unresolved }} unresolved
        </template>
      </span>
      <span class="spacer" />
      <AppButton :disabled="!conflictIndexes.length" @click="take('left')">Take left</AppButton>
      <AppButton :disabled="!conflictIndexes.length" @click="take('right')">Take right</AppButton>
      <AppButton title="Left, then right" :disabled="!conflictIndexes.length" @click="take('both')">
        Take both
      </AppButton>
      <AppConfirmButton
        variant="ghost"
        title="Put the middle back as it was loaded"
        :disabled="!dirty"
        @confirm="reset"
      >
        Reset
      </AppConfirmButton>
      <AppButton
        variant="primary"
        :title="applyBlocked ?? 'Write the middle to the file and mark it resolved'"
        :disabled="applyBlocked !== null"
        @click="apply"
      >
        Apply
      </AppButton>
    </div>
    <p v-if="error" class="merge-error">{{ error }}</p>
    <div class="panes">
      <section class="pane">
        <header class="pane-head" :title="file.leftRef">
          <span class="side">Left</span><span class="ref">{{ file.leftRef }}</span>
        </header>
        <div ref="leftHost" class="pane-body" />
      </section>
      <section class="pane middle">
        <header class="pane-head">
          <span class="side">Result</span>
          <span v-if="dirty" class="ref">Unsaved</span>
        </header>
        <div ref="middleHost" class="pane-body" />
      </section>
      <section class="pane">
        <header class="pane-head" :title="file.rightRef">
          <span class="side">Right</span><span class="ref">{{ file.rightRef }}</span>
        </header>
        <div ref="rightHost" class="pane-body" />
      </section>
    </div>

    <AppModal v-if="pendingReload" :title="`${path} changed on disk`" @close="keepEdits">
      <p class="reload-note">
        The file changed while the middle pane has unsaved edits. Reloading drops those edits.
      </p>
      <template #footer>
        <AppButton @click="keepEdits">Keep my edits</AppButton>
        <AppButton variant="primary" @click="reloadPending">Reload</AppButton>
      </template>
    </AppModal>
  </div>
</template>

<style scoped>
.conflict-merge {
  position: relative;
  display: flex;
  flex-direction: column;
  flex: 1;
  min-height: 0;
}

.toolbar {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: var(--space-2);
  padding: var(--space-2) var(--space-3);
  border-bottom: 1px solid var(--color-border-subtle);
}

.count {
  font-size: var(--text-xs);
  color: var(--color-fg-secondary);
}

.spacer {
  flex: 1;
}

.merge-error {
  margin: 0;
  padding: var(--space-2) var(--space-3);
  font-size: var(--text-xs);
  color: var(--color-error);
}

.panes {
  display: flex;
  flex: 1;
  min-height: 0;
}

.pane {
  display: flex;
  flex-direction: column;
  flex: 1;
  min-width: 0;
  border-right: 1px solid var(--color-border-subtle);
}

.pane:last-child {
  border-right: none;
}

.pane-head {
  display: flex;
  gap: var(--space-2);
  padding: var(--space-1) var(--space-3);
  font-size: var(--text-xs);
  border-bottom: 1px solid var(--color-border-subtle);
  white-space: nowrap;
  overflow: hidden;
}

.side {
  color: var(--color-fg-primary);
  font-weight: var(--weight-medium);
}

.ref {
  color: var(--color-fg-muted);
  overflow: hidden;
  text-overflow: ellipsis;
}

.pane-body {
  flex: 1;
  min-height: 0;
}

.reload-note {
  margin: 0;
  font-size: var(--text-sm);
  color: var(--color-fg-secondary);
}
</style>
