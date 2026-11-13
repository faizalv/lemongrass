<!-- SPDX-License-Identifier: MPL-2.0 -->
<!-- Copyright (C) 2026 The Lemongrass Authors -->

<script setup lang="ts">
import { computed, ref } from 'vue'
import { AppButton, AppDisclosure, AppEmptyState, AppInput } from '@sankasten'
import { postToConflict, type ConflictThreadState } from '../../conflictThread'
import { useFileDecisions } from '../../conflictFiles'
import {
  generalLine,
  manualDigest,
  roundDone,
  type BlockCard,
  type CardState,
  type FileDecisions
} from '../../conflictDecisions'
import {
  applyFile,
  draftsOf,
  focusBlock,
  manualOf,
  pendingFor,
  resolveByHand,
  setDraft
} from '../../conflictVerdicts'
import { tabParts, type NamePart } from '../../tabNames'
import type { ProjectRef } from '../../workspace'
import DecisionCard from './DecisionCard.vue'
import ConflictModeSwitch from './ConflictModeSwitch.vue'
import MarkedText from '../MarkedText.vue'

// Decisions: what the human has to decide, across every conflicted file. The model's questions
// come first, then its takes to review, then what waits for the model, then what is done. The
// conversation itself is never shown: the model's latest plain message is one line on top, and the
// human answers it in one line.
const props = defineProps<{
  project: ProjectRef
  thread: ConflictThreadState
  files: string[]
}>()

// manual opens a file in Manual, or the file the tab last showed when path is null.
const emit = defineEmits<{ manual: [path: string | null] }>()

const decisions = useFileDecisions(() => props.project)
const drafts = computed(() => draftsOf(props.project.id))

interface Placed {
  card: BlockCard
  file: FileDecisions
}

const placed = computed((): Placed[] =>
  decisions.value.flatMap((file) => (file.cards ?? []).map((card) => ({ card, file })))
)

function inStates(states: CardState[]): Placed[] {
  return placed.value.filter(({ card }) => states.includes(card.state))
}

const questions = computed(() => inStates(['question']))
const review = computed(() => inStates(['take', 'changed', 'mismatch']))
const waiting = computed(() => inStates(['waiting', 'open']))
const done = computed(() => inStates(['accepted', 'resolved']))
const noBlocks = computed(() => decisions.value.filter((file) => file.cards === null))
// Lines changed outside every block are staged with the file but no block report covers them.
const outside = computed(() => decisions.value.filter((file) => file.editsOutside > 0))
const doneOpen = ref(false)

// Who holds the conflict, highlighted since its title is the model's own words.
const holder = computed((): NamePart[] => {
  const tabId = props.thread.status?.assignee
  return tabId ? tabParts(tabId) : [{ text: 'a model' }]
})
const statusParts = computed((): NamePart[] =>
  roundDone(props.thread.messages)
    ? [...holder.value, { text: ' finished its round.' }]
    : [{ text: 'Handed to ' }, ...holder.value, { text: '.' }]
)

const line = computed(() => generalLine(props.thread.messages))
const reply = ref('')
const replying = ref(false)
const replyError = ref<string | null>(null)

async function sendReply(): Promise<void> {
  const body = reply.value.trim()
  if (!body || replying.value) return
  replying.value = true
  replyError.value = await postToConflict(props.project, [{ body, anchor: null }])
  replying.value = false
  if (!replyError.value) reply.value = ''
}

function byHand(card: BlockCard): void {
  resolveByHand(props.project.id, card)
  emit('manual', card.path)
}

function showInManual(card: BlockCard): void {
  focusBlock(props.project.id, card.path, card.blocks[0])
  emit('manual', card.path)
}

// Files with something to send on Apply, and what Apply would do with each.
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

function summary(verdicts: { verdict: string; block: number }[], settled: boolean): string {
  const counts = new Map<string, Set<number>>()
  for (const v of verdicts) {
    if (!counts.has(v.verdict)) counts.set(v.verdict, new Set())
    counts.get(v.verdict)!.add(v.block)
  }
  const words: Record<string, string> = {
    accepted: 'accepted',
    rejected: 'rejected',
    resolved: 'resolved by you',
    answered: 'answered'
  }
  const parts = [...counts].map(([kind, blocks]) => `${blocks.size} ${words[kind] ?? kind}`)
  if (settled) parts.push('stages the file')
  return parts.join(', ')
}

const applying = ref<string | null>(null)
const applyErrors = ref<Record<string, string>>({})

async function apply(file: FileDecisions): Promise<void> {
  applying.value = file.path
  const refused = await applyFile(props.project, file)
  applying.value = null
  const errors = { ...applyErrors.value }
  if (refused) errors[file.path] = refused
  else delete errors[file.path]
  applyErrors.value = errors
}

function fileName(path: string): string {
  return path.slice(path.lastIndexOf('/') + 1)
}
</script>

<template>
  <div class="decisions">
    <header class="top">
      <ConflictModeSwitch mode="decisions" @switch="emit('manual', null)" />
      <MarkedText class="holder" :parts="statusParts" capitalize />
    </header>

    <!-- The one line the model writes to the human, and the one line back. -->
    <div class="general">
      <p v-if="line" class="general-line" :title="line.body">
        <span class="general-author">Model</span>
        {{ line.body }}
      </p>
      <AppInput
        v-model="reply"
        size="sm"
        class="general-reply"
        :placeholder="
          line ? 'Reply in one line' : 'Tell the model something about the whole conflict'
        "
        :disabled="replying"
        @keydown.enter.prevent="sendReply"
      />
      <p v-if="replyError" class="error">{{ replyError }}</p>
    </div>

    <div class="body">
      <AppEmptyState
        v-if="placed.length === 0 && noBlocks.length === 0"
        layout="fill"
        :loading="decisions.length < files.length"
        title="Nothing to decide yet."
        detail="Each block the model reports shows here."
      />

      <section v-if="questions.length" class="group">
        <h3 class="group-title">Your answer needed</h3>
        <DecisionCard
          v-for="{ card, file } in questions"
          :key="card.key"
          :card="card"
          :left-ref="file.leftRef"
          :right-ref="file.rightRef"
          :draft="drafts[card.key]"
          :manual-digest="manualDigest(card, manualOf(project.id, card.path))"
          @draft="setDraft(project.id, card.key, $event)"
          @by-hand="byHand(card)"
          @manual="showInManual(card)"
        />
      </section>

      <section v-if="review.length" class="group">
        <h3 class="group-title">To review</h3>
        <DecisionCard
          v-for="{ card, file } in review"
          :key="card.key"
          :card="card"
          :left-ref="file.leftRef"
          :right-ref="file.rightRef"
          :draft="drafts[card.key]"
          :manual-digest="manualDigest(card, manualOf(project.id, card.path))"
          @draft="setDraft(project.id, card.key, $event)"
          @by-hand="byHand(card)"
          @manual="showInManual(card)"
        />
      </section>

      <section v-if="waiting.length" class="group">
        <h3 class="group-title">With the model</h3>
        <DecisionCard
          v-for="{ card, file } in waiting"
          :key="card.key"
          :card="card"
          :left-ref="file.leftRef"
          :right-ref="file.rightRef"
          :draft="drafts[card.key]"
          :manual-digest="manualDigest(card, manualOf(project.id, card.path))"
          compact
          @draft="setDraft(project.id, card.key, $event)"
          @by-hand="byHand(card)"
          @manual="showInManual(card)"
        />
      </section>

      <section v-if="outside.length" class="group">
        <h3 class="group-title">Changed outside the blocks</h3>
        <div v-for="file in outside" :key="file.path" class="no-blocks">
          <span class="no-blocks-file" :title="file.path">{{ fileName(file.path) }}</span>
          <span class="no-blocks-text">
            {{ file.editsOutside === 1 ? '1 line' : `${file.editsOutside} lines` }} changed outside
            the blocks. Apply stages them with the file.
          </span>
          <AppButton variant="text" size="xs" @click="emit('manual', file.path)">Manual</AppButton>
        </div>
      </section>

      <section v-if="noBlocks.length" class="group">
        <h3 class="group-title">No blocks to decide</h3>
        <div v-for="file in noBlocks" :key="file.path" class="no-blocks">
          <span class="no-blocks-file" :title="file.path">{{ fileName(file.path) }}</span>
          <span class="no-blocks-text">Binary, or deleted on one side. Pick a side in Manual.</span>
          <AppButton variant="text" size="xs" @click="emit('manual', file.path)">Manual</AppButton>
        </div>
      </section>

      <section v-if="done.length" class="group">
        <AppDisclosure v-model:open="doneOpen" block label="Done" :count="done.length" />
        <template v-if="doneOpen">
          <DecisionCard
            v-for="{ card, file } in done"
            :key="card.key"
            :card="card"
            :left-ref="file.leftRef"
            :right-ref="file.rightRef"
            :draft="drafts[card.key]"
            :manual-digest="manualDigest(card, manualOf(project.id, card.path))"
            compact
            @draft="setDraft(project.id, card.key, $event)"
            @by-hand="byHand(card)"
            @manual="showInManual(card)"
          />
        </template>
      </section>
    </div>

    <!-- Verdicts go to the model one file at a time, together, on that file's Apply. -->
    <footer v-if="ready.length" class="apply-bar">
      <div v-for="item in ready" :key="item.file.path" class="apply-row">
        <span class="apply-file" :title="item.file.path">{{ fileName(item.file.path) }}</span>
        <span class="apply-summary">{{ summary(item.verdicts, item.settled) }}</span>
        <span v-if="applyErrors[item.file.path]" class="error">{{
          applyErrors[item.file.path]
        }}</span>
        <span class="spacer" />
        <AppButton
          variant="primary"
          size="sm"
          :disabled="applying !== null"
          :title="
            item.settled
              ? 'Send the verdicts and stage the file'
              : 'Send the verdicts on this file to the model'
          "
          @click="apply(item.file)"
        >
          Apply
        </AppButton>
      </div>
    </footer>
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

.holder {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-size: var(--text-xs);
  color: var(--color-fg-secondary);
}

.general {
  display: flex;
  flex-direction: column;
  gap: var(--space-1);
  padding: var(--space-2) var(--space-3);
  border-bottom: 1px solid var(--color-border-subtle);
}

.general-line {
  margin: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-size: var(--text-sm);
  color: var(--color-fg-primary);
}

.general-author {
  margin-right: var(--space-1);
  font-weight: var(--weight-medium);
  color: var(--color-fg-secondary);
}

.general-reply {
  width: 100%;
}

.body {
  display: flex;
  flex-direction: column;
  gap: var(--space-4);
  flex: 1;
  min-height: 0;
  overflow-y: auto;
  padding: var(--space-3);
}

.group {
  display: flex;
  flex-direction: column;
  gap: var(--space-2);
}

.group-title {
  margin: 0;
  font-size: var(--text-xs);
  font-weight: var(--weight-medium);
  color: var(--color-fg-secondary);
}

.no-blocks {
  display: flex;
  align-items: center;
  gap: var(--space-2);
  font-size: var(--text-xs);
}

.no-blocks-file {
  font-family: var(--font-mono);
  color: var(--color-fg-primary);
}

.no-blocks-text {
  color: var(--color-fg-muted);
}

.apply-bar {
  display: flex;
  flex-direction: column;
  gap: var(--space-1);
  padding: var(--space-2) var(--space-3);
  border-top: 1px solid var(--color-border-subtle);
  background: var(--color-surface-1);
}

.apply-row {
  display: flex;
  align-items: center;
  gap: var(--space-2);
  min-width: 0;
  font-size: var(--text-xs);
}

.apply-file {
  font-family: var(--font-mono);
  color: var(--color-fg-primary);
}

.apply-summary {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  color: var(--color-fg-secondary);
}

.spacer {
  flex: 1;
}

.error {
  margin: 0;
  font-size: var(--text-xs);
  color: var(--color-error);
}
</style>
