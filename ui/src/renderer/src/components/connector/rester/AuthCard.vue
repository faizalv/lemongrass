<!-- SPDX-License-Identifier: MPL-2.0 -->
<!-- Copyright (C) 2026 The Lemongrass Authors -->

<script setup lang="ts">
import { ref, useId } from 'vue'
import { AppButton, AppChip, AppIconButton, AppInfoPopover } from '@sankasten'
import ChannelTypeIcon from '../ChannelTypeIcon.vue'

// One login setup as a card: what it signs in to and whether it is on, its values at rest (the
// default slot, shown while it is on), an info icon whose popover (the info slot) says how to use
// it, and the button that opens its editor. While it is edited the card spans the row, moves to the
// top and shows its editor (the editor slot) in place of its values.
defineProps<{
  kind: 'api' | 'web' | 'app'
  label: string
  purpose: string
  on: boolean
  // Its editor is open.
  editing: boolean
  // What an off setup needs before it can be turned on.
  offHint: string
}>()

const emit = defineEmits<{ edit: [] }>()

const infoId = useId()
const anchor = ref<HTMLElement | null>(null)

function show(event: Event): void {
  anchor.value = event.currentTarget as HTMLElement
}

function hide(): void {
  anchor.value = null
}
</script>

<template>
  <div class="auth-card" :class="{ on, editing, 'sk-ribbon sk-ribbon--top': on }">
    <div class="auth-card-head">
      <ChannelTypeIcon class="auth-card-icon" :type="kind" :size="16" />
      <span class="auth-card-name">{{ label }}</span>
      <AppChip :tone="on ? 'success' : 'quiet'">{{ on ? 'On' : 'Off' }}</AppChip>
      <span class="auth-card-fill"></span>
      <AppIconButton
        icon="info"
        size="sm"
        :label="`About ${label}`"
        title=""
        :aria-describedby="anchor ? infoId : undefined"
        @mouseenter="show"
        @mouseleave="hide"
        @focus="show"
        @blur="hide"
      />
    </div>
    <slot v-if="editing" name="editor" />
    <template v-else>
      <p class="hint">{{ purpose }}</p>
      <div class="auth-card-body">
        <slot v-if="on" />
        <p v-else class="hint">{{ offHint }}</p>
      </div>
      <div class="auth-card-actions">
        <AppButton size="sm" :variant="on ? 'ghost' : 'primary'" @click="emit('edit')">
          {{ on ? 'Edit' : 'Set up' }}
        </AppButton>
      </div>
    </template>
    <AppInfoPopover :id="infoId" :anchor="anchor"><slot name="info" /></AppInfoPopover>
  </div>
</template>
