<script setup lang="ts">
import { onMounted, ref } from 'vue'
import type { VaultChannel } from '../../../preload'

const emit = defineEmits<{
  close: []
}>()

const activeTab = ref<'connections' | 'channels'>('connections')

// -- Connections --

const connections = ref<string[]>([])
const connectionsLoading = ref(false)
const connectionsError = ref('')

async function refreshConnections(): Promise<void> {
  connectionsLoading.value = true
  connectionsError.value = ''
  try {
    connections.value = await window.api.vault.listConnections()
  } catch (err) {
    connectionsError.value = err instanceof Error ? err.message : String(err)
  } finally {
    connectionsLoading.value = false
  }
}

const showConnectionForm = ref(false)
const newConnectionName = ref('')
const newConnectionString = ref('')
const newConnectionPassphrase = ref('')
const creatingConnection = ref(false)
const connectionFormError = ref('')

function openConnectionForm(): void {
  newConnectionName.value = ''
  newConnectionString.value = ''
  newConnectionPassphrase.value = ''
  connectionFormError.value = ''
  showConnectionForm.value = true
}

function cancelConnectionForm(): void {
  showConnectionForm.value = false
}

async function submitConnection(): Promise<void> {
  if (
    !newConnectionName.value.trim() ||
    !newConnectionString.value.trim() ||
    !newConnectionPassphrase.value
  )
    return
  if (creatingConnection.value) return
  creatingConnection.value = true
  connectionFormError.value = ''
  const passphrase = newConnectionPassphrase.value
  newConnectionPassphrase.value = '' // never held onto past the call it was typed for
  try {
    await window.api.vault.putCredential(
      passphrase,
      newConnectionName.value.trim(),
      newConnectionString.value
    )
    showConnectionForm.value = false
    await refreshConnections()
  } catch (err) {
    connectionFormError.value = err instanceof Error ? err.message : String(err)
  } finally {
    creatingConnection.value = false
  }
}

// -- Channels --

const channels = ref<VaultChannel[]>([])
const loading = ref(false)
const listError = ref('')
const lastIssuedShortId = ref('')

async function refresh(): Promise<void> {
  loading.value = true
  listError.value = ''
  try {
    channels.value = await window.api.vault.list()
  } catch (err) {
    listError.value = err instanceof Error ? err.message : String(err)
  } finally {
    loading.value = false
  }
}

onMounted(() => {
  refreshConnections()
  refresh()
})

function statusFor(channel: VaultChannel): string {
  const expires = new Date(channel.ExpiresAt)
  if (Number.isNaN(expires.getTime()) || expires.getTime() <= Date.now()) return 'expired'
  const hh = String(expires.getHours()).padStart(2, '0')
  const mm = String(expires.getMinutes()).padStart(2, '0')
  return `open until ${hh}:${mm}`
}

function scopeSummary(channel: VaultChannel): string {
  const tables = channel.Scope.Tables?.join(', ') || '(none)'
  const ops = channel.Scope.Operations?.join(', ') || '(none)'
  return `tables: ${tables} · operations: ${ops}`
}

// -- Create channel --

const showCreateForm = ref(false)
const newDbName = ref('')
const newTables = ref('')
const newOperations = ref('')
const newTtlMinutes = ref(60)
const newPassphrase = ref('')
const creating = ref(false)
const createError = ref('')

function openCreateForm(): void {
  newDbName.value = connections.value[0] ?? ''
  newTables.value = ''
  newOperations.value = ''
  newTtlMinutes.value = 60
  newPassphrase.value = ''
  createError.value = ''
  showCreateForm.value = true
}

function cancelCreate(): void {
  showCreateForm.value = false
}

function splitList(value: string): string[] {
  return value
    .split(',')
    .map((s) => s.trim())
    .filter(Boolean)
}

async function submitCreate(): Promise<void> {
  if (!newDbName.value.trim() || !newPassphrase.value || creating.value) return
  creating.value = true
  createError.value = ''
  const passphrase = newPassphrase.value
  newPassphrase.value = '' // never held onto past the call it was typed for
  try {
    const { shortId } = await window.api.vault.create(
      passphrase,
      newDbName.value.trim(),
      { Tables: splitList(newTables.value), Operations: splitList(newOperations.value) },
      Math.round(newTtlMinutes.value * 60)
    )
    lastIssuedShortId.value = shortId
    showCreateForm.value = false
    await refresh()
  } catch (err) {
    createError.value = err instanceof Error ? err.message : String(err)
  } finally {
    creating.value = false
  }
}

// -- Activate --

const activatingId = ref<string | null>(null)
const activateTtlMinutes = ref(60)
const activatePassphrase = ref('')
const activating = ref(false)
const activateError = ref('')

function openActivate(id: string): void {
  activatingId.value = id
  activateTtlMinutes.value = 60
  activatePassphrase.value = ''
  activateError.value = ''
}

function cancelActivate(): void {
  activatingId.value = null
}

async function submitActivate(): Promise<void> {
  if (!activatingId.value || !activatePassphrase.value || activating.value) return
  activating.value = true
  activateError.value = ''
  const passphrase = activatePassphrase.value
  const id = activatingId.value
  activatePassphrase.value = ''
  try {
    const { shortId } = await window.api.vault.activate(
      passphrase,
      id,
      Math.round(activateTtlMinutes.value * 60)
    )
    lastIssuedShortId.value = shortId
    activatingId.value = null
    await refresh()
  } catch (err) {
    activateError.value = err instanceof Error ? err.message : String(err)
  } finally {
    activating.value = false
  }
}

// -- Revoke --

const revokingId = ref<string | null>(null)

async function revoke(id: string): Promise<void> {
  revokingId.value = id
  try {
    await window.api.vault.revoke(id)
    await refresh()
  } catch (err) {
    listError.value = err instanceof Error ? err.message : String(err)
  } finally {
    revokingId.value = null
  }
}
</script>

<template>
  <div class="channels-overlay" @click.self="emit('close')">
    <div class="channels-card">
      <div class="channels-header">
        <h3 class="channels-title">Database access</h3>
        <button class="ghost-button" @click="emit('close')">Close</button>
      </div>

      <div class="tab-row">
        <button
          class="tab"
          :class="{ active: activeTab === 'connections' }"
          @click="activeTab = 'connections'"
        >
          Connections
        </button>
        <button
          class="tab"
          :class="{ active: activeTab === 'channels' }"
          @click="activeTab = 'channels'"
        >
          Channels
        </button>
      </div>

      <p v-if="lastIssuedShortId" class="short-id-notice">
        Short id for the model: <code>{{ lastIssuedShortId }}</code> -- relay this into chat, it's a
        reference only.
      </p>

      <template v-if="activeTab === 'connections'">
        <p v-if="connectionsError" class="error-text">{{ connectionsError }}</p>
        <p v-else-if="connectionsLoading" class="empty">Loading...</p>
        <p v-else-if="connections.length === 0" class="empty">No connections yet.</p>

        <div v-else class="channel-list">
          <div v-for="name in connections" :key="name" class="channel-row">
            <div class="channel-info">
              <span class="channel-db">{{ name }}</span>
            </div>
          </div>
        </div>

        <button class="primary-button new-channel-button" @click="openConnectionForm">
          + New connection
        </button>
      </template>

      <template v-else>
        <p v-if="listError" class="error-text">{{ listError }}</p>
        <p v-else-if="loading" class="empty">Loading...</p>
        <p v-else-if="channels.length === 0" class="empty">No channels yet.</p>

        <div v-else class="channel-list">
          <div v-for="channel in channels" :key="channel.ID" class="channel-row">
            <div class="channel-info">
              <span class="channel-db">{{ channel.DBName }}</span>
              <span class="channel-scope">{{ scopeSummary(channel) }}</span>
              <span class="channel-status">{{ statusFor(channel) }}</span>
            </div>
            <div class="channel-actions">
              <button class="ghost-button" @click="openActivate(channel.ID)">Activate</button>
              <button
                class="ghost-button danger"
                :disabled="revokingId === channel.ID"
                @click="revoke(channel.ID)"
              >
                {{ revokingId === channel.ID ? 'Revoking...' : 'Revoke' }}
              </button>
            </div>

            <div v-if="activatingId === channel.ID" class="inline-form">
              <input
                v-model="activatePassphrase"
                type="password"
                placeholder="Master passphrase"
                autofocus
                @keydown.enter="submitActivate"
              />
              <input
                v-model.number="activateTtlMinutes"
                type="number"
                min="1"
                title="TTL, minutes"
              />
              <button class="ghost-button" @click="cancelActivate">Cancel</button>
              <button
                class="primary-button"
                :disabled="!activatePassphrase || activating"
                @click="submitActivate"
              >
                {{ activating ? 'Activating...' : 'Activate' }}
              </button>
            </div>
            <p v-if="activatingId === channel.ID && activateError" class="error-text">
              {{ activateError }}
            </p>
          </div>
        </div>

        <button
          class="primary-button new-channel-button"
          :disabled="connections.length === 0"
          :title="connections.length === 0 ? 'Add a connection first' : ''"
          @click="openCreateForm"
        >
          + New channel
        </button>
      </template>
    </div>

    <div v-if="showConnectionForm" class="create-overlay">
      <div class="create-card">
        <h3 class="create-title">New connection</h3>
        <input v-model="newConnectionName" type="text" placeholder="Connection name" autofocus />
        <input
          v-model="newConnectionString"
          type="text"
          placeholder="mysql://user:pass@host:port/db"
        />
        <input
          v-model="newConnectionPassphrase"
          type="password"
          placeholder="Master passphrase"
          @keydown.enter="submitConnection"
        />
        <p v-if="connectionFormError" class="error-text">{{ connectionFormError }}</p>
        <div class="create-actions">
          <button class="ghost-button" @click="cancelConnectionForm">Cancel</button>
          <button
            class="primary-button"
            :disabled="
              !newConnectionName.trim() ||
              !newConnectionString.trim() ||
              !newConnectionPassphrase ||
              creatingConnection
            "
            @click="submitConnection"
          >
            {{ creatingConnection ? 'Saving...' : 'Save' }}
          </button>
        </div>
      </div>
    </div>

    <div v-if="showCreateForm" class="create-overlay">
      <div class="create-card">
        <h3 class="create-title">New channel</h3>
        <select v-model="newDbName">
          <option v-for="name in connections" :key="name" :value="name">{{ name }}</option>
        </select>
        <input v-model="newTables" type="text" placeholder="Tables (comma-separated)" />
        <input v-model="newOperations" type="text" placeholder="Operations (comma-separated)" />
        <input v-model.number="newTtlMinutes" type="number" min="1" title="TTL, minutes" />
        <input
          v-model="newPassphrase"
          type="password"
          placeholder="Master passphrase"
          @keydown.enter="submitCreate"
        />
        <p v-if="createError" class="error-text">{{ createError }}</p>
        <div class="create-actions">
          <button class="ghost-button" @click="cancelCreate">Cancel</button>
          <button
            class="primary-button"
            :disabled="!newDbName.trim() || !newPassphrase || creating"
            @click="submitCreate"
          >
            {{ creating ? 'Creating...' : 'Create' }}
          </button>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.channels-overlay {
  position: fixed;
  inset: 0;
  z-index: 20;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: var(--space-8);
  background: var(--color-bg-overlay);
}

.channels-card {
  width: 100%;
  max-width: 640px;
  max-height: 100%;
  overflow-y: auto;
  display: flex;
  flex-direction: column;
  gap: var(--space-3);
  padding: var(--space-6);
  background: var(--color-surface-1);
  border-radius: var(--radius-xl);
  box-shadow: var(--shadow-xl);
}

.channels-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.channels-title {
  font-family: var(--font-display);
  font-size: var(--text-md);
  font-weight: var(--weight-semibold);
  color: var(--color-fg-primary);
}

.tab-row {
  display: flex;
  gap: var(--space-1);
  border-bottom: 1px solid var(--color-border-default);
}

.tab {
  padding: var(--space-2) var(--space-3);
  background: transparent;
  border: none;
  color: var(--color-fg-secondary);
  font-family: var(--font-body);
  font-size: var(--text-sm);
  cursor: pointer;
  border-bottom: 2px solid transparent;
  transition:
    color var(--duration-fast) var(--ease-out),
    border-color var(--duration-fast) var(--ease-out);
}

.tab:hover {
  color: var(--color-fg-primary);
}

.tab.active {
  color: var(--color-fg-accent);
  border-bottom-color: var(--color-amber);
}

.short-id-notice {
  padding: var(--space-2) var(--space-3);
  background: var(--color-amber-muted);
  border-radius: var(--radius-md);
  color: var(--color-fg-accent);
  font-size: var(--text-xs);
}

.short-id-notice code {
  font-family: var(--font-mono);
}

.channel-list {
  display: flex;
  flex-direction: column;
  gap: var(--space-2);
}

.channel-row {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  justify-content: space-between;
  gap: var(--space-2);
  padding: var(--space-3);
  background: var(--color-surface-2);
  border-radius: var(--radius-md);
}

.channel-info {
  display: flex;
  flex-direction: column;
  gap: 2px;
  font-family: var(--font-body);
  font-size: var(--text-sm);
}

.channel-db {
  color: var(--color-fg-primary);
  font-weight: var(--weight-medium);
}

.channel-scope,
.channel-status {
  color: var(--color-fg-muted);
  font-size: var(--text-xs);
}

.channel-actions {
  display: flex;
  gap: var(--space-2);
}

.inline-form {
  display: flex;
  flex-wrap: wrap;
  gap: var(--space-2);
  width: 100%;
  margin-top: var(--space-2);
}

.inline-form input {
  flex: 1;
  min-width: 120px;
}

.new-channel-button {
  align-self: flex-start;
}

input,
select {
  padding: var(--space-2) var(--space-3);
  background: var(--color-surface-2);
  border: 1px solid var(--color-border-default);
  border-radius: var(--radius-md);
  color: var(--color-fg-primary);
  font-family: var(--font-body);
  font-size: var(--text-sm);
}

input:focus,
select:focus {
  outline: none;
  border-color: var(--color-amber);
}

.error-text {
  color: var(--color-danger, #e5484d);
  font-size: var(--text-xs);
}

.empty {
  color: var(--color-fg-muted);
  font-size: var(--text-sm);
  padding: var(--space-4);
}

.create-overlay {
  position: fixed;
  inset: 0;
  z-index: 21;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: var(--space-8);
  background: var(--color-bg-overlay);
}

.create-card {
  width: 100%;
  max-width: 420px;
  display: flex;
  flex-direction: column;
  gap: var(--space-3);
  padding: var(--space-6);
  background: var(--color-surface-1);
  border-radius: var(--radius-xl);
  box-shadow: var(--shadow-xl);
}

.create-title {
  font-family: var(--font-display);
  font-size: var(--text-md);
  font-weight: var(--weight-semibold);
  color: var(--color-fg-primary);
}

.create-actions {
  display: flex;
  justify-content: flex-end;
  gap: var(--space-2);
}

.ghost-button,
.primary-button {
  padding: var(--space-1) var(--space-4);
  border-radius: var(--radius-pill);
  font-family: var(--font-body);
  font-size: var(--text-xs);
  font-weight: var(--weight-medium);
  cursor: pointer;
  border: none;
  transition:
    background var(--duration-fast) var(--ease-out),
    color var(--duration-fast) var(--ease-out);
}

.ghost-button {
  background: transparent;
  border: 1px solid var(--color-border-default);
  color: var(--color-fg-secondary);
}

.ghost-button:hover {
  color: var(--color-fg-primary);
  background: var(--color-surface-2);
}

.ghost-button.danger {
  color: var(--color-danger, #e5484d);
  border-color: var(--color-danger, #e5484d);
}

.primary-button {
  background: var(--color-amber);
  color: var(--color-black);
}

.primary-button:hover:not(:disabled) {
  background: var(--color-amber-dim);
}

.ghost-button:disabled,
.primary-button:disabled {
  opacity: 0.5;
  cursor: default;
}
</style>
