<!-- SPDX-License-Identifier: Apache-2.0 -->
<!-- Copyright (C) 2026 The Sankasten Authors -->

<script setup lang="ts">
withDefaults(
  defineProps<{
    // working: something runs and needs nothing. waiting: it is held up until the human acts.
    state: 'working' | 'waiting'
    // The box it fills, the size of the icon it stands in for.
    size?: number
  }>(),
  { size: 13 }
)
</script>

<!-- Shows that an item is busy in place of its icon, in the same box, so a row or a tab keeps its
     width. One look for every source, whatever the source animates on its own. -->
<template>
  <span
    class="activity"
    :data-state="state"
    role="status"
    :aria-label="state === 'working' ? 'Working' : 'Waiting for you'"
    :style="{ width: `${size}px`, height: `${size}px` }"
  >
    <svg v-if="state === 'working'" viewBox="0 0 16 16" aria-hidden="true">
      <circle class="track" cx="8" cy="8" r="6" />
      <circle class="arc" cx="8" cy="8" r="6" />
    </svg>
    <span v-else class="dot" />
  </span>
</template>

<style scoped>
.activity {
  display: inline-flex;
  flex: none;
  align-items: center;
  justify-content: center;
}

/* The turn sits on this HTML box, not on the svg: Chromium runs an SVG element's animation on the
   main thread every frame, while an HTML box's transform animation runs on the compositor. */
.activity[data-state='working'] {
  animation: spin 0.9s linear infinite;
}

.activity svg {
  width: 100%;
  height: 100%;
}

.activity circle {
  fill: none;
  stroke-width: 2.5;
}

/* The ring takes its holder's text color, as an icon does, so it matches an active tab's amber or
   a resting row's gray instead of adding a tone of its own. The faint track keeps the ring's shape
   readable while the arc turns. */
.track {
  stroke: currentColor;
  opacity: 0.25;
}

.arc {
  stroke: currentColor;
  stroke-linecap: round;
  stroke-dasharray: 11 27;
}

/* Amber is kept for what needs the human. */
.dot {
  width: 55%;
  height: 55%;
  border-radius: var(--radius-pill);
  background: var(--color-amber);
}

/* Still, but plainly on: the arc stays drawn where it stopped. */
@media (prefers-reduced-motion: reduce) {
  .activity[data-state='working'] {
    animation: none;
  }
}
</style>
