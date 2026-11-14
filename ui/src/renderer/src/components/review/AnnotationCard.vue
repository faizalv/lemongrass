<!-- SPDX-License-Identifier: MPL-2.0 -->
<!-- Copyright (C) 2026 The Lemongrass Authors -->

<script setup lang="ts">
import { nextTick, onMounted, ref } from 'vue'
import { AppButton, AppTextarea } from '@sankasten'
import AgentIcon from '../AgentIcon.vue'
import AgentWorking from '../conflict/AgentWorking.vue'
import type { CardReply } from '../../annotationGutter'

// A comment card in an editor. A draft has no saved comment yet, so cancelling it discards it
// instead of returning to view. A saved comment waits for Send and can be edited or deleted. A sent
// comment is read only, shows what answered it, and takes a reply of the human's own, which the
// caller queues like any other comment.
const props = defineProps<{
  comment: string
  draft: boolean
  sent?: boolean
  // A saved comment that answers a sent one.
  isReply?: boolean
  replies?: CardReply[]
  // A sent comment the agent has not answered yet: working while its tab's turn runs.
  waiting?: 'working' | 'idle' | null
}>()

const emit = defineEmits<{
  save: [comment: string]
  cancel: []
  remove: []
  reply: [comment: string]
}>()

const editing = ref(props.draft)
const replying = ref(false)
const text = ref(props.comment)
const inputRef = ref<InstanceType<typeof AppTextarea> | null>(null)

async function startEditing(): Promise<void> {
  text.value = props.comment
  editing.value = true
  await nextTick()
  inputRef.value?.focus()
}

async function startReply(): Promise<void> {
  text.value = ''
  replying.value = true
  await nextTick()
  inputRef.value?.focus()
}

function save(): void {
  const comment = text.value.trim()
  if (!comment) return
  if (replying.value) {
    emit('reply', comment)
    replying.value = false
    return
  }
  emit('save', comment)
  if (!props.draft) editing.value = false
}

function cancel(): void {
  if (replying.value) replying.value = false
  else if (props.draft) emit('cancel')
  else editing.value = false
}

onMounted(() => {
  if (editing.value) inputRef.value?.focus()
})
</script>

<template>
  <div class="annotation-card" :class="{ sent, reply: isReply }">
    <template v-if="editing || replying">
      <AppTextarea
        ref="inputRef"
        v-model="text"
        size="sm"
        rows="3"
        :placeholder="replying ? 'Reply' : 'Leave a comment'"
        @keydown.escape.stop.prevent="cancel"
        @keydown.enter.ctrl.exact.prevent="save"
        @keydown.enter.meta.exact.prevent="save"
      />
      <div class="actions">
        <AppButton type="button" size="sm" @click="cancel">Cancel</AppButton>
        <AppButton type="button" size="sm" variant="primary" :disabled="!text.trim()" @click="save">
          {{ replying ? 'Add reply' : draft ? 'Add comment' : 'Save' }}
        </AppButton>
      </div>
    </template>
    <template v-else>
      <p v-if="isReply" class="label">Your reply, sent with the rest</p>
      <p v-else-if="!sent" class="label">Comment, sent with the rest</p>
      <p class="comment">{{ comment }}</p>
      <div v-for="(answer, index) in replies ?? []" :key="index" class="answer">
        <span class="who">
          <AgentIcon v-if="answer.author === 'tab' && answer.vendor" :agent-id="answer.vendor" />
          <span>{{ answer.author === 'human' ? 'You' : answer.label }}</span>
        </span>
        <p class="comment">{{ answer.body }}</p>
      </div>
      <AgentWorking
        v-if="sent && waiting"
        class="waiting"
        :working="waiting === 'working'"
        working-text="Agent is replying"
        idle-text="Not answered yet"
      />
      <div class="actions">
        <template v-if="sent">
          <AppButton type="button" size="sm" @click="startReply">Reply</AppButton>
        </template>
        <template v-else>
          <AppButton type="button" size="sm" variant="danger" @click="emit('remove')">
            Delete
          </AppButton>
          <AppButton type="button" size="sm" @click="startEditing">Edit</AppButton>
        </template>
      </div>
    </template>
  </div>
</template>

<style scoped>
.annotation-card {
  display: flex;
  flex-direction: column;
  gap: var(--space-2);
  margin: var(--space-1) var(--space-2);
  padding: var(--space-2) var(--space-3);
  background: var(--color-surface-1);
  border: 1px solid var(--color-border-default);
  border-left: 3px solid var(--color-amber);
  border-radius: var(--radius-md);
  font-family: var(--font-body);
}

.annotation-card.sent {
  border-left-color: var(--color-jade);
}

.annotation-card.reply {
  margin-left: var(--space-5);
}

.label {
  margin: 0;
  font-size: var(--text-xs);
  color: var(--color-fg-muted);
}

.comment {
  margin: 0;
  color: var(--color-fg-primary);
  font-size: var(--text-xs);
  white-space: pre-wrap;
  word-break: break-word;
}

.waiting {
  padding-top: var(--space-2);
  border-top: 1px solid var(--color-border-subtle);
}
.answer {
  display: flex;
  flex-direction: column;
  gap: var(--space-1);
  padding-top: var(--space-2);
  border-top: 1px solid var(--color-border-subtle);
}

.who {
  display: flex;
  align-items: center;
  gap: var(--space-1);
  font-size: var(--text-xs);
  font-weight: var(--weight-medium);
  color: var(--color-fg-secondary);
}

.actions {
  display: flex;
  justify-content: flex-end;
  gap: var(--space-2);
}
</style>
