<!-- SPDX-License-Identifier: MPL-2.0 -->
<!-- Copyright (C) 2026 The Lemongrass Authors -->

<script setup lang="ts">
import { computed, ref } from 'vue'
import { AppButton } from '@sankasten'
import { chatOf, focusChat, setChatOpen } from '../../conflictChatStore'
import type { ChatScope } from '../../conflictChat'
import { commentDraftsOf } from '../../conflictCommentStore'
import type { FileDecisions } from '../../conflictDecisions'
import { sendQueued } from '../../conflictSend'
import { modelHolds } from '../../conflictVerdicts'
import type { ProjectRef } from '../../workspace'
import ConflictChat from './ConflictChat.vue'
import ConflictIcon from './ConflictIcon.vue'

// The Thread button, which opens the chat, and the one Send. Send posts what the human queued,
// decisions on blocks and comments on lines, together, so the agent wakes once; it reads Stage when
// only settled files are queued. The chat never waits for it: a line written there goes out at once.
// Manual floats it over the panes' bottom right; Decisions sets inline and places it at the right
// end of its own action row, since a floating bar covers that row.
const props = defineProps<{
  project: ProjectRef
  // Null when there is no file to talk about, which hides the chat.
  scope: ChatScope | null
  // The files with decisions waiting, as Decisions computes them; none in Manual.
  ready: { file: FileDecisions; settled: boolean }[]
  // How many blocks those decisions cover.
  queuedBlocks: number
  // Drawn in a row its parent owns instead of floating; Send shows only while something is queued.
  inline?: boolean
  title?: string
}>()

const comments = computed(() => commentDraftsOf(props.project.id).length)
const total = computed(() => props.queuedBlocks + comments.value)
const staging = computed(
  () => props.ready.length > 0 && comments.value === 0 && props.ready.every((item) => item.settled)
)
const label = computed(() => {
  if (total.value === 0 && props.ready.length === 0) return 'Send to agent'
  if (staging.value) return props.ready.length === 1 ? 'Stage' : `Stage (${props.ready.length})`
  return `Send to agent (${total.value})`
})
const nothing = computed(() => total.value === 0 && props.ready.length === 0)

const open = computed(() => chatOf(props.project.id).open && props.scope !== null)
const sending = ref(false)
const error = ref<string | null>(null)

// With no agent holding the conflict the chat is where one is picked, so Send leads there.
async function send(): Promise<void> {
  if (sending.value || nothing.value) return
  if (!modelHolds(props.project.id)) {
    focusChat(props.project.id)
    return
  }
  sending.value = true
  error.value = null
  error.value = await sendQueued(
    props.project,
    props.ready.map((item) => item.file)
  )
  sending.value = false
}
</script>

<template>
  <div v-if="scope" class="dock" :class="{ inline }">
    <div v-if="open || error" class="above">
      <ConflictChat v-if="open" :project="project" :scope="scope" />
      <p v-if="error" class="error">{{ error }}</p>
    </div>
    <div class="bar">
      <AppButton
        v-if="!nothing"
        variant="primary"
        size="sm"
        :disabled="sending"
        :title="title"
        @click="send"
      >
        {{ sending ? 'Sending' : label }}
      </AppButton>
      <AppButton
        variant="ghost"
        size="sm"
        :pressed="open"
        title="The thread with the agent"
        @click="open ? setChatOpen(project.id, false) : focusChat(project.id)"
      >
        <ConflictIcon name="thread" />
        Thread
      </AppButton>
    </div>
  </div>
</template>

<style scoped>
.dock {
  position: absolute;
  right: var(--space-4);
  bottom: var(--space-4);
  z-index: 20;
  display: flex;
  flex-direction: column;
  align-items: flex-end;
  gap: var(--space-2);
  max-width: calc(100% - var(--space-8));
}

.above {
  display: flex;
  flex-direction: column;
  align-items: flex-end;
  gap: var(--space-2);
}

/* In a row: the buttons take their place in it and the chat rises above them, over the view. */
.dock.inline {
  position: relative;
  right: auto;
  bottom: auto;
  flex-shrink: 0;
  max-width: none;
}

.dock.inline .above {
  position: absolute;
  right: 0;
  bottom: calc(100% + var(--space-3));
  z-index: 20;
}

.dock.inline .bar {
  padding: 0;
  background: none;
  border: none;
  box-shadow: none;
}

.bar {
  display: flex;
  align-items: center;
  gap: var(--space-2);
  padding: var(--space-2);
  background: var(--layer-float);
  border: 1px solid var(--color-border-default);
  border-radius: var(--radius-lg);
  box-shadow: var(--shadow-xl);
}

.error {
  margin: 0;
  max-width: 380px;
  padding: var(--space-1) var(--space-3);
  border-radius: var(--radius-md);
  background: var(--layer-float);
  border: 1px solid var(--color-error);
  font-size: var(--text-xs);
  color: var(--color-error);
}
</style>
