<!-- SPDX-License-Identifier: MPL-2.0 -->
<!-- Copyright (C) 2026 The Lemongrass Authors -->

<script setup lang="ts">
// One server as a capsule in the SSH view's mosaic: its name and open tab count at rest; clicked
// open, where it points, the key it logs in with, its open tabs and its actions. Its config entry
// shows only when asked for.
import { computed, ref } from 'vue'
import { AppButton, AppCapsule, AppIconButton, AppNavItem } from '@sankasten'
import CodeReader from '../CodeReader.vue'
import { describeError } from '../../vaultSession'
import { filesFolderOf } from './files/filesStore'
import { missingKeyPath, tildePath } from './sshView'
import type { SSHHostBlock, SSHHostEntry, SSHKey } from '../../../../preload/types'

const props = defineProps<{
  host: SSHHostEntry
  keys: SSHKey[]
  sshDir: string
  home: string
  // Every tab of this server shares one title, so a row tells them apart by its place in pane
  // order and the folder its Files layer shows.
  openTabs: { tabId: string; paneId: string; showing: boolean }[]
}>()

const emit = defineEmits<{
  open: []
  focus: [tabId: string, paneId: string]
  edit: []
  changed: []
}>()

const block = ref<SSHHostBlock | null>(null)
const viewing = ref(false)
const wholeFile = ref(false)
const error = ref('')

const missing = computed(() => missingKeyPath(props.host, props.keys, props.sshDir))

const jump = computed(() => props.host.resolved?.proxyJump || '')

// The settings ssh resolves for the host, the ones a login depends on; empty values are left out.
const facts = computed(() => {
  const r = props.host.resolved
  const rows: { label: string; value: string }[] = []
  if (r?.hostname) rows.push({ label: 'Address', value: r.hostname })
  if (r?.user) rows.push({ label: 'User', value: r.user })
  rows.push({ label: 'Port', value: r?.port || '22' })
  rows.push({
    label: 'Key',
    value: props.host.keyPaths.length
      ? props.host.keyPaths.map((path) => tildePath(path, props.home)).join(', ')
      : 'ssh default keys'
  })
  if (jump.value) rows.push({ label: 'Through', value: jump.value })
  return rows
})

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
  <AppCapsule
    :label="host.alias"
    :count="openTabs.length"
    :count-title="openTabs.length === 1 ? '1 tab open' : `${openTabs.length} tabs open`"
  >
    <dl v-if="host.resolved" class="ssh-facts">
      <template v-for="fact in facts" :key="fact.label">
        <dt>{{ fact.label }}</dt>
        <dd>{{ fact.value }}</dd>
      </template>
    </dl>

    <div v-if="openTabs.length" class="ssh-open-tabs" aria-label="Open tabs">
      <AppNavItem
        v-for="(open, index) in openTabs"
        :key="open.tabId"
        size="sm"
        :label="`Tab ${index + 1}`"
        :state="filesFolderOf(open.tabId) ?? 'Terminal only'"
        :active="open.showing"
        :title="`Go to tab ${index + 1} of ${host.alias}`"
        @click="emit('focus', open.tabId, open.paneId)"
      />
    </div>

    <p v-if="host.resolveError" class="settings-error">{{ host.resolveError }}</p>
    <p v-if="missing" class="ssh-card-warn">
      Its login key {{ tildePath(missing, home) }} is gone.
      <template v-if="host.managed">Pick another key with Edit.</template>
    </p>
    <p v-if="error" class="settings-error">{{ error }}</p>

    <div class="ssh-capsule-actions">
      <AppIconButton
        v-if="host.managed"
        icon="trash"
        size="sm"
        tone="danger"
        confirm
        :label="`Remove ${host.alias}`"
        @confirm="remove"
      />
      <span class="ssh-capsule-spacer" />
      <AppButton v-if="host.managed" variant="text" size="sm" @click="emit('edit')">
        Edit
      </AppButton>
      <AppButton
        variant="text"
        size="sm"
        :pressed="viewing"
        :title="`Show the entry in ${tildePath(host.file, home)}`"
        @click="toggleView"
      >
        View
      </AppButton>
      <AppButton
        variant="primary"
        size="sm"
        :title="`Log in to ${host.alias} in a terminal tab`"
        @click="emit('open')"
      >
        Open
      </AppButton>
    </div>

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
  </AppCapsule>
</template>
