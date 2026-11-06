<!-- SPDX-License-Identifier: MPL-2.0 -->
<!-- Copyright (C) 2026 The Lemongrass Authors -->

<script setup lang="ts">
// One key: its name, what uses it, and Copy public key. The public key and the rarer actions
// unfold on request.
import { computed, onBeforeUnmount, ref } from 'vue'
import { AppButton, AppChip, AppDisclosure, AppIcon } from '@sankasten'
import { describeError } from '../../vaultSession'
import { serversUsing } from './sshView'
import type { SSHHostEntry, SSHKey } from '../../../../preload/types'

const props = defineProps<{
  sshKey: SSHKey
  hosts: SSHHostEntry[]
  // An ssh-agent answers, so loading a key into it can work.
  agent: boolean
}>()

const emit = defineEmits<{
  load: [key: SSHKey]
}>()

const COPIED_MS = 1_500

const open = ref(false)
const publicText = ref('')
const copied = ref(false)
const error = ref('')
let copiedTimer: ReturnType<typeof setTimeout> | undefined

const usedBy = computed(() => serversUsing(props.sshKey, props.hosts))

const usage = computed(() => {
  const n = usedBy.value.length
  if (n === 0) return 'Not used by any server'
  return n === 1 ? 'Used by 1 server' : `Used by ${n} servers`
})

async function toggle(next: boolean): Promise<void> {
  open.value = next
  if (!next || publicText.value) return
  error.value = ''
  try {
    publicText.value = await window.api.ssh.publicKey(props.sshKey.publicPath)
  } catch (err) {
    error.value = describeError(err)
  }
}

async function copy(): Promise<void> {
  error.value = ''
  try {
    await window.api.ssh.copyPublicKey(props.sshKey.publicPath)
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
  <article class="ssh-card">
    <div class="ssh-card-head">
      <div class="ssh-card-titles">
        <AppDisclosure
          class="ssh-card-fold"
          :open="open"
          :label="sshKey.name"
          @update:open="toggle"
        />
        <span class="ssh-card-sub">
          {{ usage }}
          <span v-if="sshKey.inAgent" class="ssh-card-ok">· loaded in ssh-agent</span>
        </span>
      </div>
      <AppButton
        size="sm"
        title="Safe to share. Give it to a server admin, or paste it into GitHub."
        @click="copy"
      >
        <AppIcon :name="copied ? 'check' : 'copy'" />
        {{ copied ? 'Copied' : 'Copy public key' }}
      </AppButton>
    </div>

    <p v-if="!sshKey.hasPrivate" class="ssh-card-warn">
      Only the public half is on disk. The private key file is missing, so the key cannot log in.
    </p>
    <p v-if="error" class="settings-error">{{ error }}</p>

    <div v-if="open" class="ssh-card-more">
      <pre v-if="publicText" class="ssh-code">{{ publicText }}</pre>
      <span class="ssh-card-note">
        The public key is safe to share. Paste it into a VM's SSH keys page or GitHub, or give it to
        a server's admin. Fingerprint {{ sshKey.fingerprint }}.
      </span>
      <div v-if="usedBy.length" class="ssh-card-row">
        <span class="ssh-card-label">Used by</span>
        <AppChip v-for="host in usedBy" :key="host.alias">{{ host.alias }}</AppChip>
      </div>
      <div v-if="sshKey.hasPrivate && agent && !sshKey.inAgent" class="ssh-card-row">
        <AppButton
          size="sm"
          title="ssh-agent keeps the key unlocked until you log out, so ssh stops asking for its passphrase"
          @click="emit('load', sshKey)"
        >
          Load into ssh-agent
        </AppButton>
      </div>
    </div>
  </article>
</template>
