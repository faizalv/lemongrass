<!-- SPDX-License-Identifier: AGPL-3.0-only -->
<!-- Copyright (C) 2026 The Lemongrass Authors -->

<script setup lang="ts">
import { computed, ref } from 'vue'
import { pickConflict } from '../../git'
import type { ProjectRef } from '../../workspace'
import type {
  GitConflictChoice,
  GitConflictFile,
  GitConflictStage
} from '../../../../preload/types'
import AppButton from '../AppButton.vue'

type ConflictFile = Extract<GitConflictFile, { status: 'ok' }>

const PREVIEW_LINES = 200

const props = defineProps<{
  project: ProjectRef
  path: string
  file: ConflictFile
}>()

const busy = ref(false)
const error = ref<string | null>(null)

// A conflict with no lines to merge: one side deleted the file, both added it with no base, or
// a side is binary.
const reason = computed(() => {
  const { left, right, base } = props.file
  if (left.state === 'missing')
    return 'The left side deleted this file and the right side changed it.'
  if (right.state === 'missing')
    return 'The right side deleted this file and the left side changed it.'
  if (left.state === 'binary' || right.state === 'binary')
    return 'This file is binary, so there are no lines to merge.'
  if (base.state === 'missing')
    return 'Both sides added this file, so there is no common version to merge from.'
  return 'This conflict has no lines to merge.'
})

const sides = computed(() =>
  (['left', 'right'] as const).map((side) => {
    const stage = props.file[side]
    return {
      side,
      label: side === 'left' ? 'Left' : 'Right',
      ref: side === 'left' ? props.file.leftRef : props.file.rightRef,
      state: describe(stage),
      preview: stage.state === 'text' ? preview(stage.text) : null,
      choice: (stage.state === 'missing' ? 'delete' : side) as GitConflictChoice,
      action: stage.state === 'missing' ? 'Delete the file' : `Keep ${side}`
    }
  })
)

function describe(stage: GitConflictStage): string {
  if (stage.state === 'missing') return 'Deleted'
  if (stage.state === 'binary') return 'Binary'
  const lines = stage.text.split('\n').length - (stage.text.endsWith('\n') ? 1 : 0)
  return lines === 1 ? '1 line' : `${lines} lines`
}

function preview(text: string): string {
  const lines = text.split('\n')
  return lines.length > PREVIEW_LINES ? `${lines.slice(0, PREVIEW_LINES).join('\n')}\n...` : text
}

async function pick(choice: GitConflictChoice): Promise<void> {
  busy.value = true
  error.value = await pickConflict(props.project, props.path, choice)
  busy.value = false
}
</script>

<template>
  <div class="conflict-sides">
    <p class="reason">{{ reason }} Pick the side to keep.</p>
    <p v-if="error" class="sides-error">{{ error }}</p>
    <div class="cards">
      <section v-for="entry in sides" :key="entry.side" class="card">
        <header class="card-head">
          <span class="side">{{ entry.label }}</span>
          <span class="ref" :title="entry.ref">{{ entry.ref }}</span>
          <span class="state">{{ entry.state }}</span>
        </header>
        <pre v-if="entry.preview !== null" class="preview">{{ entry.preview }}</pre>
        <div v-else class="preview empty">{{ entry.state }}</div>
        <AppButton
          :variant="entry.choice === 'delete' ? 'danger' : 'primary'"
          :disabled="busy"
          @click="pick(entry.choice)"
        >
          {{ entry.action }}
        </AppButton>
      </section>
    </div>
  </div>
</template>

<style scoped>
.conflict-sides {
  display: flex;
  flex-direction: column;
  flex: 1;
  min-height: 0;
  gap: var(--space-3);
  padding: var(--space-4);
}

.reason {
  margin: 0;
  font-size: var(--text-sm);
  color: var(--color-fg-secondary);
}

.sides-error {
  margin: 0;
  font-size: var(--text-xs);
  color: var(--color-error);
}

.cards {
  display: flex;
  flex: 1;
  min-height: 0;
  gap: var(--space-4);
}

.card {
  display: flex;
  flex-direction: column;
  flex: 1;
  min-width: 0;
  gap: var(--space-2);
  align-items: flex-start;
}

.card-head {
  display: flex;
  gap: var(--space-2);
  width: 100%;
  font-size: var(--text-xs);
  white-space: nowrap;
}

.side {
  color: var(--color-fg-primary);
  font-weight: var(--weight-medium);
}

.ref {
  color: var(--color-fg-muted);
  overflow: hidden;
  text-overflow: ellipsis;
}

.state {
  margin-left: auto;
  color: var(--color-fg-secondary);
}

.preview {
  flex: 1;
  width: 100%;
  min-height: 0;
  margin: 0;
  padding: var(--space-2);
  overflow: auto;
  font-family: var(--font-mono);
  font-size: var(--text-xs);
  background: var(--color-surface-1);
  border: 1px solid var(--color-border-subtle);
  border-radius: var(--radius-sm);
  box-sizing: border-box;
}

.preview.empty {
  display: flex;
  align-items: center;
  justify-content: center;
  color: var(--color-fg-muted);
  font-family: var(--font-body);
}
</style>
