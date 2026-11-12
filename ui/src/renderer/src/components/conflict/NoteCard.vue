<!-- SPDX-License-Identifier: MPL-2.0 -->
<!-- Copyright (C) 2026 The Lemongrass Authors -->

<script setup lang="ts">
import { ref } from 'vue'
import { AppButton, AppTextarea } from '@sankasten'

export interface NoteReply {
  id: number
  author: string
  mine: boolean
  body: string
}

// One of the model's notes on the middle, with the replies to it and a box to answer it in place.
// A reply posts at once, as the human's message attached to the note. Model text is shown as text,
// never as HTML.
const props = defineProps<{
  author: string
  where: string
  body: string
  question: boolean
  replies: NoteReply[]
  // Posts the reply and returns the refusal, or null once it is in the thread.
  send: (body: string) => Promise<string | null>
}>()

const text = ref('')
const sending = ref(false)
const error = ref<string | null>(null)

async function reply(): Promise<void> {
  const body = text.value.trim()
  if (!body || sending.value) return
  sending.value = true
  error.value = null
  try {
    const refused = await props.send(body)
    if (refused) error.value = refused
    else text.value = ''
  } finally {
    sending.value = false
  }
}
</script>

<template>
  <div class="note-card" :class="{ question }">
    <div class="head">
      <span class="author">{{ author }}</span>
      <span>{{ question ? `asks about ${where}` : `note on ${where}` }}</span>
    </div>
    <p class="body">{{ body }}</p>
    <ul v-if="replies.length" class="replies">
      <li v-for="item in replies" :key="item.id" class="reply" :class="{ mine: item.mine }">
        <span class="author">{{ item.author }}</span>
        <span class="reply-body">{{ item.body }}</span>
      </li>
    </ul>
    <div class="composer">
      <AppTextarea
        v-model="text"
        size="sm"
        rows="1"
        :placeholder="question ? 'Answer the model' : 'Reply to this note'"
        :disabled="sending"
        @keydown.enter.ctrl.exact.prevent="reply"
        @keydown.enter.meta.exact.prevent="reply"
      />
      <AppButton
        type="button"
        size="sm"
        variant="primary"
        :disabled="!text.trim() || sending"
        title="Send the reply to the model (Ctrl+Enter)"
        @click="reply"
        >Reply</AppButton
      >
    </div>
    <p v-if="error" class="error">{{ error }}</p>
  </div>
</template>

<style scoped>
.note-card {
  display: flex;
  flex-direction: column;
  gap: var(--space-1);
  padding: var(--space-1) var(--space-2);
  border-left: 3px solid var(--color-fg-muted);
  border-radius: var(--radius-sm);
  background: var(--color-surface-1);
  font-family: var(--font-body);
  font-size: var(--text-xs);
  white-space: normal;
  cursor: auto;
}

/* A question waits on the human, so its edge carries the same red as an open block. */
.note-card.question {
  border-left-color: var(--color-error);
}

.head {
  display: flex;
  gap: var(--space-2);
  color: var(--color-fg-muted);
}

.author {
  color: var(--color-fg-secondary);
  font-weight: var(--weight-medium);
}

.body,
.reply-body {
  margin: 0;
  color: var(--color-fg-primary);
  white-space: pre-wrap;
  word-break: break-word;
}

.replies {
  display: flex;
  flex-direction: column;
  gap: var(--space-1);
  margin: 0;
  padding: 0 0 0 var(--space-2);
  list-style: none;
  border-left: 1px solid var(--color-border-default);
}

.reply {
  display: flex;
  gap: var(--space-2);
}

.composer {
  display: flex;
  align-items: flex-end;
  gap: var(--space-2);
}

.composer > :first-child {
  flex: 1;
  min-width: 0;
}

.error {
  margin: 0;
  color: var(--color-error);
}
</style>
