<!-- SPDX-License-Identifier: MPL-2.0 -->
<!-- Copyright (C) 2026 The Lemongrass Authors -->

<script setup lang="ts">
import { onMounted, ref, watch } from 'vue'
import { AppInput, AppSegmented } from '@sankasten'
import {
  appearance,
  loadAppearance,
  setTerminalFontSize,
  setTerminalTheme,
  setTheme,
  TERMINAL_FONT_SIZE,
  type TerminalTheme,
  type ThemeSetting
} from '../../appearance'
import {
  isMac,
  loadWindowControls,
  setWindowControlsPlacement,
  windowControls,
  type WindowControlsPlacement
} from '../../windowControls'

const themeOptions: { value: ThemeSetting; label: string }[] = [
  { value: 'system', label: 'System' },
  { value: 'light', label: 'Light' },
  { value: 'dark', label: 'Dark' }
]

const terminalOptions: { value: TerminalTheme; label: string }[] = [
  { value: 'dark', label: 'Always dark' },
  { value: 'app', label: 'Follow app' }
]

const placementOptions: { value: WindowControlsPlacement; label: string }[] = [
  { value: 'right', label: 'Right' },
  { value: 'left', label: 'Left, macOS style' }
]

// A size outside the range is ignored rather than clamped while typing, so typing 14 does not
// jump to the minimum at its first digit. Leaving the field shows the size in use.
const fontSize = ref<number | string>(appearance.terminalFontSize)
watch(
  () => appearance.terminalFontSize,
  (size) => (fontSize.value = size)
)
watch(fontSize, (size) => {
  if (typeof size !== 'number' || !Number.isInteger(size)) return
  if (size < TERMINAL_FONT_SIZE.min || size > TERMINAL_FONT_SIZE.max) return
  if (size !== appearance.terminalFontSize) setTerminalFontSize(size)
})

onMounted(() => {
  void loadAppearance()
  if (!isMac) void loadWindowControls()
})
</script>

<template>
  <div class="appearance">
    <div class="settings-field">
      <span id="appearance-theme">Theme</span>
      <AppSegmented
        :model-value="appearance.theme"
        :options="themeOptions"
        aria-labelledby="appearance-theme"
        @update:model-value="setTheme"
      />
    </div>

    <div class="settings-field">
      <span id="appearance-terminal">Terminal colors</span>
      <AppSegmented
        :model-value="appearance.terminalTheme"
        :options="terminalOptions"
        aria-labelledby="appearance-terminal"
        @update:model-value="setTerminalTheme"
      />
    </div>
    <p class="settings-text">
      Claude and Codex choose their own colors and most assume a dark terminal. Follow app makes
      terminals light in the light theme, so set the agent's own theme to match.
    </p>

    <label class="settings-field settings-field--narrow">
      Terminal font size
      <AppInput
        v-model.number="fontSize"
        type="number"
        :min="TERMINAL_FONT_SIZE.min"
        :max="TERMINAL_FONT_SIZE.max"
        step="1"
        @blur="fontSize = appearance.terminalFontSize"
      />
    </label>

    <template v-if="!isMac">
      <div class="settings-field">
        <span id="appearance-window-controls">Window controls</span>
        <AppSegmented
          :model-value="windowControls.placement"
          :options="placementOptions"
          aria-labelledby="appearance-window-controls"
          @update:model-value="setWindowControlsPlacement"
        />
      </div>
      <p class="settings-text">
        Left, macOS style puts close, minimize and maximize as three colored circles at the left of
        the header.
      </p>
    </template>
  </div>
</template>

<style scoped>
.appearance {
  display: flex;
  flex-direction: column;
  gap: var(--space-3);
}

.settings-field--narrow {
  max-width: 7rem;
}
</style>
