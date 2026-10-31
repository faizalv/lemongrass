<!-- SPDX-License-Identifier: AGPL-3.0-only -->
<!-- Copyright (C) 2026 The Lemongrass Authors -->

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import AppSegmented from '../AppSegmented.vue'
import AppSwitcher from '../AppSwitcher.vue'
import { refreshZoomPresets, setZoom, zoom, zoomLabel } from '../../zoom'

type Startup = 'lastProject' | 'projectSelector'

const startupOptions: { value: Startup; label: string }[] = [
  { value: 'lastProject', label: 'Last project' },
  { value: 'projectSelector', label: 'Project selector' }
]
const startup = ref<Startup>('lastProject')

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
