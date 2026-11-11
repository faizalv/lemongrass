<!-- SPDX-License-Identifier: MPL-2.0 -->
<!-- Copyright (C) 2026 The Lemongrass Authors -->

<script setup lang="ts">
import { AppIconButton, capsuleHue } from '@sankasten'
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import NotificationBell from './NotificationBell.vue'
import { isMac, loadWindowControls, windowControls } from '../windowControls'
import { zoom } from '../zoom'
import { updatePending } from '../updateState'

// Aliased so the template can reach it -- a bare `window` identifier in
// a Vue template resolves against the component instance, not globalThis.
const controls = window.api.windowControls

const props = defineProps<{
  // The open project's name; the header's gradient takes its hue, as its monogram does, so the
  // windows of different projects tell apart. Amber when none is open.
  projectName?: string
}>()

const headerHue = computed(() =>
  props.projectName ? `var(--color-${capsuleHue(props.projectName)})` : 'var(--color-amber)'
)

const emit = defineEmits<{
  'open-settings': []
  'open-guide': []
}>()

// Matches .header-bar's height, which main needs in device pixels to center the macOS lights.
const HEADER_HEIGHT = 46
// Room the macOS traffic lights take at the header's left, in device pixels: their 14 px inset
// in main/windowControls.ts plus three buttons 20 px apart.
const MAC_LIGHTS_WIDTH = 72

const isMaximized = ref(false)
const isFullScreen = ref(false)
const isFocused = ref(document.hasFocus())
const unsubscribers: (() => void)[] = []

const lightsLeft = computed(() => !isMac && windowControls.placement === 'left')
// A device-pixel width becomes fewer CSS pixels as the zoom grows. macOS hides the lights in
// full screen, so the room goes too.
const macInset = computed(() =>
  isMac && !isFullScreen.value ? `${MAC_LIGHTS_WIDTH / zoom.factor}px` : '0px'
)

function onFocus(): void {
  isFocused.value = true
}
function onBlur(): void {
  isFocused.value = false
}

// Not immediate: main placed the lights for the saved zoom when it created the window, so only a
// later change needs reporting.
if (isMac)
  watch(
    () => zoom.factor,
    (factor) => controls.setHeaderHeight(HEADER_HEIGHT * factor)
  )

onMounted(async () => {
  void loadWindowControls()
  window.addEventListener('focus', onFocus)
  window.addEventListener('blur', onBlur)
  unsubscribers.push(
    controls.onMaximizedChange((maximized) => {
      isMaximized.value = maximized
    }),
    controls.onFullScreenChange((fullScreen) => {
      isFullScreen.value = fullScreen
    })
  )
  isMaximized.value = await controls.isMaximized()
  if (isMac) isFullScreen.value = await controls.isFullScreen()
})

onBeforeUnmount(() => {
  window.removeEventListener('focus', onFocus)
  window.removeEventListener('blur', onBlur)
  for (const unsubscribe of unsubscribers) unsubscribe()
})
</script>

<template>
  <div class="header-bar" :style="{ '--mac-lights-inset': macInset, '--header-hue': headerHue }">
    <div
      v-if="lightsLeft"
      class="lights"
      :class="{ inactive: !isFocused }"
      role="group"
      aria-label="Window controls"
    >
      <button class="light light-close" title="Close" aria-label="Close" @click="controls.close()">
        <svg width="8" height="8" viewBox="0 0 8 8">
          <line x1="1.5" y1="1.5" x2="6.5" y2="6.5" />
          <line x1="6.5" y1="1.5" x2="1.5" y2="6.5" />
        </svg>
      </button>
      <button
        class="light light-minimize"
        title="Minimize"
        aria-label="Minimize"
        @click="controls.minimize()"
      >
        <svg width="8" height="8" viewBox="0 0 8 8">
          <line x1="1" y1="4" x2="7" y2="4" />
        </svg>
      </button>
      <button
        class="light light-maximize"
        :title="isMaximized ? 'Restore' : 'Maximize'"
        :aria-label="isMaximized ? 'Restore' : 'Maximize'"
        @click="controls.toggleMaximize()"
      >
        <svg width="8" height="8" viewBox="0 0 8 8">
          <line x1="1" y1="4" x2="7" y2="4" />
          <line v-if="!isMaximized" x1="4" y1="1" x2="4" y2="7" />
        </svg>
      </button>
    </div>
    <div class="drag-region">
      <slot name="title" />
    </div>
    <!-- Laid out by its content over the header's middle, outside the drag region. -->
    <slot name="center" />
    <div class="actions">
      <slot name="actions" />
    </div>
    <div class="window-controls">
      <NotificationBell />
      <AppIconButton label="Guide" size="sm" @click="emit('open-guide')">
        <svg
          viewBox="0 0 24 24"
          fill="none"
          stroke="currentColor"
          stroke-width="1.6"
          stroke-linecap="round"
          stroke-linejoin="round"
        >
          <circle cx="12" cy="12" r="10" />
          <path d="M9.09 9a3 3 0 0 1 5.83 1c0 2-3 3-3 3" />
          <line x1="12" y1="17" x2="12.01" y2="17" />
        </svg>
      </AppIconButton>
      <AppIconButton
        :label="updatePending ? 'Settings. An update is available' : 'Settings'"
        size="sm"
        :dot="updatePending ? 'accent' : undefined"
        @click="emit('open-settings')"
      >
        <svg
          viewBox="0 0 24 24"
          fill="none"
          stroke="currentColor"
          stroke-width="1.6"
          stroke-linecap="round"
          stroke-linejoin="round"
        >
          <path
            d="M12.22 2h-.44a2 2 0 0 0-2 2v.18a2 2 0 0 1-1 1.73l-.43.25a2 2 0 0 1-2 0l-.15-.08a2 2 0 0 0-2.73.73l-.22.38a2 2 0 0 0 .73 2.73l.15.1a2 2 0 0 1 1 1.72v.51a2 2 0 0 1-1 1.74l-.15.09a2 2 0 0 0-.73 2.73l.22.38a2 2 0 0 0 2.73.73l.15-.08a2 2 0 0 1 2 0l.43.25a2 2 0 0 1 1 1.73V20a2 2 0 0 0 2 2h.44a2 2 0 0 0 2-2v-.18a2 2 0 0 1 1-1.73l.43-.25a2 2 0 0 1 2 0l.15.08a2 2 0 0 0 2.73-.73l.22-.39a2 2 0 0 0-.73-2.73l-.15-.08a2 2 0 0 1-1-1.74v-.5a2 2 0 0 1 1-1.74l.15-.09a2 2 0 0 0 .73-2.73l-.22-.38a2 2 0 0 0-2.73-.73l-.15.08a2 2 0 0 1-2 0l-.43-.25a2 2 0 0 1-1-1.73V4a2 2 0 0 0-2-2z"
          />
          <circle cx="12" cy="12" r="3" />
        </svg>
      </AppIconButton>
      <template v-if="!isMac && !lightsLeft">
        <AppIconButton label="Minimize" size="sm" @click="controls.minimize()">
          <svg viewBox="0 0 10 10">
            <line x1="1" y1="8" x2="9" y2="8" stroke="currentColor" stroke-width="1" />
          </svg>
        </AppIconButton>
        <AppIconButton
          :label="isMaximized ? 'Restore' : 'Maximize'"
          size="sm"
          @click="controls.toggleMaximize()"
        >
          <svg v-if="!isMaximized" viewBox="0 0 10 10">
            <rect
              x="1"
              y="1"
              width="8"
              height="8"
              fill="none"
              stroke="currentColor"
              stroke-width="1"
            />
          </svg>
          <svg v-else viewBox="0 0 10 10">
            <rect
              x="0.5"
              y="2.5"
              width="6"
              height="6"
              fill="none"
              stroke="currentColor"
              stroke-width="1"
            />
            <path d="M2.5 2.5V0.5H9.5V7.5H7.5" fill="none" stroke="currentColor" stroke-width="1" />
          </svg>
        </AppIconButton>
        <AppIconButton label="Close" size="sm" tone="danger" @click="controls.close()">
          <svg viewBox="0 0 10 10">
            <line x1="1" y1="1" x2="9" y2="9" stroke="currentColor" stroke-width="1" />
            <line x1="9" y1="1" x2="1" y2="9" stroke="currentColor" stroke-width="1" />
          </svg>
        </AppIconButton>
      </template>
    </div>
  </div>
</template>

<style scoped>
.header-bar {
  position: relative;
  display: flex;
  align-items: stretch;
  height: 46px;
  flex-shrink: 0;
  /* Mixed into the surface, not layered with alpha, so the bar stays opaque. The hue comes from
     the template, amber when no project is open. */
  background: linear-gradient(
    90deg,
    color-mix(in oklch, var(--header-hue) var(--header-tint-start), var(--color-surface-1)),
    color-mix(in oklch, var(--header-hue) var(--header-tint-end), var(--color-surface-1)) 50%
  );
  -webkit-app-region: drag;
}

.drag-region {
  flex: 1;
  min-width: 0;
  display: flex;
  align-items: center;
  /* Keeps the title clear of the macOS traffic lights; 0 elsewhere. */
  margin-left: var(--mac-lights-inset);
  padding: 0 var(--space-4);
}

.lights {
  display: flex;
  align-items: center;
  padding: 0 0 0 var(--space-2);
  -webkit-app-region: no-drag;
}

/* A 20 px hit area around a 12 px circle, so the circles sit 20 px apart as on macOS. */
.light {
  position: relative;
  display: flex;
  align-items: center;
  justify-content: center;
  width: 20px;
  height: 20px;
  padding: 0;
  background: transparent;
  border: none;
  border-radius: var(--radius-pill);
  cursor: pointer;
}

.light::before {
  content: '';
  position: absolute;
  width: 12px;
  height: 12px;
  border-radius: 50%;
  background: var(--light-color);
}

.light svg {
  position: relative;
  fill: none;
  stroke: var(--color-window-glyph);
  stroke-width: 1.2;
  stroke-linecap: round;
  opacity: 0;
}

.light-close {
  --light-color: var(--color-window-close);
}

.light-minimize {
  --light-color: var(--color-window-minimize);
}

.light-maximize {
  --light-color: var(--color-window-maximize);
}

/* As on macOS, hovering any light shows the glyphs on all three. */
.lights:hover .light svg,
.light:focus-visible svg {
  opacity: 1;
}

.lights.inactive:not(:hover) .light {
  --light-color: var(--color-window-inactive);
}

.light:focus-visible {
  outline: 2px solid var(--color-fg-accent);
  outline-offset: 0;
}

.actions {
  display: flex;
  align-items: center;
  padding: 0 var(--space-2);
  -webkit-app-region: no-drag;
}

.window-controls {
  display: flex;
  align-items: center;
  gap: var(--space-1);
  padding: 0 var(--space-2);
  -webkit-app-region: no-drag;
}
</style>
