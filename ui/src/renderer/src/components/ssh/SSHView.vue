<!-- SPDX-License-Identifier: MPL-2.0 -->
<!-- Copyright (C) 2026 The Lemongrass Authors -->

<script setup lang="ts">
// The SSH tab: the servers you connect to and the keys you log in with, shaped like the Vault. It
// reads the ssh setup each time the tab is shown and runs nothing while hidden.
import { computed, ref, watch } from 'vue'
import { AppBanner, AppButton, AppEmptyState, AppInput, AppNavItem } from '@sankasten'
import '../../ssh.css'
import { describeError } from '../../vaultSession'
import { keyIcon, sshIcon } from '../../icons'
import { matchesQuery } from '../connector/listFilter'
import { openSSHServer, type ProjectRef } from '../../workspace'
import SSHKeyCard from './SSHKeyCard.vue'
import SSHKeyModal from './SSHKeyModal.vue'
import SSHLinkModal from './SSHLinkModal.vue'
import SSHRunModal from './SSHRunModal.vue'
import SSHServerCard from './SSHServerCard.vue'
import SSHServerModal from './SSHServerModal.vue'
import { hostTarget, sshDirOf, tildePath, type SSHSection } from './sshView'
import type { SSHHostEntry, SSHHostList, SSHKey, SSHKeyList } from '../../../../preload/types'

const props = defineProps<{
  active: boolean
  project: ProjectRef
}>()

const keys = ref<SSHKeyList | null>(null)
const hosts = ref<SSHHostList | null>(null)
const error = ref('')
const section = ref<SSHSection>('servers')
const query = ref('')

const keyModal = ref(false)
const serverModal = ref<{ host: SSHHostEntry | null; presetKey: SSHKey | null } | null>(null)
const linkModal = ref(false)
const loadModal = ref<SSHKey | null>(null)

const allHosts = computed(() => hosts.value?.hosts ?? [])
const allKeys = computed(() => keys.value?.keys ?? [])
const home = computed(() => hosts.value?.home ?? '')
const sshDir = computed(() => (hosts.value ? sshDirOf(hosts.value) : ''))
const loaded = computed(() => keys.value !== null && hosts.value !== null)

const serverRows = computed(() =>
  allHosts.value.filter((h) => matchesQuery(query.value, [h.alias, hostTarget(h)]))
)

const keyRows = computed(() =>
  allKeys.value.filter((k) => matchesQuery(query.value, [k.name, k.comment]))
)

// Servers saved in lemongrass's file that ssh does not read yet, because the Include line is
// missing or sits inside a block.
const unlinked = computed(
  () => !!hosts.value && hosts.value.include !== 'included' && allHosts.value.some((h) => h.managed)
)

async function refresh(): Promise<void> {
  error.value = ''
  try {
    const [k, h] = await Promise.all([window.api.ssh.listKeys(), window.api.ssh.listHosts()])
    keys.value = k
    hosts.value = h
  } catch (err) {
    error.value = describeError(err)
  }
}

watch(
  () => props.active,
  (active) => {
    if (active) void refresh()
  },
  { immediate: true }
)

watch(section, () => (query.value = ''))

function serverSaved(): void {
  serverModal.value = null
  void refresh()
}

function keyAddServer(key: SSHKey): void {
  keyModal.value = false
  section.value = 'servers'
  serverModal.value = { host: null, presetKey: key }
}

function addPrimary(): void {
  if (section.value === 'servers') serverModal.value = { host: null, presetKey: null }
  else keyModal.value = true
}
</script>

<template>
  <div class="ssh-pane">
    <div class="ssh-frame">
      <nav class="ssh-nav">
        <AppNavItem
          size="sm"
          label="Servers"
          :active="section === 'servers'"
          :count="allHosts.length"
          @click="section = 'servers'"
        >
          <template #icon>
            <svg
              viewBox="0 0 14 14"
              fill="none"
              stroke="currentColor"
              stroke-width="1.2"
              stroke-linecap="round"
              stroke-linejoin="round"
            >
              <component
                :is="shape.tag"
                v-for="(shape, i) in sshIcon"
                :key="i"
                v-bind="shape.attrs"
              />
            </svg>
          </template>
        </AppNavItem>
        <AppNavItem
          size="sm"
          label="Keys"
          :active="section === 'keys'"
          :count="allKeys.length"
          @click="section = 'keys'"
        >
          <template #icon>
            <svg
              viewBox="0 0 14 14"
              fill="none"
              stroke="currentColor"
              stroke-width="1.2"
              stroke-linecap="round"
              stroke-linejoin="round"
            >
              <component
                :is="shape.tag"
                v-for="(shape, i) in keyIcon"
                :key="i"
                v-bind="shape.attrs"
              />
            </svg>
          </template>
        </AppNavItem>
      </nav>

      <main class="ssh-main">
        <div class="ssh-content">
          <div class="ssh-toolbar">
            <p class="ssh-toolbar-hint">
              <template v-if="section === 'servers'">
                The servers you log in to. They live in your ssh config, so they work in any
                terminal too.
              </template>
              <template v-else>
                The keys that log you in to servers, from
                {{ hosts ? tildePath(sshDir, home) : 'your ssh folder' }}.
              </template>
            </p>
            <AppButton variant="primary" size="sm" @click="addPrimary">
              {{ section === 'servers' ? '+ Add server' : '+ Create key' }}
            </AppButton>
          </div>

          <AppBanner v-if="unlinked && section === 'servers'">
            Servers added in lemongrass work in ssh only once your ssh config reads their file.
            <template #actions>
              <AppButton size="sm" @click="linkModal = true">Link ssh config</AppButton>
            </template>
          </AppBanner>

          <p v-if="error" class="settings-error">{{ error }}</p>

          <AppEmptyState
            v-if="!loaded && !error"
            layout="fill"
            loading
            title="Reading your ssh setup."
          />

          <template v-else-if="loaded && section === 'servers'">
            <AppEmptyState
              v-if="!allHosts.length"
              layout="fill"
              :icon="sshIcon"
              title="No servers yet."
              detail="Add the servers you log in to, and pick or create the key each one uses."
            >
              <template #action>
                <AppButton variant="primary" @click="addPrimary">Add server</AppButton>
              </template>
            </AppEmptyState>
            <template v-else>
              <AppInput
                v-model="query"
                size="sm"
                icon="search"
                placeholder="Filter by name or address"
                aria-label="Filter servers"
              />
              <div class="ssh-list">
                <SSHServerCard
                  v-for="host in serverRows"
                  :key="host.alias"
                  :host="host"
                  :keys="allKeys"
                  :ssh-dir="sshDir"
                  :home="home"
                  @open="openSSHServer(props.project, host.alias)"
                  @edit="serverModal = { host, presetKey: null }"
                  @changed="refresh"
                />
                <AppEmptyState v-if="!serverRows.length" title="No server matches." />
              </div>
            </template>
          </template>

          <template v-else-if="loaded">
            <AppEmptyState
              v-if="!allKeys.length"
              layout="fill"
              :icon="keyIcon"
              title="No keys yet."
              detail="A key logs you in to servers without typing a password each time."
            >
              <template #action>
                <AppButton variant="primary" @click="addPrimary">Create key</AppButton>
              </template>
            </AppEmptyState>
            <template v-else>
              <AppInput
                v-model="query"
                size="sm"
                icon="search"
                placeholder="Filter by name or label"
                aria-label="Filter keys"
              />
              <div class="ssh-list">
                <SSHKeyCard
                  v-for="key in keyRows"
                  :key="key.publicPath"
                  :ssh-key="key"
                  :hosts="allHosts"
                  :agent="!!keys?.agent"
                  @load="loadModal = $event"
                />
                <AppEmptyState v-if="!keyRows.length" title="No key matches." />
              </div>
            </template>
          </template>
        </div>
      </main>
    </div>

    <SSHKeyModal
      v-if="keyModal"
      :existing="allKeys.map((k) => k.name)"
      @close="keyModal = false"
      @created="refresh"
      @add-server="keyAddServer"
    />
    <SSHServerModal
      v-if="serverModal && hosts"
      :host="serverModal.host"
      :hosts="hosts"
      :keys="allKeys"
      :preset-key="serverModal.presetKey"
      @close="serverModal = null"
      @key-created="refresh"
      @saved="serverSaved"
    />
    <SSHLinkModal
      v-if="linkModal && hosts"
      :hosts="hosts"
      @close="linkModal = false"
      @linked="((linkModal = false), refresh())"
    />
    <SSHRunModal
      v-if="loadModal"
      :ssh-key="loadModal"
      @close="loadModal = null"
      @finished="refresh"
    />
  </div>
</template>
