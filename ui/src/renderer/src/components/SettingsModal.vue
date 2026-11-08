<!-- SPDX-License-Identifier: MPL-2.0 -->
<!-- Copyright (C) 2026 The Lemongrass Authors -->

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import {
  AppButton,
  AppCheckbox,
  AppConfirmButton,
  AppEmptyState,
  AppInput,
  AppModal,
  AppNavGroup,
  AppNavItem
} from '@sankasten'
import {
  PRIMARY_ACCOUNT_DIR,
  accountNameError,
  accountStore,
  addAccount,
  removeAccount
} from '../accounts'
import PolicySection from './safety/PolicySection.vue'
import UpdatesSection from './updates/UpdatesSection.vue'
import DepsSection from './deps/DepsSection.vue'
import CreditsSection from './credits/CreditsSection.vue'
import GeneralSection from './general/GeneralSection.vue'
import AppearanceSection from './appearance/AppearanceSection.vue'
import UsageMeters from './UsageMeters.vue'
import { claudeUsage, refreshUsage, usageStore } from '../usage'
import type { Project, StatusLineChainState } from '../../../preload/types'
import SettingsIcon from './SettingsIcon.vue'
import { updatePending } from '../updateState'

const props = defineProps<{
  projects: Project[]
  activeProjectId: string | null
  // The section shown on open, such as Updates when the notification center opens Settings.
  initialSection?: 'updates' | 'dependencies'
}>()

const emit = defineEmits<{
  close: []
}>()

type Section =
  | 'general'
  | 'appearance'
  | 'claude'
  | 'codex'
  | 'agy'
  | 'safety'
  | 'updates'
  | 'dependencies'
  | 'credits'

const section = ref<Section>(props.initialSection ?? 'general')

// lgrassconf applies the switch a moment after lgrassd writes it, so the saved commands are read again after that.
const KEEPER_SETTLE_MS = 1500

const chain = ref<StatusLineChainState | null>(null)
const chainBusy = ref(false)

async function loadChain(): Promise<void> {
  chain.value = await window.api.usage.statusLine()
}

async function toggleChain(): Promise<void> {
  if (!chain.value || chainBusy.value) return
  chainBusy.value = true
  chain.value = (await window.api.usage.setStatusLine(!chain.value.enabled)) ?? chain.value
  setTimeout(() => {
    void loadChain().finally(() => (chainBusy.value = false))
  }, KEEPER_SETTLE_MS)
}

const chainAccounts = computed(() => ['primary', ...accountStore.items.map((a) => a.name)])
const hasOwnCommand = computed(() =>
  chainAccounts.value.some((name) => chain.value?.commands[name])
)

watch(section, (value) => {
  if (value !== 'claude' && value !== 'codex' && value !== 'agy') return
  void refreshUsage()
  if (value === 'claude') void loadChain()
})

const newName = ref('')
const addError = ref('')
const adding = ref(false)
const removeError = ref('')

const nameError = computed(() => accountNameError(newName.value))
const canAdd = computed(() => newName.value.length > 0 && !nameError.value && !adding.value)

async function submitAdd(): Promise<void> {
  if (!canAdd.value) return
  adding.value = true
  addError.value = ''
  const error = await addAccount(newName.value)
  adding.value = false
  if (error) addError.value = error
  else newName.value = ''
}

async function onRemove(name: string): Promise<void> {
  removeError.value = await removeAccount(name)
}
</script>

<template>
  <AppModal scope="app" size="lg" flush label="Settings" @close="emit('close')">
    <div class="settings-layout">
      <aside class="settings-nav">
        <h3 class="settings-nav-title">Settings</h3>
        <AppNavItem :active="section === 'general'" label="General" @click="section = 'general'">
          <template #icon><SettingsIcon name="general" /></template>
        </AppNavItem>
        <AppNavItem
          :active="section === 'appearance'"
          label="Appearance"
          @click="section = 'appearance'"
        >
          <template #icon><SettingsIcon name="appearance" /></template>
        </AppNavItem>
        <AppNavGroup size="md">
          <template #parent>
            <AppNavItem label="Accounts" @click="section = 'claude'">
              <template #icon><SettingsIcon name="accounts" /></template>
            </AppNavItem>
          </template>
          <AppNavItem :active="section === 'claude'" label="Claude" @click="section = 'claude'">
            <template #icon><SettingsIcon name="claude" /></template>
          </AppNavItem>
          <AppNavItem :active="section === 'codex'" label="Codex" @click="section = 'codex'">
            <template #icon><SettingsIcon name="codex" /></template>
          </AppNavItem>
          <AppNavItem :active="section === 'agy'" label="Antigravity" @click="section = 'agy'">
            <template #icon><SettingsIcon name="agy" /></template>
          </AppNavItem>
        </AppNavGroup>
        <AppNavItem :active="section === 'safety'" label="Safety" @click="section = 'safety'">
          <template #icon><SettingsIcon name="safety" /></template>
        </AppNavItem>
        <AppNavItem
          :active="section === 'updates'"
          label="Updates"
          :dot="updatePending ? 'An update is available' : undefined"
          @click="section = 'updates'"
        >
          <template #icon><SettingsIcon name="updates" /></template>
        </AppNavItem>
        <AppNavItem
          :active="section === 'dependencies'"
          label="Dependencies"
          @click="section = 'dependencies'"
        >
          <template #icon><SettingsIcon name="dependencies" /></template>
        </AppNavItem>
        <AppNavItem :active="section === 'credits'" label="Credits" @click="section = 'credits'">
          <template #icon><SettingsIcon name="credits" /></template>
        </AppNavItem>
      </aside>

      <section v-if="section === 'general'" class="settings-section">
        <h4 class="settings-title">General</h4>
        <GeneralSection />
      </section>

      <section v-else-if="section === 'appearance'" class="settings-section">
        <h4 class="settings-title">Appearance</h4>
        <AppearanceSection />
      </section>

      <section v-else-if="section === 'safety'" class="settings-section settings-section--fixed">
        <h4 class="settings-title">Safety</h4>
        <PolicySection :projects="props.projects" :active-project-id="props.activeProjectId" />
      </section>

      <section v-else-if="section === 'updates'" class="settings-section">
        <h4 class="settings-title">Updates</h4>
        <UpdatesSection />
      </section>

      <section v-else-if="section === 'dependencies'" class="settings-section">
        <h4 class="settings-title">Dependencies</h4>
        <DepsSection />
      </section>

      <section v-else-if="section === 'credits'" class="settings-section">
        <h4 class="settings-title">Credits</h4>
        <CreditsSection />
      </section>

      <section v-else-if="section === 'codex'" class="settings-section">
        <h4 class="settings-title">Codex</h4>
        <p class="settings-text">
          Codex runs under one login. Several accounts are for Claude only for now.
        </p>
        <ul class="settings-rows">
          <li class="settings-row">
            <span class="settings-row-title">codex</span>
            <span class="settings-row-meta">~/.codex</span>
            <UsageMeters
              :windows="usageStore.codex?.windows"
              :seen-at="usageStore.codex?.seen_at"
            />
          </li>
        </ul>
        <p class="settings-text">
          Usage comes from what Codex writes while it runs. When Codex is idle, its last known
          figures are shown.
        </p>
      </section>

      <section v-else-if="section === 'agy'" class="settings-section">
        <h4 class="settings-title">Antigravity</h4>
        <p class="settings-text">
          Antigravity runs under one login. Several accounts are for Claude only for now.
        </p>
        <ul class="settings-rows">
          <li class="settings-row">
            <span class="settings-row-title">Antigravity</span>
            <span class="settings-row-meta">~/.gemini/antigravity-cli</span>
            <UsageMeters :windows="usageStore.agy?.windows" :seen-at="usageStore.agy?.seen_at" />
          </li>
        </ul>
        <p class="settings-text">
          Weekly Gemini and third-party quota comes from Antigravity's statusline. When Antigravity
          is idle, its last known figures are shown. Your own statusline command still runs.
        </p>
      </section>

      <section v-else class="settings-section">
        <h4 class="settings-title">Claude</h4>
        <p class="settings-text">
          Each account has its own login. A new Claude tab asks which account it should run under.
          An idle account shows its last known usage.
        </p>

        <ul class="settings-rows">
          <li class="settings-row">
            <span class="settings-row-title">primary</span>
            <span class="settings-row-meta">{{ PRIMARY_ACCOUNT_DIR }}</span>
            <UsageMeters
              :windows="claudeUsage(undefined)?.windows"
              :seen-at="claudeUsage(undefined)?.seen_at"
            />
            <span class="settings-badge">Primary</span>
          </li>
          <li v-for="account in accountStore.items" :key="account.name" class="settings-row">
            <span class="settings-row-title">{{ account.name }}</span>
            <span class="settings-row-meta">{{ account.dir }}</span>
            <UsageMeters
              :windows="claudeUsage(account.name)?.windows"
              :seen-at="claudeUsage(account.name)?.seen_at"
            />
            <AppConfirmButton
              v-slot="{ armed }"
              variant="ghost"
              size="xs"
              @confirm="onRemove(account.name)"
            >
              {{ armed ? 'Click again' : 'Remove' }}
            </AppConfirmButton>
          </li>
        </ul>
        <p v-if="removeError" class="settings-error">{{ removeError }}</p>

        <form class="settings-add" @submit.prevent="submitAdd">
          <AppInput
            v-model="newName"
            class="settings-input"
            type="text"
            placeholder="Account name"
            maxlength="32"
            spellcheck="false"
            autocomplete="off"
            aria-label="New account name"
          />
          <AppButton type="submit" variant="primary" :disabled="!canAdd">Add account</AppButton>
        </form>
        <p v-if="nameError || addError" class="settings-error">{{ nameError || addError }}</p>

        <div v-if="chain" class="settings-group statusline">
          <div class="statusline-head">
            <span class="statusline-title">Statusline</span>
            <AppCheckbox
              :checked="chain.enabled"
              :disabled="chainBusy"
              title="Controls Claude and Antigravity quota and context recording. Your own statusline commands keep running."
              @click="toggleChain"
            >
              Record usage
            </AppCheckbox>
          </div>
          <p class="settings-text statusline-text">
            Lemongrass runs your own statusline commands and reads the quota and context Claude Code
            and Antigravity pass to them. Codex usage is independent of this switch. Antigravity
            tabs opened while recording is off need reopening to start recording.
          </p>
          <div class="statusline-body">
            <AppEmptyState
              v-if="!chain.enabled"
              size="sm"
              title="Off."
              detail="Your own statusline commands keep running. Claude and Antigravity quota and context are not recorded."
            />
            <dl v-else-if="hasOwnCommand" class="settings-facts">
              <template v-for="name in chainAccounts" :key="name">
                <dt>{{ name }}</dt>
                <dd>
                  <code v-if="chain.commands[name]" class="statusline-command">{{
                    chain.commands[name]
                  }}</code>
                  <span v-else class="statusline-none">none</span>
                </dd>
              </template>
            </dl>
            <AppEmptyState
              v-else
              size="sm"
              title="None of your accounts has a statusline of its own."
            />
          </div>
        </div>
      </section>
    </div>
  </AppModal>
</template>

<style scoped>
.statusline {
  margin-top: var(--space-3);
}

.statusline-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: var(--space-3);
  padding: var(--space-2) var(--space-3) 0;
}

.statusline-title {
  color: var(--color-fg-primary);
  font-size: var(--text-sm);
  font-weight: var(--weight-medium);
}

.statusline-text {
  padding: 0 var(--space-3) var(--space-2);
}

.statusline-body {
  padding: var(--space-2) var(--space-3);
  border-top: 1px solid var(--color-border-default);
}

.statusline-command {
  display: block;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-family: var(--font-mono);
  font-size: var(--text-xs);
}

.statusline-none {
  margin: 0;
  color: var(--color-fg-muted);
  font-size: var(--text-xs);
}
</style>
