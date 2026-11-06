<!-- SPDX-License-Identifier: MPL-2.0 -->
<!-- Copyright (C) 2026 The Lemongrass Authors -->

<script setup lang="ts">
// One server: its name, where it points, and its actions in view. Anything else shows only when
// something is wrong or when the human asks for it.
import { computed, ref } from 'vue'
import { AppButton, AppIconButton } from '@sankasten'
import CodeReader from '../CodeReader.vue'
import { describeError } from '../../vaultSession'
import { hostTarget, missingKeyPath, tildePath } from './sshView'
import type { SSHHostBlock, SSHHostEntry, SSHKey } from '../../../../preload/types'

const props = defineProps<{
  host: SSHHostEntry
  keys: SSHKey[]
  sshDir: string
  home: string
}>()

const emit = defineEmits<{
  open: []
  edit: []
  changed: []
}>()

const block = ref<SSHHostBlock | null>(null)
const viewing = ref(false)
const wholeFile = ref(false)
const error = ref('')

const missing = computed(() => missingKeyPath(props.host, props.keys, props.sshDir))

const jump = computed(() => props.host.resolved?.proxyJump || '')

const readerText = computed(() => {
  const b = block.value
  if (!b) return ''
  if (wholeFile.value) return b.text
  return b.text
    .split(/\r?\n/)
    .slice(b.start - 1, b.end)
    .join('\n')
})

const readerHeight = computed(() => {
  const lines = readerText.value.split('\n').length
  return `${Math.min(Math.max(lines, 3), wholeFile.value ? 24 : 14) * 20 + 12}px`
})

async function toggleView(): Promise<void> {
  viewing.value = !viewing.value
  if (!viewing.value || block.value) return
  error.value = ''
  try {
    block.value = await window.api.ssh.hostSource(props.host.alias)
  } catch (err) {
    error.value = describeError(err)
  }
}

async function remove(): Promise<void> {
  error.value = ''
  try {
    await window.api.ssh.deleteHost(props.host.alias)
    emit('changed')
  } catch (err) {
    error.value = describeError(err)
  }
}
</script>

<template>
  <article class="ssh-card">
    <div class="ssh-card-head">
      <div class="ssh-card-titles">
        <span class="ssh-card-title">{{ host.alias }}</span>
        <span class="ssh-card-sub">
          {{ hostTarget(host) }}<template v-if="jump"> through {{ jump }}</template>
        </span>
      </div>
      <!-- Fixed slots, so every card's actions line up in columns even when one is missing. -->
      <div class="ssh-card-actions">
        <AppButton
          size="sm"
          :title="`Log in to ${host.alias} in a terminal tab`"
          @click="emit('open')"
        >
          Open
        </AppButton>
        <AppButton v-if="host.managed" variant="text" size="sm" @click="emit('edit')">
          Edit
        </AppButton>
        <AppButton
          v-else
          variant="text"
          size="sm"
          :pressed="viewing"
          :title="`Show the entry in ${tildePath(host.file, home)}`"
          @click="toggleView"
        >
          View
        </AppButton>
        <AppIconButton
          v-if="host.managed"
          icon="trash"
          size="sm"
          tone="danger"
          confirm
          :label="`Remove ${host.alias}`"
          @confirm="remove"
        />
        <span v-else />
      </div>
    </div>

    <p v-if="host.resolveError" class="settings-error">{{ host.resolveError }}</p>
    <p v-if="missing" class="ssh-card-warn">
      Its login key {{ tildePath(missing, home) }} is gone.
      <template v-if="host.managed">Pick another key with Edit.</template>
    </p>
    <p v-if="error" class="settings-error">{{ error }}</p>

    <div v-if="viewing && block" class="ssh-card-more">
      <div class="ssh-card-row">
        <span class="ssh-card-note">
          Defined in {{ tildePath(block.file, home) }}. Lemongrass reads the file and never changes
          it.
        </span>
        <AppButton variant="text" size="xs" @click="wholeFile = !wholeFile">
          {{ wholeFile ? `Only ${host.alias}` : 'Whole file' }}
        </AppButton>
      </div>
      <div class="ssh-reader" :style="{ height: readerHeight }">
        <CodeReader :content="readerText" :path="block.file" />
      </div>
    </div>
  </article>
</template>
