<!-- SPDX-License-Identifier: MPL-2.0 -->
<!-- Copyright (C) 2026 The Lemongrass Authors -->

<script setup lang="ts">
import { computed, ref } from 'vue'
import { AppButton, AppEmptyState, AppInput, AppSegmented } from '@sankasten'
import { postToConflict, type ConflictThreadState } from '../../conflictThread'
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
  applyFile,
  askAboutBlocks,
  draftsOf,
  focusBlock,
  manualOf,
  pendingFor,
  resolveByHand,
  setDraft
} from '../../conflictVerdicts'
import { tabParts, type NamePart } from '../../tabNames'
import type { ProjectRef } from '../../workspace'
import AskModelModal from './AskModelModal.vue'
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
const filterChoice = ref<'needs' | 'all' | null>(null)
const filter = computed(
  () => filterChoice.value ?? (counts.value.needs + counts.value.queued > 0 ? 'needs' : 'all')
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

const selectedKey = ref<string | null>(null)
// The picked block while it is listed, else the first block that needs the human, else the first.
const current = computed(
  (): Row | null =>
    shown.value.find((row) => row.card.key === selectedKey.value) ??
    shown.value.find((row) => row.state === 'needs') ??
    shown.value[0] ??
    null
)

function blocksText(card: BlockCard): string {
  const blocks = card.blocks
  return blocks.length === 1 ? `Block ${blocks[0]}` : `Blocks ${blocks[0]} to ${blocks.at(-1)}`
}

function select(key: string): void {
  selectedKey.value = key
}

// Up and Down walk the list, as in any list that has a selection.
function onListKey(event: KeyboardEvent): void {
  if (event.key !== 'ArrowDown' && event.key !== 'ArrowUp') return
  const list = shown.value
  if (list.length === 0) return
  event.preventDefault()
  const at = list.findIndex((row) => row.card.key === current.value?.card.key)
  const next = Math.min(list.length - 1, Math.max(0, at + (event.key === 'ArrowDown' ? 1 : -1)))
  selectedKey.value = list[next].card.key
}

// After a decision the selection moves to the next block that still needs the human, after the
// current one first and then from the top.
function advance(): void {
  const list = rows.value
  const at = list.findIndex((row) => row.card.key === current.value?.card.key)
  const after = [...list.slice(at + 1), ...list.slice(0, at)]
  const next = after.find((row) => row.state === 'needs')
  if (next) selectedKey.value = next.card.key
}

// Who holds the conflict, highlighted since its title is the model's own words.
const holder = computed((): NamePart[] => {
  const tabId = props.thread.status?.assignee
  return tabId ? tabParts(tabId) : [{ text: 'a model' }]
})
const statusParts = computed((): NamePart[] => {
  const step = ` ${nextStep(counts.value)}`
  return finished.value
    ? [...holder.value, { text: ` finished its round.${step}` }]
    : [{ text: 'Handed to ' }, ...holder.value, { text: `.${step}` }]
})

const line = computed(() => generalLine(props.thread.messages))
const messaging = ref(false)
const reply = ref('')
const replying = ref(false)
const replyError = ref<string | null>(null)

async function sendReply(): Promise<void> {
  const body = reply.value.trim()
  if (!body || replying.value) return
  replying.value = true
  replyError.value = await postToConflict(props.project, [{ body, anchor: null }])
  replying.value = false
  if (!replyError.value) {
    reply.value = ''
    messaging.value = false
  }
}

function byHand(card: BlockCard): void {
  resolveByHand(props.project.id, card)
  emit('manual', card.path)
}

function showInManual(card: BlockCard): void {
  focusBlock(props.project.id, card.path, card.blocks[0])
  emit('manual', card.path)
}

const askCard = ref<BlockCard | null>(null)
const asking = ref(false)
const askError = ref<string | null>(null)

function startAsk(card: BlockCard): void {
  askError.value = null
  askCard.value = card
}

async function sendAsk(payload: { tabId: string | null; text: string }): Promise<void> {
  const card = askCard.value
  if (!card) return
  asking.value = true
  askError.value = await askAboutBlocks(props.project, card.path, card.blocks, payload.text, null)
  asking.value = false
  if (!askError.value) askCard.value = null
}

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

// Stage when every file waiting holds nothing the model has to read, else Send to model.
const staging = computed(() => ready.value.length > 0 && ready.value.every((item) => item.settled))
const sendLabel = computed(() => {
  if (ready.value.length === 0) return 'Send to model'
  if (staging.value) return ready.value.length === 1 ? 'Stage' : `Stage (${ready.value.length})`
  return `Send to model (${counts.value.queued})`
})
const sendTitle = computed(() =>
  ready.value.length === 0
    ? 'Nothing is decided yet.'
    : ready.value
        .map((item) => `${fileName(item.file.path)}: ${summary(item.verdicts, item.settled)}`)
        .join('\n')
)

const sending = ref(false)
const sendError = ref<string | null>(null)

// One batch per file, each posted together and staged when its file is settled.
async function send(): Promise<void> {
  if (sending.value || ready.value.length === 0) return
  sending.value = true
  const errors: string[] = []
  for (const item of [...ready.value]) {
    const refused = await applyFile(props.project, item.file)
    if (refused) errors.push(`${fileName(item.file.path)}: ${refused}`)
  }
  sendError.value = errors.length > 0 ? errors.join(' ') : null
  sending.value = false
}
</script>

<template>
  <div class="decisions">
    <header class="top">
      <ConflictModeSwitch mode="decisions" @switch="emit('manual', null)" />
      <MarkedText class="status" :parts="statusParts" capitalize />
      <AppButton
        variant="primary"
        size="sm"
        class="send"
        :disabled="ready.length === 0 || sending"
        :title="sendTitle"
        @click="send"
      >
        {{ sending ? 'Sending' : sendLabel }}
      </AppButton>
    </header>

    <p v-if="sendError" class="error">{{ sendError }}</p>
    <p v-if="line" class="general" :title="line.body">
      <span class="general-author">Model</span>
      {{ line.body }}
    </p>

    <AppEmptyState
      v-if="rows.length === 0 && noBlocks.length === 0"
      layout="fill"
      :loading="decisions.length < files.length"
      title="Nothing to decide yet."
      detail="Each block shows here as soon as there is one."
    />
    <div v-else class="layout">
      <aside class="list">
        <header class="list-head">
          <span class="list-title">Blocks</span>
          <AppSegmented
            size="sm"
            aria-label="Which blocks to list"
            :options="filterOptions"
            :model-value="filter"
            @update:model-value="filterChoice = $event"
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
              <span class="row-state">{{ row.label }}</span>
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

        <footer class="list-foot">
          <template v-if="messaging">
            <AppInput
              v-model="reply"
              size="sm"
              class="reply"
              placeholder="A line for the model, then Enter"
              :disabled="replying"
              @keydown.enter.prevent="sendReply"
              @keydown.escape="messaging = false"
            />
            <p v-if="replyError" class="error">{{ replyError }}</p>
          </template>
          <AppButton v-else variant="text" size="sm" @click="messaging = true">
            Message the model
          </AppButton>
        </footer>
      </aside>

      <DecisionWorkspace
        v-if="current"
        :key="current.card.key"
        :card="current.card"
        :list-state="current.state"
        :label="current.label"
        :left-ref="current.file.leftRef"
        :right-ref="current.file.rightRef"
        :draft="drafts[current.card.key]"
        :manual-digest="manualDigest(current.card, manualOf(project.id, current.card.path))"
        @draft="setDraft(project.id, current.card.key, $event)"
        @by-hand="byHand(current.card)"
        @manual="showInManual(current.card)"
        @ask="startAsk(current.card)"
        @decided="advance"
      />
      <AppEmptyState
        v-else
        layout="fill"
        title="Pick a block."
        detail="The blocks that need you are listed on the left."
      />
    </div>

    <AskModelModal
      v-if="askCard"
      :project="project"
      held
      :blocks="askCard.blocks.length"
      :sending="asking"
      :error="askError"
      @submit="sendAsk"
      @close="askCard = null"
    />
  </div>
</template>

<style scoped>
.decisions {
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

.send {
  flex-shrink: 0;
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

.list {
  display: flex;
  flex-direction: column;
  flex-shrink: 0;
  width: 280px;
  min-height: 0;
  border-right: 1px solid var(--color-border-subtle);
  background: var(--color-surface-1);
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

.row-state {
  flex-shrink: 0;
  font-size: var(--text-xs);
  color: var(--color-fg-muted);
}

.row.needs .row-state {
  color: var(--color-fg-accent);
}

.other {
  justify-content: space-between;
  font-size: var(--text-xs);
}

.other-text {
  flex-shrink: 0;
  color: var(--color-fg-muted);
}

.list-foot {
  display: flex;
  flex-direction: column;
  gap: var(--space-1);
  padding: var(--space-2) var(--space-3);
  border-top: 1px solid var(--color-border-subtle);
}

.reply {
  width: 100%;
}

.error {
  margin: 0;
  padding: var(--space-1) var(--space-3);
  font-size: var(--text-xs);
  color: var(--color-error);
}
</style>
