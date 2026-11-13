<!-- SPDX-License-Identifier: MPL-2.0 -->
<!-- Copyright (C) 2026 The Lemongrass Authors -->

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { AppButton, AppInput, AppTextarea } from '@sankasten'
import { draftFits, type BlockCard, type Draft, type ListState } from '../../conflictDecisions'
import BlockSides from './BlockSides.vue'
import ConflictIcon from './ConflictIcon.vue'

// The block the human works on, large, with its action row pinned under it. A take or an edit
// offers Accept as the one primary action and Reject; Reject asks for a sentence to the model, or
// to resolve the block by hand. A question offers the model's options and an answer in the human's
// own words. An untouched block offers asking a model about it. Nothing is sent from here: drafts
// wait for the Send button at the top of Decisions.
const props = defineProps<{
  card: BlockCard
  listState: ListState
  label: string
  leftRef: string
  rightRef: string
  draft?: Draft
  // The digest of the lines the human resolved this block to in Manual, not sent yet.
  manualDigest: string | null
}>()

const emit = defineEmits<{
  draft: [draft: Draft | null]
  byHand: []
  manual: []
  ask: []
  // The human made the decision this block waited for, so the list moves on to the next one.
  decided: []
}>()

const blocksLabel = computed(() => {
  const blocks = props.card.blocks
  if (blocks.length === 1) return `Block ${blocks[0]}`
  if (blocks.length === 2) return `Blocks ${blocks[0]} and ${blocks[1]}`
  return `Blocks ${blocks[0]} to ${blocks[blocks.length - 1]}`
})

const fits = computed(() => draftFits(props.card, props.draft))
const draftKind = computed(() => (fits.value ? props.draft?.kind : undefined))
const byHandDone = computed(
  () =>
    props.manualDigest !== null &&
    !(props.card.state === 'resolved' && props.card.verdict?.digest === props.manualDigest)
)

// What the block holds, in the words a take is judged by.
const takeText = computed(() => {
  const hold = props.card.hold
  if (hold.kind !== 'written') return ''
  if (hold.lines.length === 0) return 'Removes the block'
  const ref = (side: 'left' | 'right'): string => (side === 'left' ? props.leftRef : props.rightRef)
  const sides = hold.sides
  if (!sides) return 'Edited by the model'
  if (sides.length === 1) return `Takes ${ref(sides[0])}`
  return `Takes both, ${ref(sides[0])} first`
})

const headline = computed(() => {
  const card = props.card
  switch (card.state) {
    case 'question':
      return 'Question'
    case 'take':
      return takeText.value
    case 'changed':
      return card.verdict?.verdict === 'resolved'
        ? 'Changed since you resolved it'
        : 'Changed since you accepted it'
    case 'mismatch':
      return card.mismatch === 'markersLeft'
        ? 'Reported, but its markers are still there'
        : 'Changed with no report on it'
    case 'waiting':
      return 'With the model'
    case 'open':
      return props.listState === 'untouched' ? 'Nobody worked on this block' : 'Not reported yet'
    case 'accepted':
      return 'Accepted'
    case 'resolved':
      return 'Resolved by you'
  }
  return ''
})

const tone = computed(() => {
  if (props.card.state === 'mismatch' || props.card.state === 'changed') return 'warn'
  if (props.card.state === 'question') return 'ask'
  return 'plain'
})

// The human's last word when the block is with the model.
const sentText = computed(() => {
  const verdict = props.card.verdict
  if (!verdict || props.card.state !== 'waiting') return ''
  const said = {
    rejected: 'You rejected it',
    answered: 'You answered',
    asked: 'You asked',
    accepted: '',
    resolved: ''
  }[verdict.verdict]
  return said ? `${said}: ${verdict.text}` : ''
})

const canJudge = computed(() => props.card.state === 'take' || props.card.state === 'changed')
const canReject = computed(() => canJudge.value || props.card.state === 'mismatch')

// Rejecting opens a sentence for the model; it is a draft as soon as there are words in it.
const rejecting = ref(false)
const rejectText = ref(props.draft?.kind === 'reject' ? props.draft.text : '')
watch(rejectText, (text) => {
  if (!rejecting.value) return
  emit('draft', text.trim() ? { kind: 'reject', text } : null)
})
watch(draftKind, (kind) => {
  if (kind === 'reject') rejecting.value = true
})

function accept(): void {
  emit('draft', { kind: 'accept', digest: props.card.digest })
  emit('decided')
}

function startReject(): void {
  rejecting.value = true
}

function cancel(): void {
  rejecting.value = false
  rejectText.value = ''
  emit('draft', null)
}

// A question's answer: one of the model's options, or the human's own words.
const ownAnswer = ref(
  props.draft?.kind === 'answer' && !props.card.report?.options.includes(props.draft.text)
    ? props.draft.text
    : ''
)
const answered = computed(() =>
  draftKind.value === 'answer' && props.draft?.kind === 'answer' ? props.draft.text : ''
)

function answer(text: string): void {
  const question = props.card.report?.id
  if (!question) return
  emit('draft', text.trim() ? { kind: 'answer', text, question } : null)
}

function pickOption(option: string): void {
  ownAnswer.value = ''
  const picked = answered.value !== option
  answer(picked ? option : '')
  if (picked) emit('decided')
}

// Typing replaces a picked option; clearing the words clears an answer that was typed, never a
// picked option.
watch(ownAnswer, (text) => {
  if (text.trim()) answer(text)
  else if (answered.value && !props.card.report?.options.includes(answered.value)) answer('')
})
</script>

<template>
  <section class="workspace">
    <header class="head">
      <span class="file" :title="card.path">{{ card.path }}</span>
      <span class="blocks">{{ blocksLabel }}</span>
      <span class="state" :class="listState">{{ label }}</span>
      <span class="spacer" />
      <AppButton
        variant="text"
        size="sm"
        title="Show this block in the three panes"
        @click="emit('manual')"
      >
        <ConflictIcon name="manual" />
        Open in Manual
      </AppButton>
    </header>

    <div class="scroll">
      <h3 class="headline" :class="tone">{{ headline }}</h3>
      <p v-if="card.report && card.state !== 'waiting'" class="words">{{ card.report.text }}</p>
      <p v-if="sentText" class="sent">{{ sentText }}</p>
      <p v-if="listState === 'untouched' && card.state === 'open'" class="note">
        No model was asked to work on this block. Ask a model about it, or resolve it yourself.
      </p>
      <p v-else-if="card.state === 'open'" class="note">
        The model has not reported this block yet.
      </p>

      <BlockSides
        :sides="card.sides"
        :hold="card.hold"
        :left-ref="leftRef"
        :right-ref="rightRef"
        expanded
        :no-result="byHandDone"
      />
    </div>

    <footer class="actions">
      <template v-if="byHandDone">
        <span class="note">You resolved this block in Manual. It is sent when you press Send.</span>
        <span class="spacer" />
        <AppButton variant="ghost" size="sm" @click="emit('byHand')">Open in Manual</AppButton>
      </template>

      <template v-else-if="draftKind === 'accept'">
        <ConflictIcon name="accept" class="accepted-mark" />
        <span class="note">Accepted. It is sent when you press Send.</span>
        <span class="spacer" />
        <AppButton variant="ghost" size="sm" @click="emit('draft', null)">Undo</AppButton>
      </template>

      <template v-else-if="draftKind === 'manual'">
        <span class="note">You resolve this block by hand.</span>
        <span class="spacer" />
        <AppButton variant="primary" size="sm" @click="emit('byHand')">Open in Manual</AppButton>
        <AppButton variant="text" size="sm" @click="emit('draft', null)">Cancel</AppButton>
      </template>

      <!-- A question: the model's options as buttons, or the human's own words. -->
      <div v-else-if="card.state === 'question'" class="answer">
        <div v-if="card.report?.options.length" class="options">
          <AppButton
            v-for="option in card.report.options"
            :key="option"
            variant="primary"
            size="sm"
            :pressed="answered === option"
            @click="pickOption(option)"
          >
            {{ option }}
          </AppButton>
        </div>
        <div class="own">
          <AppInput
            v-model="ownAnswer"
            size="sm"
            class="own-answer"
            placeholder="Or answer in your own words"
          />
          <AppButton variant="text" size="sm" @click="emit('byHand')">Resolve myself</AppButton>
        </div>
      </div>

      <!-- Reject: a sentence for the model, or resolving the block by hand. -->
      <div v-else-if="rejecting && canReject" class="answer">
        <AppTextarea
          v-model="rejectText"
          size="sm"
          rows="2"
          class="reject-text"
          placeholder="Tell the model what to change"
        />
        <div class="row">
          <AppButton variant="ghost" size="sm" @click="emit('byHand')">Resolve myself</AppButton>
          <span class="spacer" />
          <AppButton variant="text" size="sm" @click="cancel">Cancel</AppButton>
        </div>
      </div>

      <template v-else-if="canReject">
        <AppButton v-if="canJudge" variant="primary" size="sm" @click="accept">
          <ConflictIcon name="accept" />
          Accept
        </AppButton>
        <AppButton variant="ghost" tone="danger" size="sm" @click="startReject">
          <ConflictIcon name="reject" />
          Reject
        </AppButton>
        <AppButton variant="text" size="sm" @click="emit('byHand')">Resolve myself</AppButton>
        <span v-if="card.state === 'mismatch'" class="note">
          Only Reject, since the file does not hold what the model reported.
        </span>
      </template>

      <template v-else-if="listState === 'untouched'">
        <AppButton variant="primary" size="sm" @click="emit('ask')">
          <ConflictIcon name="ask" />
          Ask model
        </AppButton>
        <AppButton variant="text" size="sm" @click="emit('byHand')">Resolve myself</AppButton>
      </template>

      <template v-else-if="card.state === 'open' || card.state === 'waiting'">
        <span class="note">Nothing for you to do here while the model works.</span>
        <span class="spacer" />
        <AppButton variant="text" size="sm" @click="emit('byHand')">Resolve myself</AppButton>
      </template>

      <span v-else class="note">{{ headline }}.</span>
    </footer>
  </section>
</template>

<style scoped>
.workspace {
  display: flex;
  flex-direction: column;
  flex: 1;
  min-width: 0;
  min-height: 0;
}

.head {
  display: flex;
  align-items: center;
  gap: var(--space-3);
  min-width: 0;
  padding: var(--space-2) var(--space-4);
  border-bottom: 1px solid var(--color-border-subtle);
  font-size: var(--text-sm);
}

.file {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-family: var(--font-mono);
  color: var(--color-fg-primary);
}

.blocks {
  flex-shrink: 0;
  color: var(--color-fg-muted);
}

.state {
  flex-shrink: 0;
  font-size: var(--text-xs);
  font-weight: var(--weight-medium);
  color: var(--color-fg-muted);
}

.state.needs {
  color: var(--color-fg-accent);
}

.state.queued,
.state.done {
  color: var(--color-jade);
}

.state.model {
  color: var(--color-violet);
}

.spacer {
  flex: 1;
}

.scroll {
  display: flex;
  flex-direction: column;
  gap: var(--space-3);
  flex: 1;
  min-height: 0;
  overflow-y: auto;
  padding: var(--space-4);
}

.headline {
  margin: 0;
  font-size: var(--text-lg);
  font-weight: var(--weight-semibold);
  color: var(--color-fg-primary);
}

.headline.warn {
  color: var(--color-warning);
}

.headline.ask {
  color: var(--color-info);
}

.words {
  margin: 0;
  font-size: var(--text-sm);
  color: var(--color-fg-primary);
  white-space: pre-wrap;
  word-break: break-word;
}

.sent {
  margin: 0;
  font-size: var(--text-xs);
  color: var(--color-fg-secondary);
  white-space: pre-wrap;
  word-break: break-word;
}

.note {
  margin: 0;
  font-size: var(--text-xs);
  color: var(--color-fg-muted);
}

.actions {
  display: flex;
  align-items: center;
  gap: var(--space-2);
  min-width: 0;
  padding: var(--space-3) var(--space-4);
  border-top: 1px solid var(--color-border-subtle);
  background: var(--color-surface-1);
}

.accepted-mark {
  color: var(--color-success);
}

.answer {
  display: flex;
  flex: 1;
  flex-direction: column;
  gap: var(--space-2);
  min-width: 0;
}

.options,
.row,
.own {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: var(--space-2);
}

.own-answer {
  flex: 1;
}

.reject-text {
  width: 100%;
}
</style>
