<!-- SPDX-License-Identifier: MPL-2.0 -->
<!-- Copyright (C) 2026 The Lemongrass Authors -->

<script setup lang="ts">
import {
  computed,
  onActivated,
  onBeforeUnmount,
  onDeactivated,
  onMounted,
  ref,
  shallowRef,
  watch
} from 'vue'
import { Text, type EditorState, type Extension } from '@codemirror/state'
import { EditorView } from '@codemirror/view'
import { applyConflict } from '../../git'
import {
  dropDraft,
  registerWork,
  saveDraft,
  stagesOf,
  takeDraft,
  type ConflictDraft
} from '../../conflictDrafts'
import {
  applyText,
  buildMiddle,
  buildProposal,
  collapseSpans,
  decide,
  decideAll,
  isCrlf,
  locateChunks,
  mapScroll,
  middleRanges,
  savedText,
  sideMark,
  splitLines,
  stripChunks,
  type ConflictSide,
  type ConflictTake,
  type LoadPlan,
  type MiddleChunk,
  type MiddleProposal
} from '../../conflictModel'
import { blockDigest } from '../../conflictDecisions'
import {
  chunkAtCursor,
  chunkLines,
  flashLines,
  focusLines,
  middleChunks,
  middleState,
  restoreMiddle,
  chunkReplacements,
  setFocusLines,
  showCurrent,
  showDecisions,
  snapshotMiddle,
  sideState
} from '../../conflictEditor'
import { hasConflictTab, type ProjectRef } from '../../workspace'
import { askAboutBlocks, focusOf, publishManual } from '../../conflictVerdicts'
import type {
  ConflictPane,
  GitConflictChunk,
  GitConflictFile,
  GitConflictStage,
  GitLineRange
} from '../../../../preload/types'
import { AppButton, AppIconButton, AppMenuButton, AppModal, type MenuItem } from '@sankasten'
import ConflictIcon from './ConflictIcon.vue'
import AskModelModal from './AskModelModal.vue'
import { CONFLICT_ICONS } from '../../conflictIcons'

type ConflictFile = Extract<GitConflictFile, { status: 'ok' }>

// Manual: the three panes as a merge tool, with none of a model's marks. While a model holds the
// conflict the middle is the working file with git's blocks mapped into it, so a block the human
// settled in Decisions loads settled, one the human chose to resolve by hand loads open on its
// base, and any other text a model wrote loads as an open block holding that text.
const props = withDefaults(
  defineProps<{
    project: ProjectRef
    path: string
    file: ConflictFile
    modelHolds?: boolean
    // Block numbers the human already accepted or resolved, and those to reopen by hand.
    settledBlocks?: number[]
    reopenBlocks?: number[]
  }>(),
  { modelHolds: false, settledBlocks: () => [], reopenBlocks: () => [] }
)

interface Loaded {
  chunks: GitConflictChunk[]
  finalNewline: boolean
  crlf: boolean
  doc: Text
  left: GitLineRange[]
  right: GitLineRange[]
  // git's block numbers each conflict chunk covers, by chunk index.
  blocks: Record<number, number[]>
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
const focus = ref(false)
// Flex weights of the three panes, changed by dragging the gutters between them.
const sizes = ref([1, 1, 1])
const panesEl = ref<HTMLElement | null>(null)
let workingSeen = props.file.working
type Source = {
  chunks: GitConflictChunk[]
  finalNewline: boolean
  proposal?: MiddleProposal | null
  plan?: Record<number, LoadPlan>
}
// What the middle was loaded from, which Reset goes back to.
let source: Source = { chunks: [], finalNewline: true }
// Set once the work is written, so unmounting keeps no draft of it.
let applied = false
let left: EditorView | null = null
let middle: EditorView | null = null
let right: EditorView | null = null

const position = computed(() => {
  const at = current.value === null ? -1 : conflictIndexes.value.indexOf(current.value)
  return at === -1 ? null : at + 1
})

const applyBlocked = computed((): string | null => {
  if (applying.value) return 'Applying'
  if (unresolved.value === 0) return null
  const left = unresolved.value === 1 ? '1 conflict left' : `${unresolved.value} conflicts left`
  return `${left}. Settle each one first.`
})

// The toolbar is status, not controls: a short counter, then what is left as plain text.
const counterText = computed(() => {
  const total = conflictIndexes.value.length
  if (total === 0) return 'No conflicts left'
  return position.value ? `${position.value} of ${total}` : `${total} conflicts`
})

const statusText = computed(() => {
  if (conflictIndexes.value.length === 0) return ''
  return unresolved.value > 0 ? `${unresolved.value} open` : 'all settled'
})

const statusTitle = computed(() => {
  const total = conflictIndexes.value.length
  if (total === 0) return 'No conflicts left in this file'
  const at = position.value ? `Conflict ${position.value} of ${total}. ` : ''
  return `${at}${unresolved.value} unresolved.`
})

// The whole-file actions replace every conflict in this file at once, so they sit behind the
// overflow icon, and confirm with a second click whenever the human's edits would go. They never
// reach past this file.
const menuItems = computed((): MenuItem[] => {
  const none = conflictIndexes.value.length === 0
  return [
    {
      key: 'left',
      label: `Take ${props.file.leftRef} in this file`,
      icon: CONFLICT_ICONS.takeLeft,
      disabled: none,
      confirm: dirty.value
    },
    {
      key: 'right',
      label: `Take ${props.file.rightRef} in this file`,
      icon: CONFLICT_ICONS.takeRight,
      disabled: none,
      confirm: dirty.value
    },
    {
      key: 'both',
      label: 'Take both in this file',
      icon: CONFLICT_ICONS.takeBoth,
      disabled: none,
      confirm: dirty.value
    },
    {
      key: 'reset',
      label: 'Reset',
      icon: CONFLICT_ICONS.reset,
      title: 'Put the middle back as it was loaded',
      disabled: !dirty.value,
      confirm: true
    }
  ]
})

function onMenu(key: string): void {
  if (key === 'left' || key === 'right' || key === 'both') takeAll(key)
  else if (key === 'reset') reset()
}

function stageText(stage: GitConflictStage): string {
  return stage.state === 'text' ? stage.text.replace(/\r\n/g, '\n') : ''
}

// How each written block loads: settled when the human already accepted or resolved it, or when
// no model holds the conflict, since the text is then the human's own saved work; reopened when the
// human chose to resolve it by hand; otherwise a model's text still waiting for Decisions.
function planOf(
  proposal: MiddleProposal & { blocks: Record<number, number[]> }
): Record<number, LoadPlan> {
  const plan: Record<number, LoadPlan> = {}
  for (const index of Object.keys(proposal.lines).map(Number)) {
    const blocks = proposal.blocks[index] ?? []
    if (blocks.some((n) => props.reopenBlocks.includes(n))) plan[index] = 'reopen'
    else if (!props.modelHolds || blocks.every((n) => props.settledBlocks.includes(n))) {
      plan[index] = 'settled'
    } else plan[index] = 'model'
  }
  return plan
}

// The middle is the working file with git's blocks mapped into it, so it shows what is on disk,
// the human's saved work and a model's text included. A file with no working text, or one too
// large to map, starts from git's merge of the stages.
function chunksOf(file: ConflictFile): Source {
  if (file.chunks && file.working !== null) {
    const built = buildProposal(file.chunks, file.working, file.workingChunks)
    if (built) return { ...built, plan: planOf(built.proposal) }
  }
  return { chunks: file.chunks ?? [], finalNewline: file.finalNewline }
}

// git's block numbers of each conflict chunk: given by the mapping, or counted in order when the
// middle came straight from git's merge.
function blocksOf(raw: Source): Record<number, number[]> {
  if (raw.proposal?.blocks) return raw.proposal.blocks
  const out: Record<number, number[]> = {}
  let n = 0
  raw.chunks.forEach((chunk, index) => {
    if (chunk.kind === 'conflict') out[index] = [++n]
  })
  return out
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

// Focus hides clean lines away from the conflicts.
function focusExtension(pane: ConflictPane): Extension {
  return focusLines(
    (state) => {
      const chunks = loaded.value?.chunks ?? []
      const ranges = pane === 'middle' ? middleLineRanges(state) : (loaded.value?.[pane] ?? [])
      const clean = chunks.flatMap((chunk, index) =>
        chunk.kind === 'clean' ? [ranges[index]] : []
      )
      return collapseSpans(clean, [], state.doc.lines)
    },
    () => false
  )
}

function toggleFocus(): void {
  focus.value = !focus.value
  for (const view of [left, middle, right]) if (view) setFocusLines(view, focus.value)
}

function onMiddleUpdate(state: EditorState): void {
  const at = chunkAtCursor(state)
  if (at !== null) current.value = at
  const chunks = state.field(middleChunks)
  unresolved.value = chunks.filter((chunk) => !chunk.resolved).length
  dirty.value = loaded.value !== null && !state.doc.eq(loaded.value.doc)
  pushDecisions(chunks)
  publishResolved(state, chunks)
}

// The blocks the human resolved here, with the digest of their lines, for Decisions to show and
// send, and for a later hand-over to pass on as settled.
function publishResolved(state: EditorState, chunks: MiddleChunk[]): void {
  if (!loaded.value) return
  const resolved = new Map<number, string>()
  for (const chunk of chunks) {
    if (!chunk.resolved || chunk.settled) continue
    const lines = splitLines(state.doc.sliceString(chunk.from, chunk.to))
    for (const n of loaded.value.blocks[chunk.index] ?? []) resolved.set(n, blockDigest(lines))
  }
  publishManual(props.project.id, props.path, resolved)
}

let shownDecisions = ''

// The side panes draw the middle's decisions on their chevrons, so they hear about every change.
function pushDecisions(chunks: MiddleChunk[]): void {
  const marks = chunks.map((chunk) => ({
    index: chunk.index,
    left: sideMark(chunk, 'left'),
    right: sideMark(chunk, 'right')
  }))
  const key = JSON.stringify(marks.map(({ index, left, right }) => [index, left, right]))
  if (key === shownDecisions || !left || !right) return
  shownDecisions = key
  for (const [view, side] of [
    [left, 'left'],
    [right, 'right']
  ] as const) {
    showDecisions(view, Object.fromEntries(marks.map((mark) => [mark.index, mark[side]])))
  }
}

// A draft restores the middle as it was left, decisions included; otherwise it is built from raw.
function load(file: ConflictFile, raw: Source, draft: ConflictDraft | null = null): void {
  source = raw
  const crlf = isCrlf(raw.chunks)
  const chunks = stripChunks(raw.chunks)
  const built = buildMiddle(chunks, raw.proposal ?? null, raw.plan ?? {})
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
  const middleExtras = [focusExtension('middle')]
  const middleDoc = draft
    ? restoreMiddle(draft.middle, onMiddleUpdate, middleExtras)
    : middleState(built.text, built.chunks, onMiddleUpdate, middleExtras)
  loaded.value = {
    chunks,
    finalNewline: raw.finalNewline,
    crlf,
    doc: draft ? Text.of(draft.loadedText.split('\n')) : middleDoc.doc,
    left: located.left,
    right: located.right,
    blocks: blocksOf(raw)
  }
  const leftDoc = sideState(
    leftText,
    sides('left'),
    'left',
    file.leftRef,
    (index, d) => decideSide(index, 'left', d),
    [focusExtension('left')]
  )
  const rightDoc = sideState(
    rightText,
    sides('right'),
    'right',
    file.rightRef,
    (index, d) => decideSide(index, 'right', d),
    [focusExtension('right')]
  )
  if (left && middle && right) {
    left.setState(leftDoc)
    middle.setState(middleDoc)
    right.setState(rightDoc)
  } else {
    left = new EditorView({ state: leftDoc, parent: leftHost.value! })
    middle = new EditorView({ state: middleDoc, parent: middleHost.value! })
    right = new EditorView({ state: rightDoc, parent: rightHost.value! })
    for (const view of [left, middle, right]) {
      view.scrollDOM.addEventListener('scroll', () => syncScroll(view))
    }
  }
  if (focus.value) for (const view of [left, middle, right]) setFocusLines(view, true)
  current.value = null
  error.value = null
  shownDecisions = ''
  onMiddleUpdate(middleDoc)
  if (draft) {
    current.value = draft.current
    middle.dispatch({
      effects: EditorView.scrollIntoView(middleDoc.selection.main.head, { y: 'center' })
    })
  } else {
    const firstOpen = built.chunks.find((chunk) => !chunk.resolved) ?? built.chunks[0]
    if (firstOpen) goTo(firstOpen.index)
  }
  landFocus()
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

// A block Decisions sent the human here for: the middle lands on it and lights it for a moment.
let landed = 0
function landFocus(): void {
  const request = focusOf(props.project.id)
  if (!request || request.nonce === landed || request.path !== props.path) return
  if (!middle || !loaded.value) return
  landed = request.nonce
  const blocks = loaded.value.blocks
  const index = Object.keys(blocks)
    .map(Number)
    .find((i) => blocks[i].includes(request.block))
  const chunk = middle.state.field(middleChunks).find((c) => c.index === index)
  if (index === undefined || !chunk) return
  goTo(index)
  const lines = chunkLines(middle.state.doc, chunk)
  flashLines(middle, lines.start, Math.max(lines.start, lines.end))
}

watch(
  () => focusOf(props.project.id)?.nonce,
  () => landFocus()
)

function conflictAt(index: number): Extract<GitConflictChunk, { kind: 'conflict' }> | null {
  const chunk = loaded.value?.chunks[index]
  return chunk?.kind === 'conflict' ? chunk : null
}

// A chevron or cross in a side pane decides that side of one conflict.
function decideSide(index: number, side: ConflictSide, decision: 'taken' | 'rejected'): void {
  const chunk = middle?.state.field(middleChunks).find((c) => c.index === index)
  const conflict = conflictAt(index)
  if (!middle || !chunk || !conflict) return
  const spec = chunkReplacements(middle.state, [decide(chunk, conflict, side, decision)])
  if (!spec) return
  current.value = index
  middle.dispatch(spec)
}

// The toolbar's takes decide every conflict in the file at once.
function takeAll(take: ConflictTake): void {
  if (!middle) return
  const updates = middle.state.field(middleChunks).flatMap((chunk) => {
    const conflict = conflictAt(chunk.index)
    return conflict ? [decideAll(chunk, conflict, take)] : []
  })
  const spec = chunkReplacements(middle.state, updates)
  if (spec) middle.dispatch(spec)
}

// Dragging the gutter after pane `at` moves width between it and the next pane, each keeping at
// least 15 percent, so a narrow window can still give one pane room.
function startResize(at: number, event: PointerEvent): void {
  const width = panesEl.value?.clientWidth ?? 0
  if (!width) return
  const total = sizes.value.reduce((sum, size) => sum + size, 0)
  const start = [...sizes.value]
  const startX = event.clientX
  const min = total * 0.15
  const target = event.currentTarget as HTMLElement
  target.setPointerCapture(event.pointerId)
  const move = (e: PointerEvent): void => {
    const pair = start[at] + start[at + 1]
    const next = Math.min(
      pair - min,
      Math.max(min, start[at] + ((e.clientX - startX) / width) * total)
    )
    const updated = [...start]
    updated[at] = next
    updated[at + 1] = pair - next
    sizes.value = updated
  }
  const end = (): void => {
    target.removeEventListener('pointermove', move)
    target.removeEventListener('pointerup', end)
    target.removeEventListener('pointercancel', end)
  }
  target.addEventListener('pointermove', move)
  target.addEventListener('pointerup', end)
  target.addEventListener('pointercancel', end)
}

function reset(): void {
  load(props.file, source)
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
  if (!error.value) {
    dirty.value = false
    applied = true
    dropDraft(props.project.id, props.path)
    publishManual(props.project.id, props.path, new Map())
  }
  applying.value = false
}

// A model or a shell changed the working file. With nothing unsaved in the middle it reloads at
// once, otherwise the human chooses, so unsaved edits never silently replace the model's.
function onFileChange(file: ConflictFile): void {
  if (file.working === workingSeen) return
  if (dirty.value) {
    pendingReload.value = file
    return
  }
  workingSeen = file.working
  load(file, chunksOf(file))
}

watch(() => props.file, onFileChange)

// What Decisions settled or sent here changed: the middle loads again, unless it holds the human's
// own edits, which are never dropped without asking.
const planKey = computed(() =>
  JSON.stringify([props.modelHolds, props.settledBlocks, props.reopenBlocks])
)
watch(planKey, () => {
  if (dirty.value) return
  load(props.file, chunksOf(props.file))
})

function reloadPending(): void {
  const file = pendingReload.value
  pendingReload.value = null
  if (!file) return
  workingSeen = file.working
  load(file, chunksOf(file))
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

// Ask a model about the block under the cursor: the modal holds the message and the tab, and its
// Submit is the only step that acts, since handing the conflict over wakes the model.
const askOpen = ref(false)
const asking = ref(false)
const askError = ref<string | null>(null)
const asked = ref(false)

const askBlocks = computed(() =>
  current.value === null ? [] : (loaded.value?.blocks[current.value] ?? [])
)

function startAsk(): void {
  askError.value = null
  asked.value = false
  askOpen.value = true
}

async function sendAsk(payload: { tabId: string | null; text: string }): Promise<void> {
  asking.value = true
  askError.value = await askAboutBlocks(
    props.project,
    props.path,
    askBlocks.value,
    payload.text,
    payload.tabId
  )
  asking.value = false
  if (askError.value) return
  askOpen.value = false
  asked.value = true
}

const refs = {
  operation: props.file.operation,
  leftRef: props.file.leftRef,
  rightRef: props.file.rightRef
}

// A hand-over or a send saves this tab's work to disk first, but only once the middle differs from
// what it loaded.
const unregisterWork = registerWork(props.project.id, props.path, () => {
  if (!middle || !loaded.value || middle.state.doc.eq(loaded.value.doc)) return null
  const saved = savedText(
    middle.state.doc.toString(),
    middle.state.field(middleChunks),
    loaded.value.chunks,
    refs,
    loaded.value.finalNewline,
    loaded.value.crlf
  )
  return { ...saved, seen: workingSeen, markSeen: (text: string) => (workingSeen = text) }
})

onMounted(() => {
  const draft = takeDraft(props.project.id, props.path, stagesOf(props.file))
  if (draft) {
    workingSeen = draft.workingSeen
    focus.value = draft.focus
    sizes.value = draft.sizes
  }
  load(props.file, draft ?? chunksOf(props.file), draft)
  // A working file that changed while the tab was away reloads or asks, as it would have.
  if (draft && props.file.working !== workingSeen) onFileChange(props.file)
})

// The conflict tab caches this file while it shows another, taking it out of the document, which
// loses the panes' scroll positions, so they are put back when the file is shown again.
let scrollTops: number[] | null = null
onDeactivated(() => {
  scrollTops = [left, middle, right].map((view) => view?.scrollDOM.scrollTop ?? 0)
})
// A block Decisions sent the human to lands after the restored scroll, so it wins.
onActivated(() => {
  const tops = scrollTops
  scrollTops = null
  if (tops) {
    ;[left, middle, right].forEach((view, at) => {
      if (!view) return
      view.requestMeasure()
      view.scrollDOM.scrollTop = tops[at]
    })
  }
  landFocus()
})

// The file is unmounted when its project is switched away, its tab moves to another pane or the
// tab closes; it keeps its work for the next mount unless the tab was closed or it was applied.
onBeforeUnmount(() => {
  unregisterWork()
  if (middle && loaded.value && !applied && hasConflictTab(props.project.id)) {
    saveDraft(props.project.id, props.path, {
      stages: stagesOf(props.file),
      chunks: source.chunks,
      finalNewline: source.finalNewline,
      proposal: source.proposal ?? null,
      plan: source.plan,
      loadedText: loaded.value.doc.toString(),
      refs,
      middle: snapshotMiddle(middle.state),
      workingSeen,
      focus: focus.value,
      sizes: sizes.value,
      current: current.value
    })
  }
  for (const view of [left, middle, right]) view?.destroy()
})
</script>

<template>
  <div class="conflict-merge">
    <!-- Apply is the only button with a frame: navigation and Focus are bare icons, what is left
         is plain text, and the whole-file takes wait behind the overflow icon. -->
    <div class="toolbar">
      <slot name="picker" />
      <div class="nav">
        <AppIconButton
          size="sm"
          tone="accent"
          label="Previous conflict"
          :disabled="!conflictIndexes.length"
          @click="step(-1)"
        >
          <ConflictIcon name="prev" />
        </AppIconButton>
        <span class="counter">{{ counterText }}</span>
        <AppIconButton
          size="sm"
          tone="accent"
          label="Next conflict"
          :disabled="!conflictIndexes.length"
          @click="step(1)"
        >
          <ConflictIcon name="next" />
        </AppIconButton>
      </div>
      <span v-if="statusText" class="status" :title="statusTitle">{{ statusText }}</span>
      <span class="spacer" />
      <AppButton
        variant="text"
        tone="accent"
        size="sm"
        class="ask"
        :disabled="askBlocks.length === 0"
        :title="
          askBlocks.length
            ? 'Ask a model about the block under the cursor'
            : 'Put the cursor in a conflict block to ask about it'
        "
        @click="startAsk"
      >
        <ConflictIcon name="ask" />
        Ask model
      </AppButton>
      <AppIconButton
        size="sm"
        tone="accent"
        :pressed="focus"
        :label="focus ? 'Show every line' : 'Hide the lines that need no merging'"
        @click="toggleFocus"
      >
        <ConflictIcon name="focus" />
      </AppIconButton>
      <AppMenuButton size="sm" :items="menuItems" label="Whole-file actions" @select="onMenu">
        <ConflictIcon name="more" />
      </AppMenuButton>
      <AppButton
        variant="primary"
        size="sm"
        class="apply"
        :title="applyBlocked ?? 'Write the middle to the file and mark it resolved'"
        :disabled="applyBlocked !== null"
        @click="apply"
      >
        Apply
      </AppButton>
    </div>
    <p v-if="asked" class="merge-note">Asked. The answer comes back in Decisions.</p>
    <p v-if="error" class="merge-error">{{ error }}</p>
    <div ref="panesEl" class="panes">
      <section class="pane" :style="{ flexGrow: sizes[0] }">
        <header class="pane-head" :title="file.leftRef">
          <span class="side ref">{{ file.leftRef }}</span>
        </header>
        <div ref="leftHost" class="pane-body" />
      </section>
      <div class="gutter" @pointerdown="startResize(0, $event)" />
      <section class="pane middle" :style="{ flexGrow: sizes[1] }">
        <header class="pane-head">
          <span class="side">Result</span>
          <span v-if="dirty" class="ref">Unsaved</span>
        </header>
        <div ref="middleHost" class="pane-body" />
      </section>
      <div class="gutter" @pointerdown="startResize(1, $event)" />
      <section class="pane" :style="{ flexGrow: sizes[2] }">
        <header class="pane-head" :title="file.rightRef">
          <span class="side ref">{{ file.rightRef }}</span>
        </header>
        <div ref="rightHost" class="pane-body" />
      </section>
    </div>

    <AskModelModal
      v-if="askOpen"
      :project="project"
      :held="modelHolds"
      :blocks="askBlocks.length"
      :sending="asking"
      :error="askError"
      @submit="sendAsk"
      @close="askOpen = false"
    />

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
  gap: var(--space-2);
  min-width: 0;
  padding: var(--space-2) var(--space-3);
  border-bottom: 1px solid var(--color-border-subtle);
}

.nav {
  display: flex;
  flex-shrink: 0;
  align-items: center;
  gap: 2px;
}

.counter {
  min-width: 4ch;
  font-size: var(--text-xs);
  color: var(--color-fg-secondary);
  text-align: center;
  font-variant-numeric: tabular-nums;
  white-space: nowrap;
}

/* Status, not a control: the only part of the bar that gives way when it is narrow. */
.status {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-size: var(--text-xs);
  color: var(--color-fg-muted);
}

.ask {
  flex-shrink: 0;
}

.merge-note {
  margin: 0;
  padding: var(--space-1) var(--space-3);
  font-size: var(--text-xs);
  color: var(--color-fg-secondary);
  border-bottom: 1px solid var(--color-border-subtle);
}

.toolbar .apply {
  flex-shrink: 0;
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
  flex: 1 1 0;
  min-width: 0;
}

.gutter {
  flex-shrink: 0;
  width: 4px;
  cursor: col-resize;
  background: var(--color-border-subtle);
  transition: background var(--duration-fast) var(--ease-out);
}

.gutter:hover,
.gutter:active {
  background: var(--color-amber);
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

.side.ref {
  color: var(--color-fg-primary);
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
