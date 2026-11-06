<!-- SPDX-License-Identifier: MPL-2.0 -->
<!-- Copyright (C) 2026 The Lemongrass Authors -->

<script setup lang="ts">
import { ref } from 'vue'
import { AppButton, AppModal } from '@sankasten'
import { describeError } from '../../vaultSession'
import { tildePath } from './sshView'
import type { SSHHostList } from '../../../../preload/types'

const props = defineProps<{
  hosts: SSHHostList
}>()

const emit = defineEmits<{
  close: []
  linked: []
}>()

const busy = ref(false)
const error = ref('')

async function link(): Promise<void> {
  busy.value = true
  error.value = ''
  try {
    await window.api.ssh.addInclude()
    emit('linked')
  } catch (err) {
    error.value = describeError(err)
  } finally {
    busy.value = false
  }
}
</script>

<template>
  <AppModal title="Link your ssh config" size="md" @close="emit('close')">
    <p class="link-text">
      Lemongrass saves servers in
      <code>{{ tildePath(props.hosts.managedFile, props.hosts.home) }}</code
      >. ssh reads them once the line below is at the top of
      <code>{{ tildePath(props.hosts.configFile, props.hosts.home) }}</code
      >, and then they work in any terminal too.
    </p>
    <pre class="link-line">{{ props.hosts.includeLine }}</pre>
    <p v-if="props.hosts.include === 'insideBlock'" class="link-text">
      Your config has the line already, but inside a Host or Match block, where it applies only to
      that block.
    </p>
    <p class="link-text">Nothing else in the file changes.</p>
    <p v-if="error" class="settings-error">{{ error }}</p>

    <template #footer>
      <AppButton @click="emit('close')">Cancel</AppButton>
      <AppButton variant="primary" :disabled="busy" @click="link">Add the line</AppButton>
    </template>
  </AppModal>
</template>

<style scoped>
.link-text {
  color: var(--color-fg-secondary);
  font-size: var(--text-sm);
  line-height: var(--leading-relaxed);
}

.link-text code {
  color: var(--color-fg-primary);
  font-family: inherit;
}

.link-line {
  margin: 0;
  padding: var(--space-2) var(--space-3);
  border-radius: var(--radius-sm);
  background: var(--layer-input);
  color: var(--color-fg-primary);
  font-family: var(--font-mono);
  font-size: var(--text-xs);
}
</style>
