<!-- SPDX-License-Identifier: MPL-2.0 -->
<!-- Copyright (C) 2026 The Lemongrass Authors -->

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { AppButton, AppInput, AppTextarea } from '@sankasten'
import { draftFits, type BlockCard, type Draft } from '../../conflictDecisions'
import BlockSides from './BlockSides.vue'
import ConflictIcon from './ConflictIcon.vue'

// One block the human decides. A take or an edit offers only Accept and Reject; Reject asks for a
// sentence to the model, or to resolve the block by hand. A question offers the model's options and
// an answer in the human's own words. Nothing is sent from here: drafts wait for the file's Apply.
const props = defineProps<{
  card: BlockCard
  leftRef: string
  rightRef: string
  draft?: Draft
  // The digest of the lines the human resolved this block to in Manual, not sent yet.
  manualDigest: string | null
  // Done and waiting cards show less, since nothing on them needs the human.
  compact?: boolean
}>()

const emit = defineEmits<{
  draft: [draft: Draft | null]
  byHand: []
  manual: []
}>()

const fileName = computed(() => props.card.path.slice(props.card.path.lastIndexOf('/') + 1))
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
      return 'Not reported yet'
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
  answer(answered.value === option ? '' : option)
}

// Typing replaces a picked option; clearing the words clears an answer that was typed, never a
// picked option.
watch(ownAnswer, (text) => {
  if (text.trim()) answer(text)
  else if (answered.value && !props.card.report?.options.includes(answered.value)) answer('')
})
</script>

<template>
  <article class="card" :class="[tone, { compact }]">
    <header class="head">
      <span class="file" :title="card.path">{{ fileName }}</span>
      <span class="blocks">{{ blocksLabel }}</span>
      <span class="headline">{{ headline }}</span>
      <span class="spacer" />
      <AppButton
        variant="text"
        size="xs"
        class="open-manual"
        title="Show this block in the three panes"
        @click="emit('manual')"
      >
        <ConflictIcon name="manual" />
        Manual
      </AppButton>
    </header>

    <p v-if="card.report && card.state !== 'waiting' && !compact" class="words">
      {{ card.report.text }}
    </p>
    <p v-if="sentText" class="sent">{{ sentText }}</p>

    <BlockSides
      v-if="!compact"
      :sides="card.sides"
      :hold="card.hold"
      :left-ref="leftRef"
      :right-ref="rightRef"
    />

    <!-- The human's draft on this card, waiting for the file's Apply. -->
    <div v-if="byHandDone" class="drafted">
      <span class="drafted-text">Resolved by you. Goes to the model on Apply.</span>
    </div>
    <div v-else-if="draftKind === 'accept'" class="drafted">
      <ConflictIcon name="accept" class="accepted-mark" />
      <span class="drafted-text">Accepted. Goes to the model on Apply.</span>
      <AppButton variant="text" size="xs" @click="emit('draft', null)">Undo</AppButton>
    </div>
    <div v-else-if="draftKind === 'manual'" class="drafted">
      <span class="drafted-text">You resolve this block by hand.</span>
      <AppButton variant="ghost" size="xs" @click="emit('byHand')">Open in Manual</AppButton>
      <AppButton variant="text" size="xs" @click="emit('draft', null)">Cancel</AppButton>
    </div>

    <!-- A question: the model's options as buttons, or the human's own words. -->
    <div v-else-if="card.state === 'question'" class="answer">
      <div v-if="card.report?.options.length" class="options">
        <AppButton
          v-for="option in card.report.options"
          :key="option"
          variant="ghost"
          size="sm"
          :pressed="answered === option"
          @click="pickOption(option)"
        >
          {{ option }}
        </AppButton>
      </div>
      <AppInput
        v-model="ownAnswer"
        size="sm"
        class="own-answer"
        placeholder="Or answer in your own words"
      />
      <AppButton variant="text" size="xs" class="by-hand" @click="emit('byHand')">
        Resolve myself
      </AppButton>
    </div>

    <!-- Reject: a sentence for the model, or resolving the block by hand. -->
    <div v-else-if="rejecting && canReject" class="reject">
      <AppTextarea
        v-model="rejectText"
        size="sm"
        rows="2"
        class="reject-text"
        placeholder="Tell the model what to change"
      />
      <div class="actions">
        <AppButton variant="ghost" size="sm" @click="emit('byHand')">Resolve myself</AppButton>
        <span class="spacer" />
        <AppButton variant="text" size="sm" @click="cancel">Cancel</AppButton>
      </div>
    </div>

    <div v-else-if="canReject" class="actions">
      <AppButton v-if="canJudge" variant="primary" size="sm" @click="accept">
        <ConflictIcon name="accept" />
        Accept
      </AppButton>
      <AppButton variant="ghost" tone="danger" size="sm" @click="startReject">
        <ConflictIcon name="reject" />
        Reject
      </AppButton>
      <span v-if="card.state === 'mismatch'" class="hint">
        Only Reject, since the file does not hold what the model reported.
      </span>
    </div>

    <div v-else-if="card.state === 'open'" class="actions">
      <AppButton variant="text" size="xs" @click="emit('byHand')">Resolve myself</AppButton>
    </div>
  </article>
</template>

<style scoped>
.card {
  display: flex;
  flex-direction: column;
  gap: var(--space-2);
  padding: var(--space-3);
  border: 1px solid var(--color-border-subtle);
  border-radius: var(--radius-md);
  background: var(--color-surface-1);
  font-size: var(--text-sm);
}

.card.ask {
  border-color: var(--color-sapphire);
}

.card.warn {
  border-color: var(--color-warning);
}

.card.compact {
  gap: var(--space-1);
  padding: var(--space-2) var(--space-3);
  background: var(--color-surface-0);
}

.head {
  display: flex;
  align-items: center;
  gap: var(--space-2);
  min-width: 0;
  font-size: var(--text-xs);
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

.headline {
  flex-shrink: 0;
  font-weight: var(--weight-medium);
  color: var(--color-fg-primary);
}

.card.warn .headline {
  color: var(--color-warning);
}

.card.ask .headline {
  color: var(--color-sapphire);
}

.spacer {
  flex: 1;
}

.open-manual {
  flex-shrink: 0;
}

.words {
  margin: 0;
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

.drafted {
  display: flex;
  align-items: center;
  gap: var(--space-2);
  font-size: var(--text-xs);
  color: var(--color-fg-secondary);
}

.accepted-mark {
  color: var(--color-jade);
}

.answer,
.reject {
  display: flex;
  flex-direction: column;
  gap: var(--space-2);
}

.options {
  display: flex;
  flex-wrap: wrap;
  gap: var(--space-2);
}

.by-hand {
  align-self: flex-start;
}

.reject-text {
  width: 100%;
}

.actions {
  display: flex;
  align-items: center;
  gap: var(--space-2);
}

.hint {
  font-size: var(--text-xs);
  color: var(--color-fg-muted);
}
</style>
