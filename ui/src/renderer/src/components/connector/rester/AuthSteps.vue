<!-- SPDX-License-Identifier: MPL-2.0 -->
<!-- Copyright (C) 2026 The Lemongrass Authors -->

<script setup lang="ts">
import { AppChip, AppCodeValue, IconFrame, type IconShape } from '@sankasten'
import type { AuthStep, AuthStepKind } from '../../../authFacts'

// A login as a timeline: a numbered row per step, with an icon chip for what it does, for a fill
// the login field it types and the target it acts on as a code value. The row wraps, so a long
// target takes a line of its own instead of being cut.
defineProps<{ steps: AuthStep[] }>()

const pencil = 'M2.5 11.5l.6-2.7 6.4-6.4 2.1 2.1-6.4 6.4zM8.2 3.7l2.1 2.1'
const pointer = 'M4 2l7 5-3 .8L9 12 7.3 12.6 6.2 8.6 4 10.5z'

const ICONS: Record<AuthStepKind, IconShape[]> = {
  goto: [{ tag: 'path', attrs: { d: 'M2 7h9M8 4l3 3-3 3' } }],
  fill: [{ tag: 'path', attrs: { d: pencil } }],
  click: [{ tag: 'path', attrs: { d: pointer } }],
  tap: [{ tag: 'path', attrs: { d: pointer } }],
  wait: [
    { tag: 'circle', attrs: { cx: 7, cy: 7, r: 5 } },
    { tag: 'path', attrs: { d: 'M7 4.5V7l2 1.2' } }
  ],
  launch: [{ tag: 'path', attrs: { d: 'M4.5 3l6 4-6 4z' } }],
  key: [
    { tag: 'rect', attrs: { x: 2, y: 4, width: 10, height: 6, rx: 1.2 } },
    { tag: 'path', attrs: { d: 'M4.5 7h5' } }
  ]
}
</script>

<template>
  <ol class="auth-steps">
    <li v-for="(step, index) in steps" :key="index" class="auth-step">
      <span class="auth-step-num">{{ index + 1 }}</span>
      <span class="auth-step-body">
        <AppChip class="auth-step-action">
          <IconFrame :size="12">
            <component
              :is="shape.tag"
              v-for="(shape, i) in ICONS[step.kind]"
              :key="i"
              v-bind="shape.attrs"
            />
          </IconFrame>
          {{ step.label }}
        </AppChip>
        <AppChip v-if="step.field" tone="accent">← {{ step.field }}</AppChip>
        <AppCodeValue v-if="step.target" :value="step.target" />
      </span>
    </li>
    <li v-if="steps.length === 0" class="hint">No login steps yet.</li>
  </ol>
</template>
