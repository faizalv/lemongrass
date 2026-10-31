<!-- SPDX-License-Identifier: AGPL-3.0-only -->
<!-- Copyright (C) 2026 The Lemongrass Authors -->

<script setup lang="ts">
import { computed, onMounted } from 'vue'
import AppSwitcher from '../AppSwitcher.vue'
import { refreshZoomPresets, setZoom, zoom, zoomLabel } from '../../zoom'

const items = computed(() =>
  zoom.presets.map((factor) => ({ id: String(factor), label: zoomLabel(factor) }))
)

onMounted(() => void refreshZoomPresets())
</script>

<template>
  <div class="general">
    <div class="settings-field">
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

.general .settings-field {
  max-width: 12rem;
}
</style>
