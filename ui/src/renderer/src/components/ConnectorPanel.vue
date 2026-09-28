<script setup lang="ts">
import { computed, onMounted, ref, toRef } from 'vue'
import '../connector.css'
import {
  clearShortIds,
  connectorSession,
  describeError,
  lockConnector,
  noteConnectorActivity,
  unlockConnector
} from '../connector'
import ConnectionsSection from './connector/ConnectionsSection.vue'
import DbChannelsSection from './connector/DbChannelsSection.vue'
import DomainsSection from './connector/DomainsSection.vue'
import HttpChannelsSection from './connector/HttpChannelsSection.vue'

const props = defineProps<{
  active: boolean
}>()

const activeKind = ref<'database' | 'http'>('database')
const activeDbTab = ref<'connections' | 'channels'>('connections')
const activeHttpTab = ref<'domains' | 'httpChannels'>('domains')

const vaultUnlocked = toRef(connectorSession, 'unlocked')
const unlockInput = ref('')
const unlockError = ref('')
const unlocking = ref(false)

const hasPassphrase = ref(false)
const hasPassphraseLoading = ref(true)
const unlockMode = computed(() => (hasPassphrase.value ? 'enter' : 'set'))

async function refreshHasPassphrase(): Promise<void> {
  hasPassphraseLoading.value = true
  try {
    hasPassphrase.value = await window.api.vault.hasPassphrase()
  } catch (err) {
    unlockError.value = describeError(err)
  } finally {
    hasPassphraseLoading.value = false
  }
}

onMounted(refreshHasPassphrase)

async function unlockVault(): Promise<void> {
  if (!unlockInput.value || unlocking.value) return
  unlocking.value = true
  unlockError.value = ''
  const passphrase = unlockInput.value
  try {
    if (unlockMode.value === 'set') {
      await window.api.vault.setPassphrase(passphrase)
      hasPassphrase.value = true
    } else {
      await window.api.vault.verifyPassphrase(passphrase)
    }
    unlockInput.value = ''
    unlockConnector(passphrase)
  } catch (err) {
    unlockError.value = describeError(err)
  } finally {
    unlocking.value = false
  }
}

const showResetConfirm = ref(false)
const resetting = ref(false)
const resetError = ref('')

function openResetConfirm(): void {
  showResetConfirm.value = true
  resetError.value = ''
}

function cancelResetConfirm(): void {
  showResetConfirm.value = false
}

async function confirmReset(): Promise<void> {
  if (resetting.value) return
  resetting.value = true
  resetError.value = ''
  try {
    await window.api.vault.resetVault()
    showResetConfirm.value = false
    unlockError.value = ''
    unlockInput.value = ''
    hasPassphrase.value = false
    clearShortIds()
  } catch (err) {
    resetError.value = describeError(err)
  } finally {
    resetting.value = false
  }
}

function shown(kind: 'database' | 'http', tab: string): boolean {
  if (!props.active || activeKind.value !== kind) return false
  return (kind === 'database' ? activeDbTab.value : activeHttpTab.value) === tab
}
</script>

<template>
  <div
    class="connector-tab"
    @click.capture="noteConnectorActivity"
    @keydown.capture="noteConnectorActivity"
  >
    <div class="connector-frame" :class="{ 'connector-frame--gate': !vaultUnlocked }">
      <div v-if="!vaultUnlocked" class="connector-gate lg-scroll">
        <div v-if="!hasPassphraseLoading" class="unlock-panel">
          <template v-if="!showResetConfirm">
            <h4 class="inline-section-title">
              {{
                unlockMode === 'set' ? 'Set your vault passphrase' : 'Enter your vault passphrase'
              }}
            </h4>
            <p class="hint">
              <template v-if="unlockMode === 'set'">
                This becomes your vault's passphrase. Use the same one every time.
              </template>
              <template v-else>
                Unlocks your connections and channel actions for this session. Auto-locks after 5
                minutes idle.
              </template>
            </p>
            <div class="unlock-row">
              <input
                v-model="unlockInput"
                type="password"
                placeholder="Vault passphrase"
                autofocus
                @keydown.enter="unlockVault"
              />
              <button
                class="primary-button"
                :disabled="!unlockInput || unlocking"
                @click="unlockVault"
              >
                {{ unlocking ? 'Checking...' : 'Unlock' }}
              </button>
            </div>
            <p v-if="unlockError" class="error-text">{{ unlockError }}</p>
            <button
              v-if="unlockMode === 'enter'"
              class="ghost-button reset-link"
              @click="openResetConfirm"
            >
              Forgot your passphrase?
            </button>
          </template>

          <template v-else>
            <h4 class="inline-section-title">Reset the vault</h4>
            <p class="hint">
              This deletes every saved connection and channel permanently. This can't be undone.
            </p>
            <p v-if="resetError" class="error-text">{{ resetError }}</p>
            <div class="unlock-row">
              <button class="ghost-button" @click="cancelResetConfirm">Cancel</button>
              <button class="ghost-button danger" :disabled="resetting" @click="confirmReset">
                {{ resetting ? 'Resetting...' : 'Reset vault' }}
              </button>
            </div>
          </template>
        </div>
      </div>

      <template v-else>
        <nav class="connector-nav">
          <button
            class="nav-item"
            :class="{ active: activeKind === 'database' }"
            @click="activeKind = 'database'"
          >
            Database
          </button>
          <button
            class="nav-item"
            :class="{ active: activeKind === 'http' }"
            @click="activeKind = 'http'"
          >
            HTTP
          </button>
          <div class="nav-lock">
            <span class="unlock-status">Vault unlocked</span>
            <button class="ghost-button" @click="lockConnector">Lock</button>
          </div>
        </nav>

        <div class="connector-main lg-scroll">
          <div class="connector-content">
            <div v-show="activeKind === 'database'" class="kind-pane">
              <div class="tab-row">
                <button
                  class="tab"
                  :class="{ active: activeDbTab === 'connections' }"
                  @click="activeDbTab = 'connections'"
                >
                  Connections
                </button>
                <button
                  class="tab"
                  :class="{ active: activeDbTab === 'channels' }"
                  @click="activeDbTab = 'channels'"
                >
                  Channels
                </button>
              </div>
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
              <div class="tab-row">
                <button
                  class="tab"
                  :class="{ active: activeHttpTab === 'domains' }"
                  @click="activeHttpTab = 'domains'"
                >
                  Domains
                </button>
                <button
                  class="tab"
                  :class="{ active: activeHttpTab === 'httpChannels' }"
                  @click="activeHttpTab = 'httpChannels'"
                >
                  HTTP Channels
                </button>
              </div>
              <DomainsSection
                v-show="activeHttpTab === 'domains'"
                :visible="shown('http', 'domains')"
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
