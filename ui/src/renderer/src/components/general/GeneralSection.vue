<!-- SPDX-License-Identifier: MPL-2.0 -->
<!-- Copyright (C) 2026 The Lemongrass Authors -->

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { AppSegmented, AppSwitcher } from '@sankasten'
import { refreshZoomPresets, setZoom, zoom, zoomLabel } from '../../zoom'
import { AUTO_LOCK_CHOICES, loadVaultAutoLock, setVaultAutoLock } from '../../vaultSession'
import { RUN_LIMIT_CHOICES } from '../../../../preload/resterHistory'
import { loadRunLimit, setRunLimit } from '../../resterClientStore'

type Startup = 'lastProject' | 'projectSelector'

const startupOptions: { value: Startup; label: string }[] = [
  { value: 'lastProject', label: 'Last project' },
  { value: 'projectSelector', label: 'Project selector' }
]
const startup = ref<Startup>('lastProject')

const autoLockOptions = AUTO_LOCK_CHOICES.map((minutes) => ({
  value: String(minutes),
  label: `${minutes} min`
}))
const autoLock = ref('5')

function changeAutoLock(value: string): void {
  autoLock.value = value
  setVaultAutoLock(Number(value))
}

const runLimitOptions = RUN_LIMIT_CHOICES.map((runs) => ({
  value: String(runs),
  label: `${runs} runs`
}))
const runLimit = ref('10')

function changeRunLimit(value: string): void {
  runLimit.value = value
  setRunLimit(Number(value))
}

const items = computed(() =>
  zoom.presets.map((factor) => ({ id: String(factor), label: zoomLabel(factor) }))
)

function setStartup(value: Startup): void {
  startup.value = value
  window.api.uiState.save({ startup: value })
}

onMounted(async () => {
  void refreshZoomPresets()
  startup.value = (await window.api.uiState.load()).startup ?? 'lastProject'
  autoLock.value = String(await loadVaultAutoLock())
  runLimit.value = String(await loadRunLimit())
})
</script>

<template>
  <div class="general">
    <div class="settings-field">
      <span id="general-startup">On startup</span>
      <AppSegmented
        :model-value="startup"
        :options="startupOptions"
        aria-labelledby="general-startup"
        @update:model-value="setStartup"
      />
    </div>
    <p class="settings-text">
      Project selector opens no project at start, so no tab or agent starts until you pick one.
    </p>

    <div class="settings-field">
      <span id="general-vault-lock">Vault auto-lock</span>
      <AppSegmented
        :model-value="autoLock"
        :options="autoLockOptions"
        aria-labelledby="general-vault-lock"
        @update:model-value="changeAutoLock"
      />
    </div>
    <p class="settings-text">
      The vault locks after this long without a click or keypress in a Vault tab, the Rester Client
      or a decision.
    </p>

    <div class="settings-field">
      <span id="general-rester-history">Rester Client history</span>
      <AppSegmented
        :model-value="runLimit"
        :options="runLimitOptions"
        aria-labelledby="general-rester-history"
        @update:model-value="changeRunLimit"
      />
    </div>
    <p class="settings-text">
      The most runs each request keeps. A request's runs together stay under 10 MB, so large
      responses keep fewer, and the latest run is always kept.
    </p>

    <div class="settings-field settings-field--zoom">
      <span>Zoom</span>
      <AppSwitcher
        :items="items"
        :active-id="String(zoom.factor)"
        :label="zoomLabel(zoom.factor)"
        placeholder="Zoom"
        empty-text="No sizes"
        framed
        @select="(id) => setZoom(Number(id))"
      />
    </div>
    <p class="settings-text">
      Ctrl+= and Ctrl+- step through these sizes, and Ctrl+0 goes back to 100%. A wider screen
      offers larger sizes.
    </p>
  </div>
</template>

<style scoped>
.general {
  display: flex;
  flex-direction: column;
  gap: var(--space-3);
}

.settings-field--zoom {
  max-width: 12rem;
  margin-top: var(--space-2);
}
</style>
