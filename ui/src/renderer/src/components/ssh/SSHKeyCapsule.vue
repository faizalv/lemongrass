<!-- SPDX-License-Identifier: MPL-2.0 -->
<!-- Copyright (C) 2026 The Lemongrass Authors -->

<script setup lang="ts">
// One key as a capsule in the SSH view's mosaic: only its name at rest; clicked open, what uses it,
// its public key and its actions. The public key is read the first time the capsule opens.
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import { AppButton, AppCapsule, AppChip, AppIcon } from '@sankasten'
import { describeError } from '../../vaultSession'
import { serversUsing } from './sshView'
import type { SSHHostEntry, SSHKey } from '../../../../preload/types'

const props = defineProps<{
  sshKey: SSHKey
  hosts: SSHHostEntry[]
  // An ssh-agent answers, so loading a key into it can work.
  agent: boolean
  expanded: boolean
}>()

const emit = defineEmits<{
  load: [key: SSHKey]
}>()

const COPIED_MS = 1_500

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

async function readPublicKey(): Promise<void> {
  if (publicText.value) return
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

watch(
  () => props.expanded,
  (expanded) => {
    if (expanded) void readPublicKey()
  },
  { immediate: true }
)

onBeforeUnmount(() => clearTimeout(copiedTimer))
</script>

<template>
  <AppCapsule :label="sshKey.name">
    <dl class="ssh-facts">
      <dt>Used by</dt>
      <dd class="ssh-facts-chips">
        <template v-if="usedBy.length">
          <AppChip v-for="host in usedBy" :key="host.alias">{{ host.alias }}</AppChip>
        </template>
        <template v-else>{{ usage }}</template>
      </dd>
      <dt>Agent</dt>
      <dd>
        <span v-if="sshKey.inAgent" class="ssh-card-ok">Loaded in ssh-agent</span>
        <template v-else>Not loaded</template>
      </dd>
      <dt>Fingerprint</dt>
      <dd>{{ sshKey.fingerprint }}</dd>
    </dl>

    <p v-if="!sshKey.hasPrivate" class="ssh-card-warn">
      Only the public half is on disk. The private key file is missing, so the key cannot log in.
    </p>
    <p v-if="error" class="settings-error">{{ error }}</p>

    <pre v-if="publicText" class="ssh-code">{{ publicText }}</pre>
    <span class="ssh-card-note">
      The public key is safe to share. Paste it into a VM's SSH keys page or GitHub, or give it to a
      server's admin.
    </span>

    <div class="ssh-capsule-actions">
      <AppButton
        v-if="sshKey.hasPrivate && agent && !sshKey.inAgent"
        variant="text"
        size="sm"
        title="ssh-agent keeps the key unlocked until you log out, so ssh stops asking for its passphrase"
        @click="emit('load', sshKey)"
      >
        Load into ssh-agent
      </AppButton>
      <span class="ssh-capsule-spacer" />
      <AppButton
        variant="primary"
        size="sm"
        title="Safe to share. Give it to a server admin, or paste it into GitHub."
        @click="copy"
      >
        <AppIcon :name="copied ? 'check' : 'copy'" />
        {{ copied ? 'Copied' : 'Copy public key' }}
      </AppButton>
    </div>
  </AppCapsule>
</template>
