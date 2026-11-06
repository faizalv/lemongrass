<!-- SPDX-License-Identifier: MPL-2.0 -->
<!-- Copyright (C) 2026 The Lemongrass Authors -->

<script setup lang="ts">
import {
  AppButton,
  AppCheckbox,
  AppDisclosure,
  AppEmptyState,
  AppIconButton,
  AppInput,
  AppModal
} from '@sankasten'
import { computed, ref } from 'vue'
import { gitStateOf, runStash } from '../git'
import { formatAge } from '../gitTime'
import type { ProjectRef } from '../workspace'
import type { GitStash } from '../../../preload/types'
import GitIcon from './GitIcon.vue'
import RevisionView from './RevisionView.vue'

const props = defineProps<{
  project: ProjectRef
  hasChanges: boolean
}>()

const state = computed(() => gitStateOf(props.project.id))
const stashes = computed(() => state.value.stashes)
const folded = ref(true)
const saving = ref(false)
const message = ref('')
const includeUntracked = ref(true)
const error = ref<string | null>(null)
const viewing = ref<GitStash | null>(null)

async function perform(
  action: 'apply' | 'pop' | 'drop' | { push: string; includeUntracked: boolean },
  ref = ''
): Promise<boolean> {
  error.value = null
  const failure = await runStash(props.project, action, ref)
  error.value = failure
  return failure === null
}

async function save(): Promise<void> {
  const text = message.value
  if (await perform({ push: text, includeUntracked: includeUntracked.value })) {
    message.value = ''
    saving.value = false
  }
}
</script>

<template>
  <div class="stash-section">
    <div class="stash-head">
      <AppDisclosure
        label="Stashes"
        :count="stashes.length || undefined"
        count-look="pill"
        :open="!folded"
        @update:open="folded = !$event"
      />
      <AppIconButton
        size="sm"
        tone="accent"
        :label="hasChanges ? 'Stash changes' : 'No changes to stash'"
        :disabled="!hasChanges || state.busy !== null"
        @click="((saving = !saving), (folded = false))"
      >
        <GitIcon name="stash" />
      </AppIconButton>
    </div>

    <template v-if="!folded">
      <div v-if="saving" class="stash-form">
        <AppInput
          v-model="message"
          size="sm"
          class="stash-input"
          placeholder="Message (optional)"
          aria-label="Stash message"
          :disabled="state.busy !== null"
          @keydown.enter="save"
        />
        <AppCheckbox
          size="sm"
          :checked="includeUntracked"
          @click="includeUntracked = !includeUntracked"
        >
          Include untracked files
        </AppCheckbox>
        <AppButton variant="primary" size="sm" :disabled="state.busy !== null" @click="save">
          Stash
        </AppButton>
      </div>
      <p v-if="error" class="stash-error">{{ error }}</p>
      <AppEmptyState v-if="stashes.length === 0" size="sm" title="No stashes" />
      <ul v-else class="stash-list">
        <li v-for="stash in stashes" :key="stash.ref" class="stash-item">
          <button class="stash-row" :title="stash.message" @click="viewing = stash">
            <span class="stash-message">{{ stash.message }}</span>
            <span class="stash-age">{{ formatAge(stash.time) }}</span>
          </button>
          <div class="stash-actions">
            <AppIconButton
              size="xs"
              tone="accent"
              label="Apply and keep the stash"
              :disabled="state.busy !== null"
              @click="perform('apply', stash.ref)"
            >
              <GitIcon name="apply" />
            </AppIconButton>
            <AppIconButton
              size="xs"
              tone="accent"
              label="Apply and remove the stash"
              :disabled="state.busy !== null"
              @click="perform('pop', stash.ref)"
            >
              <GitIcon name="pop" />
            </AppIconButton>
            <AppIconButton
              size="xs"
              tone="danger"
              label="Drop the stash"
              confirm
              confirm-label="Click again to drop"
              :disabled="state.busy !== null"
              @confirm="perform('drop', stash.ref)"
            >
              <GitIcon name="drop" />
            </AppIconButton>
          </div>
        </li>
      </ul>
    </template>

    <AppModal v-if="viewing" size="lg" flush :title="viewing.message" @close="viewing = null">
      <div class="stash-view">
        <RevisionView :project="project" :revision="{ kind: 'stash', ref: viewing.ref }" />
      </div>
    </AppModal>
  </div>
</template>

<style scoped>
.stash-section {
  display: flex;
  flex-direction: column;
  gap: var(--space-1);
}

.stash-head {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 0 var(--space-2);
}

.stash-form {
  display: flex;
  flex-direction: column;
  gap: var(--space-2);
  padding: 0 var(--space-2);
}

.stash-input {
  width: 100%;
}

.stash-error {
  max-height: 120px;
  overflow-y: auto;
  padding: var(--space-2);
  background: var(--color-error-muted);
  border-radius: var(--radius-md);
  color: var(--color-error);
  font-size: var(--text-xs);
  white-space: pre-wrap;
  word-break: break-word;
}

.stash-list {
  max-height: 160px;
  overflow-y: auto;
  list-style: none;
}

.stash-item {
  padding: 2px var(--space-2);
}

.stash-row {
  display: flex;
  align-items: center;
  gap: 6px;
  width: 100%;
  padding: 2px 0;
  background: none;
  border: none;
  color: var(--color-fg-secondary);
  font-family: var(--font-body);
  font-size: var(--text-xs);
  text-align: left;
  cursor: pointer;
}

.stash-row:hover {
  color: var(--color-fg-primary);
}

.stash-message {
  flex: 1;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.stash-age {
  flex-shrink: 0;
  color: var(--color-fg-muted);
}

.stash-actions {
  display: flex;
}

.stash-view {
  display: flex;
  height: 60vh;
}
</style>
