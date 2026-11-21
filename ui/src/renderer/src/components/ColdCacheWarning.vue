<!-- SPDX-License-Identifier: MPL-2.0 -->
<!-- Copyright (C) 2026 The Lemongrass Authors -->

<script setup lang="ts">
// The warning over a shell whose prompt cache expired over a large context: its next prompt would
// resend all of it at full price, so typing into it waits for the human's choice. It is drawn by
// the pane over its body, never inside the terminal, so no terminal layer can sit above it, and it
// answers the keyboard, so it can never trap the tab.
import { computed, nextTick, onMounted, ref } from 'vue'
import { AppButton, AppIcon } from '@sankasten'
import { continueCold, nudgeWaiting } from '../coldTabs'
import { taskOfTab } from '../taskOwners'
import { contextStore, tokenCount } from '../usage'
import { ageNow } from '../ageClock'
import { focusShell } from '../shellRegistry'
import { openFreshTabOnTask, type ProjectRef } from '../workspace'
import { DEFAULT_CACHE_TTL_S } from '../../../preload/cacheState'
import MarkedText from './MarkedText.vue'
import { spanText } from '../elapsed'

const props = defineProps<{
  tabId: string
  project: ProjectRef
}>()

const cardEl = ref<HTMLDivElement>()

const facts = computed(() => {
  const context = contextStore.tabs[props.tabId]
  if (!context) return null
  return {
    tokens: tokenCount(context.used_tokens),
    idle: spanText(ageNow.value - context.seen_at * 1000),
    ttl: spanText((context.cache_ttl || DEFAULT_CACHE_TTL_S) * 1000),
    nudge: nudgeWaiting(props.tabId),
    task: taskOfTab(props.project.path, props.tabId)
  }
})

function onContinue(): void {
  continueCold(props.tabId)
  focusShell(props.tabId)
}

function onFreshTab(): void {
  const task = facts.value?.task
  if (task) openFreshTabOnTask(props.project, props.tabId, task)
}

// Escape always continues, and Enter on the card itself takes the highlighted choice; a focused
// button handles its own Enter.
function onKeydown(event: KeyboardEvent): void {
  if (event.key === 'Escape') {
    event.preventDefault()
    onContinue()
  } else if (event.key === 'Enter' && event.target === cardEl.value) {
    event.preventDefault()
    if (facts.value?.task) onFreshTab()
    else onContinue()
  }
}

// Focus moves here only from this pane's own terminal, so a tab going cold never pulls the
// keyboard away from another pane the human is typing in.
onMounted(async () => {
  await nextTick()
  const body = cardEl.value?.closest('.pane-body')
  if (body && body.contains(document.activeElement)) cardEl.value?.focus()
})
</script>

<template>
  <div v-if="facts" class="cold-cover" @pointerdown.self="cardEl?.focus()">
    <div
      ref="cardEl"
      class="cold-card"
      role="alertdialog"
      aria-labelledby="cold-title"
      tabindex="-1"
      @keydown="onKeydown"
    >
      <AppIcon class="cold-icon" name="warning" :size="20" />
      <p id="cold-title" class="cold-title">This session's cache has expired</p>
      <p class="cold-text">
        Its last response was {{ facts.idle }} ago, and its prompt cache lives {{ facts.ttl }}. The
        next prompt resends about {{ facts.tokens }} tokens of context at full price.
      </p>
      <p v-if="facts.nudge" class="cold-text">
        A thread message is waiting for this tab. It reaches the agent with your next prompt.
      </p>
      <p v-if="facts.task" class="cold-text">
        <MarkedText
          :parts="[
            { text: 'This tab works on ' },
            { text: facts.task, mark: true },
            { text: `. A fresh tab can pick it up from the task's files for far less.` }
          ]"
        />
      </p>
      <div class="cold-actions">
        <span class="cold-keys">Esc continues</span>
        <AppButton :variant="facts.task ? 'ghost' : 'primary'" @click="onContinue">
          Continue anyway
        </AppButton>
        <AppButton v-if="facts.task" variant="primary" @click="onFreshTab">
          Fresh tab on {{ facts.task }}
        </AppButton>
      </div>
    </div>
  </div>
</template>

<style scoped>
/* Above every tab body in the pane, whatever stacking its own content sets up. */
.cold-cover {
  position: absolute;
  inset: 0;
  z-index: 20;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: var(--space-4);
  background: color-mix(in oklch, var(--color-surface-0) 82%, transparent);
}

.cold-card {
  display: flex;
  flex-direction: column;
  align-items: flex-start;
  gap: var(--space-3);
  max-width: 460px;
  padding: var(--space-5);
  background: var(--color-surface-2);
  border: 1px solid var(--color-amber-dim);
  border-radius: var(--radius-lg);
  box-shadow: var(--shadow-lg);
  font-family: var(--font-body);
  outline: none;
}

.cold-card:focus-visible {
  border-color: var(--color-amber);
}

.cold-icon {
  color: var(--color-amber);
}

.cold-title {
  margin: 0;
  color: var(--color-fg-primary);
  font-size: var(--text-md);
  font-weight: 600;
}

.cold-text {
  margin: 0;
  color: var(--color-fg-secondary);
  font-size: var(--text-sm);
  line-height: var(--leading-snug);
}

.cold-actions {
  display: flex;
  align-items: center;
  gap: var(--space-2);
  align-self: stretch;
  justify-content: flex-end;
  margin-top: var(--space-2);
}

.cold-keys {
  margin-right: auto;
  color: var(--color-fg-muted);
  font-size: var(--text-xs);
}
</style>
