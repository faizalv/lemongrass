<!-- SPDX-License-Identifier: MPL-2.0 -->
<!-- Copyright (C) 2026 The Lemongrass Authors -->

<script setup lang="ts">
// Loads a key into ssh-agent (ssh-add). It says what that does first, then runs in a terminal the
// human types the passphrase into.
import { computed, ref } from 'vue'
import { AppButton, AppModal } from '@sankasten'
import SSHRunTerminal from './SSHRunTerminal.vue'
import type { SSHKey } from '../../../../preload/types'

const props = defineProps<{
  sshKey: SSHKey
}>()

const emit = defineEmits<{
  close: []
  finished: []
}>()

type Phase = 'intro' | 'running' | 'done'

const phase = ref<Phase>('intro')
const exitCode = ref<number | null>(null)
const error = ref('')

const result = computed(() => {
  if (phase.value !== 'done' || exitCode.value === null) return ''
  if (exitCode.value !== 0) {
    return `Stopped with exit code ${exitCode.value}. The terminal shows the reason.`
  }
  return `${props.sshKey.name} is loaded. ssh stops asking for its passphrase until you log out.`
})

function onExit(code: number): void {
  exitCode.value = code
  phase.value = 'done'
  emit('finished')
}

function onFailed(message: string): void {
  error.value = message
  phase.value = 'intro'
}
</script>

<template>
  <AppModal :title="`Load ${sshKey.name} into ssh-agent`" size="md" @close="emit('close')">
    <p class="run-text">
      ssh-agent keeps the key unlocked until you log out, so ssh stops asking for its passphrase on
      every connection. Type the passphrase once in the terminal below.
    </p>
    <SSHRunTerminal
      v-if="phase !== 'intro'"
      :request="{ kind: 'addToAgent', keyPath: sshKey.privatePath }"
      @exit="onExit"
      @failed="onFailed"
    />
    <p v-if="result" class="run-status" :class="{ ok: exitCode === 0 }">{{ result }}</p>
    <p v-if="error" class="settings-error">{{ error }}</p>

    <template #footer>
      <AppButton @click="emit('close')">
        {{ phase === 'running' ? 'Stop' : phase === 'done' ? 'Close' : 'Not now' }}
      </AppButton>
      <AppButton
        v-if="phase === 'intro'"
        variant="primary"
        @click="((error = ''), (phase = 'running'))"
      >
        Load key
      </AppButton>
    </template>
  </AppModal>
</template>

<style scoped>
.run-text {
  color: var(--color-fg-secondary);
  font-size: var(--text-sm);
  line-height: var(--leading-relaxed);
}

.run-status {
  color: var(--color-error);
  font-size: var(--text-sm);
}

.run-status.ok {
  color: var(--color-success);
}
</style>
