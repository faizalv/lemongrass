<!-- SPDX-License-Identifier: MPL-2.0 -->
<!-- Copyright (C) 2026 The Lemongrass Authors -->

<script setup lang="ts">
// A server tab: a terminal logged in to one server. The ssh command line comes from main, which
// owns the ControlMaster socket folder; the terminal itself is an ordinary shell registry session,
// so it keeps running while the tab is hidden or moved.
import { onMounted, ref } from 'vue'
import { AppEmptyState } from '@sankasten'
import ShellView from '../ShellView.vue'
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
  <ShellView v-if="spec" :spec="spec" :active="active" />
  <AppEmptyState
    v-else-if="error"
    layout="fill"
    :icon="sshIcon"
    :title="`${alias} cannot open.`"
    :detail="error"
  />
</template>
