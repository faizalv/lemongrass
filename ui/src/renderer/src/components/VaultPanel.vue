<!-- SPDX-License-Identifier: MPL-2.0 -->
<!-- Copyright (C) 2026 The Lemongrass Authors -->

<script setup lang="ts">
import { ref, toRef, watch } from 'vue'
import '../vault.css'
import { vaultSession, lockVaultSession, noteVaultActivity } from '../vaultSession'
import ConnectionsSection from './connector/ConnectionsSection.vue'
import DbChannelsSection from './connector/DbChannelsSection.vue'
import ResterList from './connector/rester/ResterList.vue'
import HttpChannelsSection from './connector/HttpChannelsSection.vue'
import {
  AppButton,
  AppIcon,
  AppIconButton,
  AppNavItem,
  AppSegmented,
  type IconShape
} from '@sankasten'
import VaultNavIcon from './VaultNavIcon.vue'
import VaultUnlockGate from './VaultUnlockGate.vue'
import DecisionsSection from './decisions/DecisionsSection.vue'
import { askState } from '../asks'
import { decisionsIcon } from '../icons'

const props = defineProps<{
  active: boolean
}>()

const activeKind = ref<'decisions' | 'database' | 'http'>('decisions')
const activeDbTab = ref<'connections' | 'channels'>('connections')
const activeHttpTab = ref<'resters' | 'httpChannels'>('resters')

const vaultUnlocked = toRef(vaultSession, 'unlocked')

// The sidebar folds to an icon rail, and the choice is kept across restarts.
const NAV_KEY = 'vault.navCollapsed'
const navCollapsed = ref(localStorage.getItem(NAV_KEY) === '1')
watch(navCollapsed, (collapsed) => localStorage.setItem(NAV_KEY, collapsed ? '1' : '0'))

const databaseIcon: IconShape[] = [
  { tag: 'path', attrs: { d: 'M2.5 3.5a4.5 1.8 0 1 0 9 0a4.5 1.8 0 1 0-9 0' } },
  { tag: 'path', attrs: { d: 'M2.5 3.5v7c0 1 2 1.8 4.5 1.8s4.5-.8 4.5-1.8v-7' } },
  { tag: 'path', attrs: { d: 'M2.5 7c0 1 2 1.8 4.5 1.8S11.5 8 11.5 7' } }
]

const resterIcon: IconShape[] = [
  { tag: 'circle', attrs: { cx: 7, cy: 7, r: 5.5 } },
  { tag: 'path', attrs: { d: 'M1.5 7h11' } },
  {
    tag: 'path',
    attrs: {
      d: 'M7 1.5c1.7 1.6 2.5 3.4 2.5 5.5S8.7 10.9 7 12.5C5.3 10.9 4.5 9.1 4.5 7S5.3 3.1 7 1.5z'
    }
  }
]

const entries = [
  { kind: 'decisions', label: 'Decisions', icon: decisionsIcon },
  { kind: 'database', label: 'Database', icon: databaseIcon },
  { kind: 'http', label: 'Rester', icon: resterIcon }
] as const

const dbTabs = [
  { value: 'connections', label: 'Connections' },
  { value: 'channels', label: 'Channels' }
] as const

const httpTabs = [
  { value: 'resters', label: 'Resters' },
  { value: 'httpChannels', label: 'Channels' }
] as const

function shown(kind: 'database' | 'http', tab: string): boolean {
  if (!props.active || activeKind.value !== kind) return false
  return (kind === 'database' ? activeDbTab.value : activeHttpTab.value) === tab
}
</script>

<template>
  <div class="vault-tab" @click.capture="noteVaultActivity" @keydown.capture="noteVaultActivity">
    <div class="vault-frame" :class="{ 'vault-frame--gate': !vaultUnlocked }">
      <VaultUnlockGate v-if="!vaultUnlocked" />

      <template v-else>
        <nav class="vault-nav" :class="{ collapsed: navCollapsed }">
          <div class="nav-top">
            <AppIconButton
              size="sm"
              :icon="navCollapsed ? 'chevronRight' : 'chevronLeft'"
              :label="navCollapsed ? 'Expand sidebar' : 'Collapse sidebar'"
              @click="navCollapsed = !navCollapsed"
            />
          </div>

          <template v-if="navCollapsed">
            <AppIconButton
              v-for="entry in entries"
              :key="entry.kind"
              :label="entry.label"
              :pressed="activeKind === entry.kind"
              :dot="entry.kind === 'decisions' && askState.count.waiting > 0 ? 'accent' : undefined"
              @click="activeKind = entry.kind"
            >
              <VaultNavIcon :shapes="entry.icon" />
            </AppIconButton>
          </template>

          <template v-else>
            <template v-for="entry in entries" :key="entry.kind">
              <span v-if="entry.kind === 'database'" class="nav-group-label">Connector</span>
              <AppNavItem
                size="sm"
                :label="entry.label"
                :active="activeKind === entry.kind"
                :count="entry.kind === 'decisions' ? askState.count.waiting : undefined"
                :count-title="entry.kind === 'decisions' ? 'Decisions waiting' : undefined"
                @click="activeKind = entry.kind"
              >
                <template #icon><VaultNavIcon :shapes="entry.icon" /></template>
              </AppNavItem>
            </template>
          </template>

          <div class="nav-lock">
            <AppIconButton
              v-if="navCollapsed"
              icon="lock"
              variant="ghost"
              tone="accent"
              size="sm"
              label="Lock vault"
              @click="lockVaultSession"
            />
            <AppButton v-else size="sm" tone="accent" @click="lockVaultSession">
              <AppIcon name="lock" :size="12" />
              Lock
            </AppButton>
          </div>
        </nav>

        <div class="vault-main">
          <div class="vault-content">
            <div v-show="activeKind === 'decisions'" class="kind-pane">
              <DecisionsSection :visible="active && activeKind === 'decisions'" />
            </div>

            <div v-show="activeKind === 'database'" class="kind-pane">
              <AppSegmented
                v-model="activeDbTab"
                size="sm"
                aria-label="Database"
                :options="[...dbTabs]"
              />
              <ConnectionsSection
                v-show="activeDbTab === 'connections'"
                :visible="shown('database', 'connections')"
              />
              <DbChannelsSection
                v-show="activeDbTab === 'channels'"
                :visible="shown('database', 'channels')"
              />
            </div>

            <div v-show="activeKind === 'http'" class="kind-pane">
              <AppSegmented
                v-model="activeHttpTab"
                size="sm"
                aria-label="Rester"
                :options="[...httpTabs]"
              />
              <ResterList
                v-show="activeHttpTab === 'resters'"
                :visible="shown('http', 'resters')"
              />
              <HttpChannelsSection
                v-show="activeHttpTab === 'httpChannels'"
                :visible="shown('http', 'httpChannels')"
              />
            </div>
          </div>
        </div>
      </template>
    </div>
  </div>
</template>
