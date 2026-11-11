<!-- SPDX-License-Identifier: Apache-2.0 -->
<!-- Copyright (C) 2026 The Sankasten Authors -->

<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, ref, useAttrs, watch } from 'vue'
import AppButton from './AppButton.vue'
import { CONFIRM_DELAY_MS, createClickConfirm, type ConfirmState } from '../lib/clickConfirm'
import { placeFloat, placementStyle } from '../lib/floatPlace'

// Every click-again confirm in the app goes through this button, so the wait, the fill, the
// callout and the armed colour stay the same everywhere and no site has to word the prompt itself.
// The caller's class still sets size and layout.
defineOptions({ inheritAttrs: false })

const props = withDefaults(
  defineProps<{
    // The callout's prompt once armed, and the aria-label then. Starts with "Click
    // again" so the prompt always says what to do.
    confirmLabel?: string
    // Renders through AppButton in that variant; without it the root is a bare button.
    variant?: 'primary' | 'ghost' | 'danger'
    // error for a destructive action; jade for a go action such as starting a task.
    tone?: 'error' | 'jade'
    // Passed to AppButton; a bare root (no variant) takes its size from the caller.
    size?: 'xs' | 'sm' | 'md'
  }>(),
  { confirmLabel: 'Click again to confirm', variant: undefined, tone: 'error', size: 'md' }
)

const emit = defineEmits<{ confirm: [] }>()

const NUDGE_MS = 320
const CALLOUT_GAP = 6

const attrs = useAttrs()
const state = ref<ConfirmState>('idle')
// Set by a click during the wait: the user did not know to wait, so the callout explains the
// pattern until the confirm ends.
const explained = ref(false)
const nudging = ref(false)
let nudgeTimer: ReturnType<typeof setTimeout> | undefined

const confirm = createClickConfirm((next) => {
  state.value = next
  if (next === 'idle') explained.value = false
})

const root = computed(() =>
  props.variant ? { is: AppButton, variant: props.variant } : { is: 'button' }
)
const armed = computed(() => state.value === 'armed')
const calloutShown = computed(() => armed.value || explained.value)

const rootRef = ref<HTMLElement | { $el: HTMLElement } | null>(null)
const calloutRef = ref<HTMLElement | null>(null)
// Hidden until placed, so the callout never flashes at the window's corner.
const HIDDEN = { visibility: 'hidden' }
const calloutStyle = ref<Record<string, string>>(HIDDEN)

function rootEl(): HTMLElement | null {
  const value = rootRef.value
  return value && '$el' in value ? value.$el : value
}

function onClick(): void {
  const result = confirm.press()
  if (result === 'confirmed') emit('confirm')
  else if (result === 'early') {
    explained.value = true
    nudging.value = false
    clearTimeout(nudgeTimer)
    void nextTick(() => {
      nudging.value = true
      nudgeTimer = setTimeout(() => (nudging.value = false), NUDGE_MS)
    })
  }
}

// Leaving resets only once armed, so a small hand movement over a small target does not cancel
// the wait without the user noticing.
function onLeave(): void {
  if (armed.value) confirm.reset()
}

function onEscape(event: KeyboardEvent): void {
  if (state.value === 'idle') return
  event.stopPropagation()
  confirm.reset()
}

// The callout is fixed to the window in body, so a menu or popover with hidden overflow never
// clips it, and it is placed beside the button whenever it shows or the window moves under it.
function placeCallout(): void {
  const anchor = rootEl()
  const callout = calloutRef.value
  if (!anchor || !callout) return
  const rect = callout.getBoundingClientRect()
  calloutStyle.value = placementStyle(
    placeFloat(anchor.getBoundingClientRect(), rect.width, rect.height, CALLOUT_GAP)
  )
}

watch(calloutShown, async (shown) => {
  if (shown) {
    window.addEventListener('resize', placeCallout)
    window.addEventListener('scroll', placeCallout, true)
    await nextTick()
    placeCallout()
  } else {
    calloutStyle.value = HIDDEN
    window.removeEventListener('resize', placeCallout)
    window.removeEventListener('scroll', placeCallout, true)
  }
})

// The armed prompt is wider than the waiting one; placing again keeps it inside the window.
watch(armed, async () => {
  await nextTick()
  if (calloutShown.value) placeCallout()
})

onBeforeUnmount(() => {
  confirm.reset()
  clearTimeout(nudgeTimer)
  window.removeEventListener('resize', placeCallout)
  window.removeEventListener('scroll', placeCallout, true)
})
</script>

<!-- type="button" comes first so a caller can override it, and so a confirm inside a form never
     submits it on the first click. The native tooltip is dropped once the confirm starts, since it
     would cover the callout that already says the same thing. -->
<template>
  <component
    :is="root.is"
    ref="rootRef"
    type="button"
    v-bind="{ ...attrs, ...(root.variant ? { variant: root.variant, size } : {}) }"
    class="app-confirm"
    :class="{ 'is-nudging': nudging }"
    :data-confirm="state"
    :data-tone="tone"
    :title="state === 'idle' ? attrs.title : undefined"
    :aria-label="armed ? confirmLabel : attrs['aria-label']"
    :style="{ '--confirm-delay': `${CONFIRM_DELAY_MS}ms` }"
    @click="onClick"
    @blur="confirm.reset"
    @mouseleave="onLeave"
    @keydown.esc="onEscape"
  >
    <slot :state="state" :armed="armed" />
    <!-- Inside the root so the component keeps a single root element (callers focus it through
         $el and toggle it with v-show); the Teleport leaves only a placeholder in the button. -->
    <Teleport to="body">
      <div
        v-if="calloutShown"
        ref="calloutRef"
        class="app-confirm-callout"
        :data-tone="tone"
        :style="calloutStyle"
        role="status"
        aria-live="polite"
      >
        <span class="callout-prompt">{{
          armed ? confirmLabel : 'Wait for the fill, then click again.'
        }}</span>
        <span v-if="explained" class="callout-note">
          Actions confirm in place here instead of a dialog.
        </span>
      </div>
    </Teleport>
  </component>
</template>

<style scoped>
/* isolation makes the fill's negative z-index paint above the control's own background but under
   its content. */
.app-confirm {
  --confirm-color: var(--color-error);
  --confirm-muted: var(--color-error-muted);
  position: relative;
  overflow: hidden;
  isolation: isolate;
}

.app-confirm[data-tone='jade'] {
  --confirm-color: var(--color-jade);
  --confirm-muted: var(--color-jade-muted);
}

/* From the first click the control takes the confirm look, so a primary fill never shows under
   the sweep; the tone's muted colour then fills it left to right over the wait and ends exactly
   in the armed look. */
.app-confirm[data-confirm='pending'] {
  cursor: progress;
  background: transparent !important;
  border-color: var(--confirm-color) !important;
  color: var(--confirm-color) !important;
}

.app-confirm[data-confirm='pending']::before {
  content: '';
  position: absolute;
  inset: 0;
  z-index: -1;
  background: var(--confirm-muted);
  transform-origin: left;
  animation: confirm-fill var(--confirm-delay) linear forwards;
}

@keyframes confirm-fill {
  from {
    transform: scaleX(0);
  }
  to {
    transform: scaleX(1);
  }
}

/* !important because the armed look must win over any caller's hover, focus or colour rule. */
.app-confirm[data-confirm='armed'] {
  background: var(--confirm-muted) !important;
  border-color: var(--confirm-color) !important;
  color: var(--confirm-color) !important;
}

.app-confirm[data-confirm='pending'] :deep(*),
.app-confirm[data-confirm='armed'] :deep(*) {
  color: inherit !important;
}

/* A click during the wait shakes the control, so the click visibly landed. */
.app-confirm.is-nudging {
  animation: confirm-nudge 320ms ease-in-out;
}

@keyframes confirm-nudge {
  20% {
    transform: translateX(-3px);
  }
  40% {
    transform: translateX(3px);
  }
  60% {
    transform: translateX(-2px);
  }
  80% {
    transform: translateX(2px);
  }
}

@media (prefers-reduced-motion: reduce) {
  .app-confirm.is-nudging {
    animation: none;
  }
}

.app-confirm-callout {
  --confirm-color: var(--color-error);
  position: fixed;
  z-index: 1000;
  display: flex;
  flex-direction: column;
  gap: 2px;
  max-width: 280px;
  padding: var(--space-2) var(--space-3);
  background: var(--layer-float);
  border: 1px solid var(--confirm-color);
  border-radius: var(--radius-md);
  box-shadow: var(--shadow-md);
  font-family: var(--font-body);
  font-size: var(--text-xs);
  line-height: 1.4;
  pointer-events: none;
}

.app-confirm-callout[data-tone='jade'] {
  --confirm-color: var(--color-jade);
}

.callout-prompt {
  color: var(--confirm-color);
  font-weight: var(--weight-semibold);
}

.callout-note {
  color: var(--color-fg-muted);
}
</style>
