<!-- SPDX-License-Identifier: MPL-2.0 -->
<!-- Copyright (C) 2026 The Lemongrass Authors -->

<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import {
  AppButton,
  AppEmptyState,
  AppIconButton,
  AppSegmented,
  AppSwitcher,
  AppTextarea
} from '@sankasten'
import { chatOf, setChatOpen, setChatText, setChatWide } from '../../conflictChatStore'
import {
  chatMessageFor,
  chatMessages,
  contextLine,
  inScope,
  scopeChip,
  type ChatScope
} from '../../conflictChat'
import { clearCommentDrafts, commentDraftsOf } from '../../conflictCommentStore'
import { commentMessages } from '../../conflictComments'
import { conflictThreadOf, handConflictTo, postToConflict } from '../../conflictThread'
import { modelHolds, writtenBeforeHandOver } from '../../conflictVerdicts'
import { resolveTargetsOf, agentVendorOf } from '../../reviewTargets'
import { tabParts } from '../../tabNames'
import type { ProjectRef } from '../../workspace'
import type {
  ConflictHumanMessage,
  TabBusy,
  WorkgroupThreadMessage
} from '../../../../preload/types'
import AgentIcon from '../AgentIcon.vue'
import MarkedText from '../MarkedText.vue'

// The thread with the agent, floating over the conflict. It lists the plain messages of the one
// thread of this conflict, the human's and the agent's, each tagged with where it was sent from,
// and a line the human writes here goes out at once, apart from the comments queued on lines. With
// no agent holding the conflict it is where one is picked: the first line sent hands the conflict
// over, with the queued comments, and the agent wakes once.
const props = defineProps<{
  project: ProjectRef
  // What the box is about right now: the selected block in Decisions, the file in Manual.
  scope: ChatScope
}>()

// Each poll runs lgrassd once, so this is as often as the picker can afford.
const BUSY_POLL_MS = 2500

const chat = computed(() => chatOf(props.project.id))
const held = computed(() => modelHolds(props.project.id))
const thread = computed(() => conflictThreadOf(props.project.id))

// Widening asks about the whole file instead of the block, and never reaches past the file.
const sendScope = computed((): ChatScope =>
  chat.value.wide ? { path: props.scope.path, block: null } : props.scope
)

const everything = ref(false)
const scopeOptions = computed(() => [
  { value: 'here' as const, label: props.scope.block === null ? 'This file' : 'This block' },
  { value: 'all' as const, label: 'All' }
])

const shown = computed((): WorkgroupThreadMessage[] => {
  const all = chatMessages(thread.value.messages)
  return everything.value ? all : all.filter((message) => inScope(message, props.scope))
})

const queued = computed(() => commentDraftsOf(props.project.id).length)
const text = computed({
  get: () => chat.value.text,
  set: (value: string) => setChatText(props.project.id, value)
})

const busy = ref<Record<string, TabBusy>>({})
const tabId = ref<string | null>(null)
let pollTimer: ReturnType<typeof setInterval> | null = null

const targets = computed(() =>
  resolveTargetsOf(props.project.id).map((target) => {
    const state = busy.value[target.tabId]
    return {
      id: target.tabId,
      label: target.label,
      disabled: state?.busy ? (state.reason ?? 'Busy') : undefined
    }
  })
)
const picked = computed(() => targets.value.find((target) => target.id === tabId.value) ?? null)

const blocker = computed(() => {
  if (!text.value.trim() && (held.value || queued.value === 0)) return 'Write something first.'
  if (held.value) return null
  if (!picked.value) return 'Pick the agent to hand the conflict to.'
  return picked.value.disabled ?? null
})

async function pollBusy(): Promise<void> {
  try {
    busy.value = await window.api.tabSessions.states(props.project.path)
  } catch {
    busy.value = {}
  }
}

function startPolling(): void {
  if (pollTimer) return
  void pollBusy()
  pollTimer = setInterval(pollBusy, BUSY_POLL_MS)
}

function stopPolling(): void {
  if (pollTimer) clearInterval(pollTimer)
  pollTimer = null
}

watch(held, (isHeld) => (isHeld ? stopPolling() : startPolling()), { immediate: true })

const sending = ref(false)
const error = ref<string | null>(null)

// A line sent while an agent holds the conflict is posted at once and wakes it. Without one the
// conflict is handed to the picked tab with the line and every queued comment in the same step.
async function send(): Promise<void> {
  if (blocker.value || sending.value) return
  sending.value = true
  error.value = null
  const body = text.value.trim()
  const line = body ? [chatMessageFor(sendScope.value, body)] : []
  let refused: string | null
  if (held.value) {
    refused = await postToConflict(props.project, line)
  } else {
    const comments = commentMessages(commentDraftsOf(props.project.id).map((d) => ({ ...d })))
    const messages: ConflictHumanMessage[] = [...line, ...comments]
    // A line about one block is also the ask of that block, so the agent is told to leave the
    // others alone.
    const block = sendScope.value.block
    const asked =
      body && block !== null
        ? {
            path: sendScope.value.path,
            verdicts: [{ block, verdict: 'asked' as const, text: body }]
          }
        : null
    refused = await handConflictTo(
      props.project,
      tabId.value!,
      () => writtenBeforeHandOver(props.project),
      asked,
      messages
    )
    if (!refused) clearCommentDrafts(props.project.id)
  }
  sending.value = false
  if (refused) {
    error.value = refused
    return
  }
  setChatText(props.project.id, '')
}

const list = ref<HTMLElement | null>(null)
const input = ref<InstanceType<typeof AppTextarea> | null>(null)

watch(
  () => shown.value.length,
  async () => {
    await nextTick()
    if (list.value) list.value.scrollTop = list.value.scrollHeight
  }
)

watch(
  () => chat.value.focus,
  async () => {
    await nextTick()
    input.value?.focus()
  }
)

onMounted(async () => {
  await nextTick()
  if (list.value) list.value.scrollTop = list.value.scrollHeight
  input.value?.focus()
})

onBeforeUnmount(stopPolling)

function timeOf(message: WorkgroupThreadMessage): string {
  const at = new Date(message.createdAt)
  return Number.isNaN(at.getTime())
    ? ''
    : at.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })
}

function lines(message: WorkgroupThreadMessage): string | null {
  const anchor = message.anchor
  if (!anchor || anchor.startLine < 1) return null
  const span =
    anchor.startLine === anchor.endLine
      ? `line ${anchor.startLine}`
      : `lines ${anchor.startLine} to ${anchor.endLine}`
  const side = anchor.pane === 'left' || anchor.pane === 'right' ? ` (${anchor.pane} side)` : ''
  return `${span}${side}`
}
</script>

<template>
  <section class="chat" role="dialog" aria-label="Thread with the agent">
    <header class="head">
      <h3 class="title">Thread</h3>
      <AppSegmented
        v-if="held"
        size="sm"
        aria-label="Which messages to list"
        :options="scopeOptions"
        :model-value="everything ? 'all' : 'here'"
        @update:model-value="everything = $event === 'all'"
      />
      <span class="spacer" />
      <AppIconButton size="sm" label="Close the thread" @click="setChatOpen(project.id, false)">
        <svg
          width="12"
          height="12"
          viewBox="0 0 12 12"
          fill="none"
          stroke="currentColor"
          stroke-width="1.3"
          stroke-linecap="round"
        >
          <path d="M2.5 2.5l7 7M9.5 2.5l-7 7" />
        </svg>
      </AppIconButton>
    </header>

    <div ref="list" class="messages">
      <AppEmptyState
        v-if="!held"
        layout="fill"
        size="sm"
        title="No thread yet."
        detail="Pick an agent below and write to it. Comments you queued on lines go with your first line."
      />
      <p v-else-if="shown.length === 0" class="empty">
        Nothing here yet. {{ everything ? '' : 'Switch to All to see the whole thread.' }}
      </p>
      <article
        v-for="message in shown"
        :key="message.id"
        class="message"
        :class="message.author === 'human' ? 'human' : 'agent'"
      >
        <header class="meta">
          <span class="who">
            <AgentIcon
              v-if="message.author === 'tab' && agentVendorOf(project.id, message.tabId)"
              :agent-id="agentVendorOf(project.id, message.tabId)!"
            />
            <MarkedText v-if="message.author === 'tab'" :parts="tabParts(message.tabId)" />
            <span v-else>You</span>
          </span>
          <span v-if="scopeChip(message)" class="chip">{{ scopeChip(message) }}</span>
          <span class="time">{{ timeOf(message) }}</span>
        </header>
        <p v-if="lines(message)" class="where">On {{ lines(message) }}</p>
        <pre v-if="message.anchor?.quote" class="quote">{{ message.anchor.quote }}</pre>
        <p class="body">{{ message.body }}</p>
      </article>
    </div>

    <footer class="foot">
      <AppSwitcher
        v-if="!held"
        :items="targets"
        :active-id="tabId"
        placeholder="Pick the agent to hand it to"
        empty-text="No Claude, Codex or Antigravity tab is open in this project."
        framed
        size="sm"
        above
        @select="tabId = $event"
      />
      <p class="context">
        <span>{{ contextLine(sendScope) }}</span>
        <AppButton
          v-if="scope.block !== null"
          variant="text"
          size="xs"
          @click="setChatWide(project.id, !chat.wide)"
        >
          {{ chat.wide ? 'Ask about the block' : 'Ask about the whole file' }}
        </AppButton>
      </p>
      <AppTextarea
        ref="input"
        v-model="text"
        size="sm"
        rows="3"
        placeholder="Write to the agent"
        @keydown.ctrl.enter.prevent="send"
        @keydown.meta.enter.prevent="send"
      />
      <div class="row">
        <span v-if="!held && queued > 0" class="note">
          {{ queued }} {{ queued === 1 ? 'comment goes' : 'comments go' }} with it.
        </span>
        <span class="spacer" />
        <AppButton
          variant="primary"
          size="sm"
          :disabled="!!blocker || sending"
          :title="blocker ?? undefined"
          @click="send"
        >
          {{ sending ? 'Sending' : 'Send' }}
        </AppButton>
      </div>
      <p v-if="error" class="error">{{ error }}</p>
    </footer>
  </section>
</template>

<style scoped>
.chat {
  display: flex;
  flex-direction: column;
  width: 380px;
  max-height: min(560px, calc(100vh - 160px));
  min-height: 0;
  overflow: hidden;
  background: var(--layer-float);
  border: 1px solid var(--color-border-default);
  border-radius: var(--radius-lg);
  box-shadow: var(--shadow-xl);
}

.head {
  display: flex;
  align-items: center;
  gap: var(--space-2);
  padding: var(--space-2) var(--space-3);
  border-bottom: 1px solid var(--color-border-subtle);
}

.title {
  margin: 0;
  font-size: var(--text-sm);
  font-weight: var(--weight-semibold);
  color: var(--color-fg-primary);
}

.spacer {
  flex: 1;
}

.messages {
  display: flex;
  flex-direction: column;
  gap: var(--space-3);
  flex: 1;
  min-height: 160px;
  overflow-y: auto;
  padding: var(--space-3);
}

.empty {
  margin: 0;
  font-size: var(--text-xs);
  color: var(--color-fg-muted);
}

.message {
  display: flex;
  flex-direction: column;
  gap: var(--space-1);
  padding: var(--space-2) var(--space-3);
  border-radius: var(--radius-md);
  background: var(--color-surface-1);
}

.message.human {
  background: var(--color-surface-2);
}

.meta {
  display: flex;
  align-items: center;
  gap: var(--space-2);
  min-width: 0;
  font-size: var(--text-xs);
}

.who {
  display: flex;
  align-items: center;
  gap: var(--space-1);
  min-width: 0;
  font-weight: var(--weight-medium);
  color: var(--color-fg-secondary);
}

.chip {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  padding: 0 var(--space-2);
  border-radius: var(--radius-pill);
  background: var(--color-surface-3);
  color: var(--color-fg-secondary);
}

.time {
  margin-left: auto;
  flex-shrink: 0;
  color: var(--color-fg-muted);
}

.where {
  margin: 0;
  font-size: var(--text-xs);
  color: var(--color-fg-muted);
}

.quote {
  margin: 0;
  padding: var(--space-1) var(--space-2);
  overflow-x: auto;
  border-left: 2px solid var(--color-border-default);
  font-family: var(--font-mono);
  font-size: var(--text-xs);
  color: var(--color-fg-secondary);
}

.body {
  margin: 0;
  font-size: var(--text-sm);
  color: var(--color-fg-primary);
  white-space: pre-wrap;
  word-break: break-word;
}

.foot {
  display: flex;
  flex-direction: column;
  gap: var(--space-2);
  padding: var(--space-2) var(--space-3) var(--space-3);
  border-top: 1px solid var(--color-border-subtle);
}

.context {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: var(--space-2);
  margin: 0;
  font-size: var(--text-xs);
  color: var(--color-fg-secondary);
}

.row {
  display: flex;
  align-items: center;
  gap: var(--space-2);
}

.note {
  font-size: var(--text-xs);
  color: var(--color-fg-muted);
}

.error {
  margin: 0;
  font-size: var(--text-xs);
  color: var(--color-error);
}
</style>
