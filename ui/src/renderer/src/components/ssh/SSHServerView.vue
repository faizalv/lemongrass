<!-- SPDX-License-Identifier: MPL-2.0 -->
<!-- Copyright (C) 2026 The Lemongrass Authors -->

<script setup lang="ts">
// A server tab: a terminal logged in to one server, with the Files layer over its right side.
// The ssh command line comes from main, which owns the ControlMaster socket folder; the terminal
// itself is an ordinary shell registry session, so it keeps running while the tab is hidden or
// moved.
import { computed, onMounted, ref } from 'vue'
import { AppButton, AppEmptyState } from '@sankasten'
import ShellView from '../ShellView.vue'
import TabIcon from '../TabIcon.vue'
import FilesLayer from './files/FilesLayer.vue'
import { FILES_PATHS, shapesOf } from './files/drawerIcons'
import { filesOf } from './files/filesStore'
import { describeError } from '../../vaultSession'
import { sshIcon } from '../../icons'
import type { ShellSpec } from '../../shellRegistry'

const props = defineProps<{
  tabId: string
  alias: string
  active: boolean
}>()

const spec = ref<ShellSpec | null>(null)
const error = ref('')
const files = computed(() => filesOf(props.tabId, props.alias))

onMounted(async () => {
  try {
    const args = await window.api.ssh.terminalArgs(props.alias)
    spec.value = { id: props.tabId, command: 'ssh', args }
  } catch (err) {
    error.value = describeError(err)
  }
})
</script>

<template>
  <div v-if="spec" class="server-view">
    <ShellView :spec="spec" :active="active" />
    <FilesLayer v-if="files.open" :tab-id="tabId" :alias="alias" @close="files.open = false" />
    <!-- Over terminal text, so it keeps a solid fill; the layer's own Close replaces it while open. -->
    <AppButton v-else class="files-toggle" variant="primary" size="sm" @click="files.open = true">
      <TabIcon :shapes="shapesOf(FILES_PATHS.files)" />
      Files
    </AppButton>
  </div>
  <AppEmptyState
    v-else-if="error"
    layout="fill"
    :icon="sshIcon"
    :title="`${alias} cannot open.`"
    :detail="error"
  />
</template>

<style scoped>
.server-view {
  position: relative;
  flex: 1;
  min-height: 0;
  display: flex;
  flex-direction: column;
}

.server-view .files-toggle {
  position: absolute;
  top: 5px;
  right: var(--space-4);
  z-index: 3;
  gap: var(--space-1);
}
</style>
