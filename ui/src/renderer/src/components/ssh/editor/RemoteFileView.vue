<!-- SPDX-License-Identifier: MPL-2.0 -->
<!-- Copyright (C) 2026 The Lemongrass Authors -->

<script setup lang="ts">
// A file on a server, edited in CodeEditor and saved back through the server tab's file helper.
// It needs that tab open and logged in, and says which is missing. A save is refused when the
// server's copy moved since the read, and the banner then offers Reload or Overwrite.
import { computed, onMounted, ref } from 'vue'
import { AppButton, AppEmptyState, AppIconButton } from '@sankasten'
import CodeEditor from '../../CodeEditor.vue'
import { sshIcon } from '../../../icons'
import { openSSHServer, serverTabFor, type ProjectRef } from '../../../workspace'
import { actingAsRoot } from '../files/filesStore'
import {
  editorChanged,
  editorStateOf,
  readRemoteFile,
  remoteFileOf,
  saveRemoteFile,
  type RemoteTarget
} from './remoteFiles'

const props = defineProps<{
  project: ProjectRef
  tabId: string
  alias: string
  path: string
}>()

const file = computed(() => remoteFileOf(props.tabId))
const serverTabId = ref<string | null>(null)
const name = computed(() => props.path.slice(props.path.lastIndexOf('/') + 1))
const root = computed(() => !!serverTabId.value && actingAsRoot(serverTabId.value))

function target(): RemoteTarget | null {
  serverTabId.value = serverTabFor(props.project.id, props.tabId)
  return serverTabId.value
    ? { serverTabId: serverTabId.value, alias: props.alias, path: props.path }
    : null
}

function read(): void {
  const t = target()
  if (t) void readRemoteFile(props.tabId, t)
}

function save(force = false): void {
  const t = target()
  if (t) void saveRemoteFile(props.tabId, t, force)
  else file.value.notice = `Open the ${props.alias} server tab, then save again.`
}

function openServer(): void {
  openSSHServer(props.project, props.alias)
}

onMounted(() => {
  if (file.value.revision === 0) read()
  else target()
})
</script>

<template>
  <div class="remote-file">
    <div class="file-bar">
      <span class="file-path" :title="`${alias}:${path}`">{{ alias }}:{{ path }}</span>
      <AppButton
        variant="primary"
        size="sm"
        :disabled="file.status !== 'ready' || !file.dirty || file.saving"
        @click="save()"
      >
        {{ file.saving ? 'Saving' : 'Save' }}
      </AppButton>
    </div>

    <div v-if="root" class="banner root" role="status">Acting as root</div>
    <div v-if="file.changed" class="banner warn" role="alert">
      <span class="banner-text">Changed on the server since you opened it.</span>
      <AppButton size="sm" @click="read">Reload</AppButton>
      <AppButton variant="danger" size="sm" :disabled="file.saving" @click="save(true)">
        Overwrite
      </AppButton>
    </div>
    <div v-if="file.notice" class="banner warn" role="alert">
      <span class="banner-text">{{ file.notice }}</span>
      <AppIconButton size="xs" icon="close" label="Dismiss" @click="file.notice = ''" />
    </div>

    <AppEmptyState
      v-if="!serverTabId && file.status !== 'ready'"
      layout="fill"
      :icon="sshIcon"
      :title="`Open the ${alias} server tab to edit its files.`"
    >
      <template #action>
        <AppButton @click="openServer">Open server</AppButton>
        <AppButton variant="text" @click="read">Retry</AppButton>
      </template>
    </AppEmptyState>
    <AppEmptyState
      v-else-if="file.status === 'notLoggedIn'"
      layout="fill"
      :icon="sshIcon"
      :title="`Log in in the ${alias} terminal first.`"
      detail="The editor uses the terminal's connection."
    >
      <template #action>
        <AppButton @click="read">Retry</AppButton>
      </template>
    </AppEmptyState>
    <AppEmptyState
      v-else-if="file.status === 'binary'"
      layout="fill"
      :title="`${name} is not a text file.`"
    />
    <AppEmptyState
      v-else-if="file.status === 'tooLarge'"
      layout="fill"
      :title="`${name} is larger than 2 MB.`"
      detail="Download it from Files instead."
    />
    <AppEmptyState
      v-else-if="file.status === 'isDir'"
      layout="fill"
      :title="`${name} is a folder.`"
    />
    <AppEmptyState
      v-else-if="file.status === 'error'"
      layout="fill"
      :title="`${name} cannot be read.`"
      :detail="file.error"
    >
      <template #action>
        <AppButton @click="read">Retry</AppButton>
      </template>
    </AppEmptyState>
    <AppEmptyState
      v-else-if="file.status === 'loading'"
      layout="fill"
      loading
      :title="`Opening ${name}`"
    />
    <CodeEditor
      v-else
      :content="file.content"
      :path="path"
      :revision="file.revision"
      :restore="editorStateOf(tabId)"
      @change="(state) => editorChanged(tabId, state)"
      @save="save()"
    />
  </div>
</template>

<style scoped>
.remote-file {
  flex: 1;
  min-height: 0;
  display: flex;
  flex-direction: column;
  font-family: var(--font-body);
}

.file-bar {
  display: flex;
  align-items: center;
  gap: var(--space-2);
  min-height: 38px;
  padding: 0 var(--space-3);
  border-bottom: 1px solid var(--color-border-subtle);
}

.file-path {
  flex: 1;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  color: var(--color-fg-muted);
  font-size: var(--text-xs);
}

.banner {
  display: flex;
  align-items: center;
  gap: var(--space-2);
  min-height: 34px;
  padding: 0 var(--space-3);
  border-bottom: 1px solid var(--color-border-subtle);
  font-size: var(--text-xs);
}

.banner.root {
  min-height: 28px;
  border-bottom-color: var(--color-error);
  background: var(--color-error-muted);
  color: var(--color-error);
  font-weight: var(--weight-medium);
}

.banner.warn {
  color: var(--color-error);
}

.banner-text {
  flex: 1;
  min-width: 0;
  padding: var(--space-1) 0;
}
</style>
