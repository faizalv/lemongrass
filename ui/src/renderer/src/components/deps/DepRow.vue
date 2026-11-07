<!-- SPDX-License-Identifier: MPL-2.0 -->
<!-- Copyright (C) 2026 The Lemongrass Authors -->

<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import { AppButton, AppProgressLine } from '@sankasten'
import { formatElapsed } from '../../elapsed'
import {
  actionLabel,
  anyAgentReady,
  buttonWeight,
  depLabel,
  featuresText,
  offPathText,
  originText,
  routeBlocked,
  routeNote,
  statusText,
  tone
} from '../../depsView'
import type { DepEntry, DepsState } from '../../../../preload/types'

const props = defineProps<{
  dep: DepEntry
  state: DepsState
}>()

const installing = computed(() =>
  props.state.installing?.name === props.dep.name ? props.state.installing : null
)
const verb = computed(() => (installing.value?.action === 'upgrade' ? 'Updating' : 'Installing'))

// The elapsed counter ticks once a second only while this row's install runs.
const now = ref(Date.now())
let ticker: ReturnType<typeof setInterval> | undefined

watch(
  () => !!installing.value,
  (on) => {
    clearInterval(ticker)
    ticker = undefined
    if (!on) return
    now.value = Date.now()
    ticker = setInterval(() => (now.value = Date.now()), 1000)
  },
  { immediate: true }
)

onBeforeUnmount(() => clearInterval(ticker))

const elapsed = computed(() =>
  installing.value ? formatElapsed(Math.max(0, now.value - installing.value.startedAt)) : null
)

const outcome = computed(() => props.state.outcomes[props.dep.name] ?? null)
const weight = computed(() => buttonWeight(props.dep, !anyAgentReady(props.state.report)))
const blocked = computed(() =>
  props.dep.offer ? routeBlocked(props.dep.offer, props.state.unsupported) : null
)

// One install at a time, since the package manager holds a single lock, and none while a check runs.
const disabled = computed(
  () => !!blocked.value || !!props.state.installing || props.state.phase === 'checking'
)

const note = computed(() =>
  props.dep.offer && !blocked.value
    ? routeNote(props.dep, props.state.report?.os.family ?? '')
    : null
)

const offPath = computed(() => {
  const dir = props.state.offPath[props.dep.name]
  return dir && props.dep.status === 'ok' ? offPathText(props.dep, dir) : null
})

const meta = computed(() => [props.dep.path, originText(props.dep)].filter(Boolean).join(', '))

function install(): void {
  void window.api.deps.install(props.dep.name)
}
</script>

<template>
  <li class="dep-row">
    <div class="dep-head">
      <span class="settings-row-title">{{ depLabel(dep) }}</span>
      <span class="dep-status" :class="tone(dep)">{{ statusText(dep) }}</span>
      <span v-if="!dep.required" class="settings-badge">Optional</span>
      <span class="dep-spacer" />
      <AppButton
        v-if="weight && dep.offer"
        size="xs"
        type="button"
        :variant="weight"
        :disabled="disabled"
        :title="blocked ?? undefined"
        @click="install"
      >
        {{ installing ? 'Working' : `${actionLabel(dep.offer)} ${dep.offer.package}` }}
      </AppButton>
    </div>
    <p v-if="meta" class="settings-row-meta dep-meta" :title="meta">{{ meta }}</p>
    <p v-if="dep.features.length" class="dep-text">{{ featuresText(dep.features) }}</p>
    <p v-if="dep.message" class="dep-text dep-message">{{ dep.message }}</p>
    <p v-if="blocked && weight" class="dep-text">{{ blocked }}</p>
    <p v-if="note && weight && !installing" class="dep-text">{{ note }}</p>
    <p v-if="offPath" class="dep-text">{{ offPath }}</p>
    <div v-if="installing" class="dep-output">
      <div class="dep-run">
        <AppProgressLine class="dep-run-line" :label="`${verb} ${installing.package}`" />
        <span class="dep-run-clock"
          >{{ verb }} {{ installing.package }} <span class="dep-run-time">{{ elapsed }}</span></span
        >
      </div>
      <pre v-if="installing.lines.length" class="dep-lines sk-running">{{
        installing.lines.join('\n')
      }}</pre>
    </div>
    <p v-else-if="outcome" class="dep-text" :class="outcome.ok ? 'dep-ok' : 'settings-error'">
      {{ outcome.text }}
    </p>
  </li>
</template>

<style scoped>
.dep-row {
  display: flex;
  flex-direction: column;
  gap: var(--space-1);
  padding: var(--space-2) var(--space-3);
  border: 1px solid var(--color-border-default);
  border-radius: var(--radius-md, 8px);
}

.dep-head {
  display: flex;
  align-items: center;
  gap: var(--space-3);
}

.dep-spacer {
  flex: 1;
}

.dep-status {
  font-family: var(--font-mono);
  font-size: var(--text-xs);
}

.dep-status.ok {
  color: var(--color-success);
}

.dep-status.warn {
  color: var(--color-warning);
}

.dep-status.muted {
  color: var(--color-fg-muted);
}

.dep-meta {
  flex: none;
  margin: 0;
}

.dep-text {
  margin: 0;
  color: var(--color-fg-secondary);
  font-size: var(--text-xs);
  line-height: var(--leading-relaxed);
}

.dep-message {
  color: var(--color-fg-primary);
}

.dep-ok {
  color: var(--color-success);
}

.dep-output {
  display: flex;
  flex-direction: column;
  gap: var(--space-1);
}

.dep-run {
  display: flex;
  align-items: center;
  gap: var(--space-3);
}

.dep-run-line {
  flex: 1;
  min-width: 0;
}

.dep-run-clock {
  flex-shrink: 0;
  color: var(--color-jade);
  font-size: var(--text-xs);
  font-weight: var(--weight-semibold);
}

/* The body font has no tabular digits, so the time is set in mono and the line beside it keeps still
   while the counter ticks. */
.dep-run-time {
  font-family: var(--font-mono);
}

/* The transparent border is what sk-running colors while the install runs. */
.dep-lines {
  margin: 0;
  padding: var(--space-2);
  border: 1px solid transparent;
  overflow-x: auto;
  background: var(--color-surface-0);
  border-radius: var(--radius-sm, 4px);
  color: var(--color-fg-muted);
  font-family: var(--font-mono);
  font-size: var(--text-xs);
  white-space: pre;
}
</style>
