<!-- SPDX-License-Identifier: MPL-2.0 -->
<!-- Copyright (C) 2026 The Lemongrass Authors -->

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { AppButton, AppModal, AppSwitcher, AppTextarea } from '@sankasten'
import { resolveTargetsOf } from '../../reviewTargets'
import { glowTab } from '../../tabGlow'
import type { ProjectRef } from '../../workspace'
import type { TabBusy } from '../../../../preload/types'

// Ask Model: one modal holds the optional message and the tab, and Submit is the only step that
// acts. Without a model holding the conflict it hands the conflict over with the question; with
// one the tab is already known and the modal is only the message.
const props = defineProps<{
  project: ProjectRef
  // A model already holds the conflict, so there is no tab to pick.
  held: boolean
  // How many blocks the question is about.
  blocks: number
  sending: boolean
  error: string | null
}>()

const emit = defineEmits<{
  submit: [payload: { tabId: string | null; text: string }]
  close: []
}>()

// Each poll runs lgrassd once, so this is as often as the picker can afford.
const BUSY_POLL_MS = 2500

const text = ref('')
const tabId = ref<string | null>(null)
const busy = ref<Record<string, TabBusy>>({})
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
  if (props.held) return null
  if (!picked.value) return 'Pick the tab to hand the conflict to.'
  return picked.value.disabled ?? null
})

async function pollBusy(): Promise<void> {
  try {
    busy.value = await window.api.tabSessions.states(props.project.path)
  } catch {
    busy.value = {}
  }
}

onMounted(() => {
  if (props.held) return
  void pollBusy()
  pollTimer = setInterval(pollBusy, BUSY_POLL_MS)
})

onBeforeUnmount(() => {
  if (pollTimer) clearInterval(pollTimer)
  glowTab(null)
})

function submit(): void {
  if (blocker.value || props.sending) return
  emit('submit', { tabId: props.held ? null : tabId.value, text: text.value })
}
</script>

<template>
  <AppModal
    :title="blocks === 1 ? 'Ask a model about this block' : 'Ask a model about these blocks'"
    @close="emit('close')"
  >
    <div class="ask-body">
      <AppTextarea
        v-model="text"
        size="sm"
        rows="3"
        class="ask-text"
        placeholder="Optional. What should it tell you or change here?"
        @keydown.ctrl.enter.prevent="submit"
        @keydown.meta.enter.prevent="submit"
      />
      <template v-if="!held">
        <AppSwitcher
          :items="targets"
          :active-id="tabId"
          placeholder="Pick the tab to hand it to"
          empty-text="No Claude or Codex tab is open in this project."
          framed
          @select="tabId = $event"
          @point="glowTab"
        />
        <p class="ask-note">
          The tab gets the whole conflict, and is told you ask only about
          {{ blocks === 1 ? 'this block' : 'these blocks' }} and which blocks you already resolved.
        </p>
      </template>
      <p v-else class="ask-note">The model holding this conflict gets your question.</p>
      <p v-if="error" class="ask-error">{{ error }}</p>
    </div>
    <template #footer>
      <AppButton @click="emit('close')">Cancel</AppButton>
      <AppButton
        variant="primary"
        :disabled="!!blocker || sending"
        :title="blocker ?? undefined"
        @click="submit"
      >
        {{ sending ? 'Asking' : 'Ask' }}
      </AppButton>
    </template>
  </AppModal>
</template>

<style scoped>
.ask-body {
  display: flex;
  flex-direction: column;
  gap: var(--space-3);
}

.ask-text {
  width: 100%;
}

.ask-note {
  margin: 0;
  color: var(--color-fg-muted);
  font-size: var(--text-xs);
}

.ask-error {
  margin: 0;
  color: var(--color-error);
  font-size: var(--text-xs);
}
</style>
