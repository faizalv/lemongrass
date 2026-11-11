<!-- SPDX-License-Identifier: MPL-2.0 -->
<!-- Copyright (C) 2026 The Lemongrass Authors -->

<script setup lang="ts">
// The SSH view of the sidebar column: the servers you connect to and the keys you log in with. It
// reads the ssh setup each time the view is shown and runs nothing while another view is.
import { computed, onMounted, ref, watch } from 'vue'
import {
  AppBanner,
  AppButton,
  AppEmptyState,
  AppIcon,
  AppInput,
  AppMosaic,
  AppSegmented
} from '@sankasten'
import '../../ssh.css'
import { describeError } from '../../vaultSession'
import { keyIcon, sshIcon } from '../../icons'
import { matchesQuery } from '../connector/listFilter'
import { openSSHServer, type ProjectRef } from '../../workspace'
import SSHKeyCapsule from './SSHKeyCapsule.vue'
import SSHKeyModal from './SSHKeyModal.vue'
import SSHLinkModal from './SSHLinkModal.vue'
import SSHRunModal from './SSHRunModal.vue'
import SSHServerCapsule from './SSHServerCapsule.vue'
import SSHServerModal from './SSHServerModal.vue'
import { hostTarget, sshDirOf, type SSHSection } from './sshView'
import type { SSHHostEntry, SSHHostList, SSHKey, SSHKeyList } from '../../../../preload/types'

const props = defineProps<{
  project: ProjectRef
}>()

const SECTIONS: { value: SSHSection; label: string }[] = [
  { value: 'servers', label: 'Servers' },
  { value: 'keys', label: 'Keys' }
]

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

// The server and the key capsule clicked open, one of each at a time.
const expandedServer = ref<string | null>(null)
const expandedKey = ref<string | null>(null)
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

onMounted(refresh)

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
    <main class="ssh-main">
      <div class="ssh-content">
        <AppSegmented
          block
          aria-label="SSH section"
          :options="SECTIONS"
          :model-value="section"
          @update:model-value="section = $event"
        />
        <!-- The add action belongs to the section shown, so it sits under the switch and names
             what it adds. -->
        <AppButton variant="ghost" size="sm" class="ssh-add" @click="addPrimary">
          <AppIcon name="plus" />
          {{ section === 'servers' ? 'Add server' : 'Create key' }}
        </AppButton>

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
          />
          <template v-else>
            <AppInput
              v-model="query"
              size="sm"
              icon="search"
              placeholder="Filter by name or address"
              aria-label="Filter servers"
            />
            <AppMosaic v-model:expanded="expandedServer">
              <SSHServerCapsule
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
            </AppMosaic>
            <AppEmptyState v-if="!serverRows.length" title="No server matches." />
          </template>
        </template>

        <template v-else-if="loaded">
          <AppEmptyState
            v-if="!allKeys.length"
            layout="fill"
            :icon="keyIcon"
            title="No keys yet."
            detail="A key logs you in to servers without typing a password each time."
          />
          <template v-else>
            <AppInput
              v-model="query"
              size="sm"
              icon="search"
              placeholder="Filter by name or label"
              aria-label="Filter keys"
            />
            <AppMosaic v-model:expanded="expandedKey">
              <SSHKeyCapsule
                v-for="key in keyRows"
                :key="key.publicPath"
                :ssh-key="key"
                :hosts="allHosts"
                :agent="!!keys?.agent"
                :expanded="expandedKey === key.name"
                @load="loadModal = $event"
              />
            </AppMosaic>
            <AppEmptyState v-if="!keyRows.length" title="No key matches." />
          </template>
        </template>
      </div>
    </main>

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
