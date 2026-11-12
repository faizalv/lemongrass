<!-- SPDX-License-Identifier: MPL-2.0 -->
<!-- Copyright (C) 2026 The Lemongrass Authors -->

<script setup lang="ts">
import {
  AppButton,
  AppChip,
  AppEmptyState,
  AppIconButton,
  AppSwitcher,
  AppTextarea
} from '@sankasten'
import { computed, inject, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { clearReview, removeAnnotation, reviewOf, setReviewMessage } from '../../annotations'
import {
  CHAT_MIN_HEIGHT,
  conflictChat,
  loadConflictChat,
  setConflictChatHeight,
  setConflictChatMinimized
} from '../../conflictChatSettings'
import { conflictJumpKey } from '../../conflictJump'
import { chipText, firstLine, startsRun } from '../../conflictThreadModel'
import { handConflictTo, postToConflict, type ConflictThreadState } from '../../conflictThread'
import { bodyParts, type BodyPart } from '../../mentions'
import { resolveTargetsOf } from '../../reviewTargets'
import { shellTitles } from '../../shellRegistry'
import type { ProjectRef } from '../../workspace'
import type {
  ConflictHumanMessage,
  ConflictMessageAnchor,
  TabBusy,
  WorkgroupThreadMessage
} from '../../../../preload/types'

const props = defineProps<{
  project: ProjectRef
  scope: string
  thread: ConflictThreadState
}>()

// Each poll runs lgrassd once, so this is as often as the picker can afford.
const BUSY_POLL_MS = 2500

const review = computed(() => reviewOf(props.project.id, props.scope))
const count = computed(() => review.value.annotations.length)
// The tab provides the jump; a chat shown anywhere else has nothing to open.
const jump = inject(conflictJumpKey, () => {})
// The anchored messages whose quoted lines are unfolded under their chip.
const expanded = ref(new Set<number>())
// A reply's attachment scrolls to the message it answers and lights it for a moment.
const lit = ref<number | null>(null)
const open = computed(() => !!props.thread.status && !props.thread.closed)
const assignee = computed(() => props.thread.status?.assignee ?? '')
const busy = ref<Record<string, TabBusy>>({})
const targetId = ref<string | null>(null)
const sending = ref(false)
const error = ref<string | null>(null)
const scroller = ref<HTMLElement | null>(null)
const root = ref<HTMLElement | null>(null)

const targets = computed(() =>
  resolveTargetsOf(props.project.id).map((target) => ({
    id: target.tabId,
    label: target.label,
    disabled: busy.value[target.tabId]?.busy
      ? (busy.value[target.tabId].reason ?? 'Busy')
      : undefined
  }))
)
const target = computed(() => targets.value.find((t) => t.id === targetId.value) ?? null)

const text = computed({
  get: () => review.value.message,
  set: (value: string) => setReviewMessage(props.project.id, props.scope, value)
})

const blocker = computed(() => {
  if (count.value === 0 && !review.value.message.trim()) {
    return 'Write a message or add a line comment first.'
  }
  if (open.value) return null
  if (!target.value) return 'Pick a tab to hand the conflict to.'
  return target.value.disabled ?? null
})

function labelOf(message: WorkgroupThreadMessage): string {
  if (message.author === 'human') return 'You'
  return shellTitles[message.tabId] ?? message.label
}

function linesOf(anchor: ConflictMessageAnchor): string {
  return anchor.startLine === anchor.endLine
    ? `${anchor.path}, line ${anchor.startLine}`
    : `${anchor.path}, lines ${anchor.startLine} to ${anchor.endLine}`
}

// A file chip of a submit opens the file at its first conflict, since it names no lines.
function fileAnchor(path: string): ConflictMessageAnchor {
  return { path, startLine: 0, endLine: 0, quote: '' }
}

function toggleQuote(id: number): void {
  const next = new Set(expanded.value)
  if (next.has(id)) next.delete(id)
  else next.add(id)
  expanded.value = next
}

const byId = computed(() => new Map(props.thread.messages.map((m) => [m.id, m])))

function answered(message: WorkgroupThreadMessage): WorkgroupThreadMessage | null {
  return message.replyTo ? (byId.value.get(message.replyTo) ?? null) : null
}

async function showMessage(id: number): Promise<void> {
  const el = scroller.value?.querySelector<HTMLElement>(`[data-message="${id}"]`)
  if (!el) return
  el.scrollIntoView({ block: 'center', behavior: 'smooth' })
  lit.value = id
  setTimeout(() => {
    if (lit.value === id) lit.value = null
  }, 1200)
}

// Dragging the top edge sets the chat's height, kept for every conflict tab. The panes above keep
// at least as much room as the chat's own minimum.
function startResize(event: PointerEvent): void {
  const start = conflictChat.height
  const startY = event.clientY
  const host = root.value?.parentElement
  const max = Math.max(CHAT_MIN_HEIGHT, (host?.clientHeight ?? 800) - CHAT_MIN_HEIGHT)
  const target = event.currentTarget as HTMLElement
  target.setPointerCapture(event.pointerId)
  const move = (e: PointerEvent): void => {
    conflictChat.height = Math.min(max, Math.max(CHAT_MIN_HEIGHT, start + (startY - e.clientY)))
  }
  const up = (): void => {
    target.removeEventListener('pointermove', move)
    target.removeEventListener('pointerup', up)
    setConflictChatHeight(conflictChat.height)
  }
  target.addEventListener('pointermove', move)
  target.addEventListener('pointerup', up)
}

function partsOf(body: string): BodyPart[] {
  const members = assignee.value
    ? [{ tabId: assignee.value, label: shellTitles[assignee.value] ?? 'the model' }]
    : []
  return bodyParts(body, members)
}

function formatTime(value: string): string {
  const date = new Date(value)
  return Number.isNaN(date.getTime()) ? value : date.toLocaleTimeString()
}

function nearBottom(): boolean {
  const el = scroller.value
  return !el || el.scrollHeight - el.scrollTop - el.clientHeight < 40
}

// New messages scroll into view unless the human is reading back.
watch(
  () => props.thread.messages.length,
  async () => {
    const stick = nearBottom()
    await nextTick()
    if (stick && scroller.value) scroller.value.scrollTop = scroller.value.scrollHeight
  }
)

let pollTimer: ReturnType<typeof setInterval> | null = null

async function pollBusy(): Promise<void> {
  try {
    busy.value = await window.api.tabSessions.states(props.project.path)
  } catch {
    busy.value = {}
  }
}

// The picker only shows while no thread is open, so busy states are polled only then.
watch(
  open,
  (isOpen) => {
    if (pollTimer) clearInterval(pollTimer)
    pollTimer = null
    if (isOpen) return
    void pollBusy()
    pollTimer = setInterval(pollBusy, BUSY_POLL_MS)
  },
  { immediate: true }
)

onMounted(async () => {
  void loadConflictChat()
  await nextTick()
  if (scroller.value) scroller.value.scrollTop = scroller.value.scrollHeight
})

onBeforeUnmount(() => {
  if (pollTimer) clearInterval(pollTimer)
})

// Line comments stay drafts until Send, then go out as anchored messages, the text last.
function outgoing(): ConflictHumanMessage[] {
  const messages: ConflictHumanMessage[] = review.value.annotations.map((annotation) => ({
    body: annotation.comment,
    anchor: {
      path: annotation.anchor.path,
      startLine: annotation.anchor.startLine,
      endLine: annotation.anchor.endLine,
      quote: annotation.quote
    }
  }))
  const overall = review.value.message.trim()
  if (overall) messages.push({ body: overall, anchor: null })
  return messages
}

async function send(): Promise<void> {
  if (blocker.value || sending.value) return
  sending.value = true
  error.value = null
  try {
    if (!open.value) {
      if (!targetId.value) return
      const refused = await handConflictTo(props.project, targetId.value)
      if (refused) {
        error.value = refused
        return
      }
    }
    const refused = await postToConflict(props.project, outgoing())
    if (refused) error.value = refused
    else clearReview(props.project.id, props.scope)
  } finally {
    sending.value = false
  }
}

function onKeydown(event: KeyboardEvent): void {
  if (event.key === 'Enter' && (event.ctrlKey || event.metaKey)) {
    event.preventDefault()
    void send()
  }
}
</script>

<template>
  <div
    ref="root"
    class="thread-bar"
    :class="{ minimized: conflictChat.minimized }"
    :style="conflictChat.minimized ? undefined : { height: `${conflictChat.height}px` }"
  >
    <div
      v-if="!conflictChat.minimized"
      class="resize"
      title="Drag to resize the conflict chat"
      @pointerdown.prevent="startResize"
    />
    <div class="bar-head">
      <span class="title">Conflict thread</span>
      <span v-if="open" class="head-note">
        with <span class="head-name">{{ shellTitles[assignee] ?? 'a tab that is closed' }}</span>
      </span>
      <span v-else class="head-note">Not handed to a tab yet</span>
      <span class="spacer" />
      <AppIconButton
        :icon="conflictChat.minimized ? 'chevronDown' : 'minus'"
        size="sm"
        :label="conflictChat.minimized ? 'Show the conflict chat' : 'Minimize the conflict chat'"
        :pressed="conflictChat.minimized"
        @click="setConflictChatMinimized(!conflictChat.minimized)"
      />
    </div>

    <template v-if="!conflictChat.minimized">
      <div v-if="open" ref="scroller" class="messages">
        <AppEmptyState v-if="thread.messages.length === 0" size="sm" title="No messages yet." />
        <template v-for="(message, index) in thread.messages" :key="message.id">
          <div v-if="message.kind === 'submit'" class="event" :data-message="message.id">
            <span>{{ labelOf(message) }} submitted</span>
            <AppChip
              v-for="file in message.files"
              :key="file"
              mono
              class="chip"
              tabindex="0"
              :title="`Open ${file}`"
              @click="jump(fileAnchor(file))"
              @keydown.enter="jump(fileAnchor(file))"
              >{{ file.split('/').pop() }}</AppChip
            >
            <time class="time">{{ formatTime(message.createdAt) }}</time>
          </div>
          <div
            v-else
            class="row"
            :class="[message.author === 'human' ? 'mine' : 'theirs', { lit: lit === message.id }]"
            :data-message="message.id"
          >
            <span v-if="startsRun(thread.messages, index)" class="author">{{
              labelOf(message)
            }}</span>
            <div class="bubble">
              <button
                v-if="answered(message)"
                type="button"
                class="answers"
                :title="'Show the message this answers'"
                @click="showMessage(answered(message)!.id)"
              >
                <span class="answers-author">{{ labelOf(answered(message)!) }}</span>
                {{ firstLine(answered(message)!.body) }}
              </button>
              <p class="body">
                <template v-for="(part, at) in partsOf(message.body)" :key="at">
                  <span :class="{ mention: part.mention }">{{ part.text }}</span>
                </template>
              </p>
              <div v-if="message.anchor" class="attachment">
                <AppChip
                  mono
                  class="chip"
                  tabindex="0"
                  :title="`Open ${linesOf(message.anchor)}`"
                  @click="jump(message.anchor)"
                  @keydown.enter="jump(message.anchor)"
                  >{{ chipText(message.anchor) }}</AppChip
                >
                <AppIconButton
                  v-if="message.anchor.quote"
                  :icon="expanded.has(message.id) ? 'chevronDown' : 'chevronRight'"
                  size="sm"
                  :label="
                    expanded.has(message.id) ? 'Hide the quoted lines' : 'Show the quoted lines'
                  "
                  @click="toggleQuote(message.id)"
                />
              </div>
              <pre v-if="message.anchor && expanded.has(message.id)" class="quote">{{
                message.anchor.quote
              }}</pre>
              <time class="time">{{ formatTime(message.createdAt) }}</time>
            </div>
          </div>
        </template>
      </div>
      <div v-else class="messages empty">
        <AppEmptyState
          size="sm"
          title="Not handed to a tab yet."
          detail="Pick a tab below and send, and the model resolves the conflict with you here."
        />
      </div>
      <p v-if="thread.error" class="error">{{ thread.error }}</p>

      <div v-if="count > 0" class="drafts">
        <AppChip
          v-for="annotation in review.annotations"
          :key="annotation.id"
          mono
          removable
          remove-label="Drop this line comment"
          class="chip"
          tabindex="0"
          :title="`${annotation.comment}\n${linesOf({ ...annotation.anchor, quote: annotation.quote })}`"
          @click="jump({ ...annotation.anchor, quote: annotation.quote })"
          @remove="removeAnnotation(project.id, scope, annotation.id)"
          >{{ chipText({ path: annotation.anchor.path, quote: annotation.quote }) }}</AppChip
        >
      </div>
      <div class="composer">
        <AppTextarea
          v-model="text"
          class="composer-text"
          size="sm"
          resize="none"
          rows="2"
          spellcheck="false"
          :placeholder="open ? 'Reply in the conflict thread' : 'What should the model know?'"
          :disabled="sending"
          @keydown="onKeydown"
        />
        <div v-if="!open" class="picker">
          <AppSwitcher
            :items="targets"
            :active-id="targetId"
            placeholder="Hand to tab"
            empty-text="No Claude or Codex tab is open in this project."
            framed
            size="sm"
            above
            @select="targetId = $event"
          />
        </div>
        <AppButton
          variant="primary"
          size="sm"
          class="send"
          :disabled="!!blocker || sending"
          :title="
            blocker ?? (open ? 'Send to the conflict thread' : 'Hand the conflict over and send')
          "
          @click="send"
        >
          {{ sending ? 'Sending' : 'Send' }}
        </AppButton>
      </div>
      <p v-if="error" class="error">{{ error }}</p>
    </template>
  </div>
</template>

<style scoped>
.thread-bar {
  position: relative;
  display: flex;
  flex-direction: column;
  flex-shrink: 0;
  gap: var(--space-2);
  min-height: 0;
  padding: var(--space-2) var(--space-3);
  background: var(--color-surface-1);
  border-top: 1px solid var(--color-border-default);
  font-size: var(--text-xs);
}

.resize {
  position: absolute;
  top: -3px;
  right: 0;
  left: 0;
  height: 6px;
  cursor: row-resize;
}

.bar-head {
  display: flex;
  align-items: center;
  gap: var(--space-2);
}

.title {
  color: var(--color-fg-primary);
  font-weight: var(--weight-medium);
}

.head-note {
  color: var(--color-fg-muted);
}

.head-name {
  color: var(--color-fg-primary);
}

.spacer {
  flex: 1;
}

.messages {
  display: flex;
  flex: 1;
  flex-direction: column;
  gap: var(--space-2);
  min-height: 0;
  overflow-y: auto;
  padding-right: var(--space-1);
}

.messages.empty {
  justify-content: center;
}

.row {
  display: flex;
  flex-direction: column;
  gap: 2px;
  max-width: 78%;
}

.row.theirs {
  align-self: flex-start;
}

.row.mine {
  align-self: flex-end;
  align-items: flex-end;
}

.author {
  color: var(--color-fg-secondary);
  font-weight: var(--weight-medium);
}

.bubble {
  display: flex;
  flex-direction: column;
  gap: var(--space-1);
  padding: var(--space-1) var(--space-2);
  border: 1px solid var(--color-border-default);
  border-radius: var(--radius-md);
  background: var(--color-surface-0);
  transition: background-color 0.3s;
}

.row.mine .bubble {
  border-color: transparent;
  background: var(--color-surface-2);
}

.row.lit .bubble {
  background: var(--color-amber-muted);
}

.body {
  margin: 0;
  color: var(--color-fg-primary);
  white-space: pre-wrap;
  word-break: break-word;
}

.body .mention {
  color: var(--color-fg-accent);
}

.answers {
  display: block;
  max-width: 100%;
  overflow: hidden;
  padding: 2px var(--space-2);
  border: none;
  border-left: 2px solid var(--color-fg-muted);
  border-radius: var(--radius-sm);
  background: var(--color-surface-1);
  color: var(--color-fg-secondary);
  font: inherit;
  text-align: left;
  text-overflow: ellipsis;
  white-space: nowrap;
  cursor: pointer;
}

.answers:hover {
  color: var(--color-fg-primary);
}

.answers-author {
  font-weight: var(--weight-medium);
}

.attachment {
  display: flex;
  align-items: center;
  gap: var(--space-1);
}

.chip {
  cursor: pointer;
}

.quote {
  margin: 0;
  padding: var(--space-1) var(--space-2);
  overflow-x: auto;
  border-radius: var(--radius-sm);
  background: var(--color-surface-1);
  color: var(--color-fg-muted);
  font-family: var(--font-mono);
}

.time {
  align-self: flex-end;
  color: var(--color-fg-muted);
}

.event {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  justify-content: center;
  gap: var(--space-1);
  color: var(--color-fg-muted);
}

.event .time {
  align-self: center;
}

.drafts {
  display: flex;
  flex-wrap: wrap;
  gap: var(--space-1);
}

.composer {
  display: flex;
  align-items: flex-end;
  gap: var(--space-2);
}

.composer-text {
  flex: 1;
  min-width: 160px;
}

.picker {
  width: 220px;
}

.error {
  margin: 0;
  color: var(--color-error);
}
</style>
