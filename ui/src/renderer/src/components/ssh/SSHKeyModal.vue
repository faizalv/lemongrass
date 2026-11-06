<!-- SPDX-License-Identifier: MPL-2.0 -->
<!-- Copyright (C) 2026 The Lemongrass Authors -->

<script setup lang="ts">
// Create key. A server learns a key from its admin or its VM management page, never from the app,
// so creating one ends on the public key to hand over.
import { onBeforeUnmount, ref } from 'vue'
import { AppButton, AppIcon, AppModal } from '@sankasten'
import SSHKeyForm from './SSHKeyForm.vue'
import { describeError } from '../../vaultSession'
import type { SSHKey } from '../../../../preload/types'

defineProps<{
  existing: string[]
}>()

const emit = defineEmits<{
  close: []
  created: [key: SSHKey]
  addServer: [key: SSHKey]
}>()

const COPIED_MS = 1_500

const form = ref<InstanceType<typeof SSHKeyForm> | null>(null)
const key = ref<SSHKey | null>(null)
const publicText = ref('')
const copied = ref(false)
const error = ref('')
let copiedTimer: ReturnType<typeof setTimeout> | undefined

async function created(next: SSHKey): Promise<void> {
  key.value = next
  emit('created', next)
  try {
    publicText.value = await window.api.ssh.publicKey(next.publicPath)
  } catch (err) {
    error.value = describeError(err)
  }
}

async function copy(): Promise<void> {
  if (!key.value) return
  error.value = ''
  try {
    await window.api.ssh.copyPublicKey(key.value.publicPath)
    copied.value = true
    clearTimeout(copiedTimer)
    copiedTimer = setTimeout(() => (copied.value = false), COPIED_MS)
  } catch (err) {
    error.value = describeError(err)
  }
}

onBeforeUnmount(() => clearTimeout(copiedTimer))
</script>

<template>
  <AppModal :title="key ? `${key.name} is ready` : 'Create key'" size="md" @close="emit('close')">
    <template v-if="!key">
      <p class="key-text">
        A key pair logs you in to servers without a password. The private half stays on the
        computer, and the public half goes to each server you want to reach.
      </p>
      <SSHKeyForm ref="form" :existing="existing" @created="created" />
    </template>
    <template v-else>
      <p class="key-text">
        Paste the public key into the VM's SSH keys page, or give it to the server's admin. Once the
        server has it, the key logs you in.
      </p>
      <pre v-if="publicText" class="key-public">{{ publicText }}</pre>
      <p v-if="error" class="settings-error">{{ error }}</p>
    </template>

    <template #footer>
      <template v-if="!key">
        <AppButton @click="emit('close')">Cancel</AppButton>
        <AppButton variant="primary" :disabled="!form?.canCreate" @click="form?.create()">
          Create key
        </AppButton>
      </template>
      <template v-else>
        <AppButton @click="emit('close')">Done</AppButton>
        <AppButton @click="emit('addServer', key)">Add a server that uses it</AppButton>
        <AppButton variant="primary" @click="copy">
          <AppIcon :name="copied ? 'check' : 'copy'" />
          {{ copied ? 'Copied' : 'Copy public key' }}
        </AppButton>
      </template>
    </template>
  </AppModal>
</template>

<style scoped>
.key-text {
  color: var(--color-fg-secondary);
  font-size: var(--text-sm);
  line-height: var(--leading-relaxed);
}

/* The public key is pasted as written, so it keeps the mono face. */
.key-public {
  margin: 0;
  padding: var(--space-2) var(--space-3);
  border-radius: var(--radius-sm);
  background: var(--layer-input);
  color: var(--color-fg-primary);
  font-family: var(--font-mono);
  font-size: var(--text-xs);
  line-height: 1.55;
  white-space: pre-wrap;
  word-break: break-all;
  user-select: text;
}
</style>
