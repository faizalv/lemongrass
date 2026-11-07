<!-- SPDX-License-Identifier: MPL-2.0 -->
<!-- Copyright (C) 2026 The Lemongrass Authors -->

<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import { AppButton, AppEmptyState, AppModal } from '@sankasten'
import MarkedText from '../MarkedText.vue'
import DepRow from './DepRow.vue'
import { depsStore } from '../../deps'
import { agentDeps, agentGroupText, anyAgentReady } from '../../depsView'
import type { NamePart } from '../../tabNames'

// A check that finishes sooner than this never shows the spinner, so a cached check does not flash it.
const SPINNER_DELAY_MS = 300

// Closing without Continue hides the layer for this run only; setupVersion stays unsaved, so it
// shows again on the next startup.
const hidden = ref(false)

const state = computed(() => depsStore.state)
const open = computed(() => !hidden.value && !!state.value?.setupDue)
const required = computed(() => state.value?.report?.deps.filter((dep) => dep.required) ?? [])
const pending = computed(() => required.value.filter((dep) => dep.status !== 'ok'))
// The agents come first: with none of them no tab can run, while a missing tool only turns its features off.
const agents = computed(() => agentDeps(state.value?.report))
const agentPending = computed(() => agents.value.length > 0 && !anyAgentReady(state.value?.report))
const allReady = computed(() => pending.value.length === 0 && !agentPending.value)
const waiting = computed(() => state.value?.phase === 'checking' && !state.value.report)

const spinnerShown = ref(false)
let spinnerTimer: ReturnType<typeof setTimeout> | undefined
watch(
  waiting,
  (now) => {
    clearTimeout(spinnerTimer)
    spinnerShown.value = false
    if (now) spinnerTimer = setTimeout(() => (spinnerShown.value = true), SPINNER_DELAY_MS)
  },
  { immediate: true }
)
onBeforeUnmount(() => clearTimeout(spinnerTimer))

const manageParts: NamePart[] = [
  { text: 'You can manage other dependencies in ' },
  { text: 'Settings', mark: true },
  { text: ', under ' },
  { text: 'Dependencies', mark: true },
  { text: '.' }
]

// The install button carries the weight while a required dependency or the agent group is pending, so Continue
// steps down.
const continueVariant = computed(() =>
  pending.value.some((dep) => dep.offer) ||
  (agentPending.value && agents.value.some((dep) => dep.offer))
    ? 'ghost'
    : 'primary'
)

async function finish(): Promise<void> {
  hidden.value = true
  await window.api.deps.setupDone()
}
</script>

<template>
  <AppModal
    v-if="open && state"
    scope="app"
    size="md"
    title="Dependency Check"
    @close="hidden = true"
  >
    <div class="setup">
      <template v-if="waiting">
        <AppEmptyState v-if="spinnerShown" loading title="Optimizing Lemongrass" />
      </template>

      <template v-else-if="state.phase === 'failed' && !state.report">
        <p class="settings-error">{{ state.message }}</p>
        <p class="settings-text">You can check again from Settings, under Dependencies.</p>
      </template>

      <template v-else-if="state.report">
        <p v-if="allReady" class="settings-text">Everything lemongrass needs is in place.</p>
        <template v-if="agents.length > 0">
          <p v-if="agentPending" class="settings-text">{{ agentGroupText(agents) }}</p>
          <ul class="settings-rows">
            <DepRow v-for="dep in agents" :key="dep.name" :dep="dep" :state="state" />
          </ul>
        </template>
        <p v-if="pending.length > 0" class="settings-text">
          Lemongrass needs these tools. Without them, the features they serve stay off.
        </p>
        <ul class="settings-rows">
          <DepRow v-for="dep in required" :key="dep.name" :dep="dep" :state="state" />
        </ul>
        <p v-if="state.unsupported" class="settings-text">{{ state.unsupported }}</p>
        <p class="settings-text"><MarkedText :parts="manageParts" /></p>
      </template>
    </div>

    <template #footer>
      <AppButton type="button" :variant="continueVariant" @click="finish">Continue</AppButton>
    </template>
  </AppModal>
</template>

<style scoped>
.setup {
  display: flex;
  flex-direction: column;
  gap: var(--space-3);
}
</style>
