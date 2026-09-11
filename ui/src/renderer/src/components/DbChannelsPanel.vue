<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
import type { VaultChannel } from '../../../preload'

const emit = defineEmits<{
  close: []
}>()

// Electron wraps every failed ipcRenderer.invoke as "Error invoking remote method '...':
// Error: <message>" -- strips that noise and maps known backend messages to plain copy, so
// what reaches the screen is never that wrapper text.
const knownErrors: Record<string, string> = {
  'vault: wrong passphrase': 'Wrong passphrase.'
}

function describeError(err: unknown): string {
  const raw = err instanceof Error ? err.message : String(err)
  const stripped = raw.replace(/^Error invoking remote method '[^']*': (Error: )?/, '')
  return knownErrors[stripped] ?? stripped
}

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
    connectionsError.value = describeError(err)
  } finally {
    connectionsLoading.value = false
  }
}

// -- Vault unlock: one passphrase for the whole vault (modal-wide, not per-tab), entered
// once per panel session and held in memory only (never persisted) -- every action below
// that needs it reuses this instead of asking again. Nothing else in the modal is usable
// until this is unlocked. Checked against a small fixed verifier (canary.go on the Go
// side), independent of any real connection, so "wrong passphrase" is known immediately
// and definitively rather than inferred from connections failing to verify.

const vaultUnlocked = ref(false)
const sessionPassphrase = ref('')
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

const AUTO_LOCK_MS = 5 * 60 * 1000
let idleTimer: ReturnType<typeof setTimeout> | null = null

function clearIdleTimer(): void {
  if (idleTimer) {
    clearTimeout(idleTimer)
    idleTimer = null
  }
}

function resetIdleTimer(): void {
  clearIdleTimer()
  if (!vaultUnlocked.value) return
  idleTimer = setTimeout(lockVault, AUTO_LOCK_MS)
}

// Any click or keypress inside the modal counts as activity, resetting the auto-lock clock.
function onActivity(): void {
  if (vaultUnlocked.value) resetIdleTimer()
}

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
    sessionPassphrase.value = passphrase
    vaultUnlocked.value = true
    resetIdleTimer()
    verifyAllConnections()
  } catch (err) {
    unlockError.value = describeError(err)
  } finally {
    unlocking.value = false
  }
}

function lockVault(): void {
  vaultUnlocked.value = false
  sessionPassphrase.value = ''
  rowStatus.value = {}
  rowStatusMessage.value = {}
  clearIdleTimer()
}

// -- Reset: the only way back from a forgotten passphrase. Wipes every stored connection
// and channel permanently, since nothing encrypted under a forgotten passphrase can be
// recovered without it.

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
    rowStatus.value = {}
    rowStatusMessage.value = {}
    await refreshConnections()
    await refresh()
  } catch (err) {
    resetError.value = describeError(err)
  } finally {
    resetting.value = false
  }
}

const rowStatus = ref<Record<string, 'verifying' | 'ok' | 'error'>>({})
const rowStatusMessage = ref<Record<string, string>>({})

async function verifyRow(name: string): Promise<void> {
  rowStatus.value[name] = 'verifying'
  try {
    await window.api.vault.testSavedConnection(sessionPassphrase.value, name)
    rowStatus.value[name] = 'ok'
    rowStatusMessage.value[name] = 'Connected'
  } catch (err) {
    rowStatus.value[name] = 'error'
    rowStatusMessage.value[name] = describeError(err)
  }
}

async function verifyAllConnections(): Promise<void> {
  if (!vaultUnlocked.value) return
  await Promise.all(connections.value.map(verifyRow))
}

async function testRow(name: string): Promise<void> {
  if (!vaultUnlocked.value) return
  await verifyRow(name)
}

function capsuleLabel(status?: 'verifying' | 'ok' | 'error'): string {
  switch (status) {
    case 'verifying':
      return 'Testing…'
    case 'ok':
      return 'Connected'
    case 'error':
      return 'Invalid'
    default:
      return 'Not tested'
  }
}

// -- New connection form: structured fields composed into a connection string, and a test
// that has to pass against the exact string that would be saved before Save is allowed.

type EngineChoice = 'mysql' | 'mariadb' | 'postgres'
type TestStatus = 'idle' | 'testing' | 'ok' | 'error'

const defaultPorts: Record<EngineChoice, string> = {
  mysql: '3306',
  mariadb: '3306',
  postgres: '5432'
}

const showConnectionForm = ref(false)
const newConnectionName = ref('')
const newConnectionEngine = ref<EngineChoice>('mysql')
const newConnectionHost = ref('')
const newConnectionPort = ref(defaultPorts.mysql)
const newConnectionUser = ref('')
const newConnectionPassword = ref('')
const newConnectionDatabase = ref('')
const connectionFormError = ref('')
const creatingConnection = ref(false)

const newConnectionTestStatus = ref<TestStatus>('idle')
const newConnectionTestError = ref('')

// Any field the composed string is built from invalidates a previous test -- the string
// that was tested and the string that gets saved must always be the same one.
watch(
  [
    newConnectionEngine,
    newConnectionHost,
    newConnectionPort,
    newConnectionUser,
    newConnectionPassword,
    newConnectionDatabase
  ],
  () => {
    newConnectionTestStatus.value = 'idle'
    newConnectionTestError.value = ''
  }
)

function composeConnectionString(): string {
  const user = newConnectionUser.value.trim()
  const password = newConnectionPassword.value
  const auth = user
    ? `${encodeURIComponent(user)}${password ? ':' + encodeURIComponent(password) : ''}@`
    : ''
  const port = newConnectionPort.value.trim()
  const host = newConnectionHost.value.trim()
  const hostPort = port ? `${host}:${port}` : host
  const database = newConnectionDatabase.value.trim()
  return `${newConnectionEngine.value}://${auth}${hostPort}${database ? '/' + database : ''}`
}

function openConnectionForm(): void {
  newConnectionName.value = ''
  newConnectionEngine.value = 'mysql'
  newConnectionHost.value = ''
  newConnectionPort.value = defaultPorts.mysql
  newConnectionUser.value = ''
  newConnectionPassword.value = ''
  newConnectionDatabase.value = ''
  connectionFormError.value = ''
  newConnectionTestStatus.value = 'idle'
  newConnectionTestError.value = ''
  showConnectionForm.value = true
}

function cancelConnectionForm(): void {
  showConnectionForm.value = false
}

// Only fills the port if it's empty or still at some engine's default -- never overwrites a
// port the user actually typed themselves.
function onEngineChange(): void {
  if (!newConnectionPort.value || Object.values(defaultPorts).includes(newConnectionPort.value)) {
    newConnectionPort.value = defaultPorts[newConnectionEngine.value]
  }
}

const canTestNewConnection = computed(
  () => newConnectionHost.value.trim().length > 0 && newConnectionTestStatus.value !== 'testing'
)

async function testNewConnection(): Promise<void> {
  if (!canTestNewConnection.value) return
  newConnectionTestStatus.value = 'testing'
  newConnectionTestError.value = ''
  try {
    await window.api.vault.testConnection(composeConnectionString())
    newConnectionTestStatus.value = 'ok'
  } catch (err) {
    newConnectionTestStatus.value = 'error'
    newConnectionTestError.value = describeError(err)
  }
}

async function submitConnection(): Promise<void> {
  if (
    !newConnectionName.value.trim() ||
    !vaultUnlocked.value ||
    newConnectionTestStatus.value !== 'ok' ||
    creatingConnection.value
  )
    return
  creatingConnection.value = true
  connectionFormError.value = ''
  try {
    await window.api.vault.putCredential(
      sessionPassphrase.value,
      newConnectionName.value.trim(),
      composeConnectionString()
    )
    showConnectionForm.value = false
    await refreshConnections()
    await verifyAllConnections()
  } catch (err) {
    connectionFormError.value = describeError(err)
  } finally {
    creatingConnection.value = false
  }
}

// -- Delete a saved connection -- no passphrase needed, DeleteConnection carries no root
// secret, so this works regardless of vault-unlock state.

const deletingConnectionName = ref<string | null>(null)

async function deleteConnection(name: string): Promise<void> {
  deletingConnectionName.value = name
  try {
    await window.api.vault.deleteConnection(name)
    delete rowStatus.value[name]
    delete rowStatusMessage.value[name]
    await refreshConnections()
  } catch (err) {
    connectionsError.value = describeError(err)
  } finally {
    deletingConnectionName.value = null
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
    listError.value = describeError(err)
  } finally {
    loading.value = false
  }
}

onMounted(() => {
  refreshHasPassphrase()
  refreshConnections()
  refresh()
})

onUnmounted(() => {
  clearIdleTimer()
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
const creating = ref(false)
const createError = ref('')

function openCreateForm(): void {
  newDbName.value = connections.value[0] ?? ''
  newTables.value = ''
  newOperations.value = ''
  newTtlMinutes.value = 60
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
  if (!newDbName.value.trim() || !vaultUnlocked.value || creating.value) return
  creating.value = true
  createError.value = ''
  try {
    const { shortId } = await window.api.vault.create(
      sessionPassphrase.value,
      newDbName.value.trim(),
      { Tables: splitList(newTables.value), Operations: splitList(newOperations.value) },
      Math.round(newTtlMinutes.value * 60)
    )
    lastIssuedShortId.value = shortId
    showCreateForm.value = false
    await refresh()
  } catch (err) {
    createError.value = describeError(err)
  } finally {
    creating.value = false
  }
}

// -- Activate --

const activatingId = ref<string | null>(null)
const activateTtlMinutes = ref(60)
const activating = ref(false)
const activateError = ref('')

function openActivate(id: string): void {
  activatingId.value = id
  activateTtlMinutes.value = 60
  activateError.value = ''
}

function cancelActivate(): void {
  activatingId.value = null
}

async function submitActivate(): Promise<void> {
  if (!activatingId.value || !vaultUnlocked.value || activating.value) return
  activating.value = true
  activateError.value = ''
  const id = activatingId.value
  try {
    const { shortId } = await window.api.vault.activate(
      sessionPassphrase.value,
      id,
      Math.round(activateTtlMinutes.value * 60)
    )
    lastIssuedShortId.value = shortId
    activatingId.value = null
    await refresh()
  } catch (err) {
    activateError.value = describeError(err)
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
    listError.value = describeError(err)
  } finally {
    revokingId.value = null
  }
}
</script>

<template>
  <div
    class="channels-overlay"
    @click.self="emit('close')"
    @click.capture="onActivity"
    @keydown.capture="onActivity"
  >
    <div class="channels-card">
      <div class="channels-header">
        <h3 class="channels-title">Database access</h3>
        <button class="ghost-button" @click="emit('close')">Close</button>
      </div>

      <div v-if="!hasPassphraseLoading" class="unlock-panel">
        <template v-if="!vaultUnlocked && !showResetConfirm">
          <h4 class="inline-section-title">
            {{ unlockMode === 'set' ? 'Set your vault passphrase' : 'Enter your vault passphrase' }}
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

        <template v-else-if="showResetConfirm">
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

        <div v-else class="unlock-row">
          <span class="unlock-status">Vault unlocked</span>
          <button class="ghost-button" @click="lockVault">Lock</button>
        </div>
      </div>

      <template v-if="vaultUnlocked">
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
          Short id for the model: <code>{{ lastIssuedShortId }}</code
          >. Relay this into chat as a reference.
        </p>

        <template v-if="activeTab === 'connections'">
          <p v-if="connectionsError" class="error-text">{{ connectionsError }}</p>
          <p v-else-if="connectionsLoading" class="empty">Loading...</p>
          <p v-else-if="connections.length === 0" class="empty">No connections yet.</p>

          <div v-else class="channel-list">
            <div v-for="name in connections" :key="name" class="channel-row">
              <div class="channel-info">
                <span class="channel-db">{{ name }}</span>
                <span
                  class="status-capsule"
                  :class="'status-' + (rowStatus[name] ?? 'idle')"
                  :title="rowStatusMessage[name] || ''"
                >
                  {{ capsuleLabel(rowStatus[name]) }}
                </span>
              </div>
              <div class="channel-actions">
                <button
                  class="ghost-button"
                  :disabled="!vaultUnlocked || rowStatus[name] === 'verifying'"
                  :title="!vaultUnlocked ? 'Unlock the vault first' : ''"
                  @click="testRow(name)"
                >
                  {{ rowStatus[name] === 'verifying' ? 'Testing...' : 'Test' }}
                </button>
                <button
                  class="ghost-button danger"
                  :disabled="deletingConnectionName === name"
                  @click="deleteConnection(name)"
                >
                  {{ deletingConnectionName === name ? 'Deleting...' : 'Delete' }}
                </button>
              </div>
            </div>
          </div>

          <button
            v-if="!showConnectionForm"
            class="primary-button new-channel-button"
            @click="openConnectionForm"
          >
            + New connection
          </button>

          <div v-else class="inline-section">
            <h4 class="inline-section-title">New connection</h4>

            <div>
              <label class="field-label" for="conn-name">Name</label>
              <input
                id="conn-name"
                v-model="newConnectionName"
                type="text"
                placeholder="e.g. staging-db"
                autofocus
              />
            </div>

            <div class="field-grid">
              <div>
                <label class="field-label" for="conn-engine">Engine</label>
                <select id="conn-engine" v-model="newConnectionEngine" @change="onEngineChange">
                  <option value="mysql">MySQL</option>
                  <option value="mariadb">MariaDB</option>
                  <option value="postgres">Postgres</option>
                </select>
              </div>
              <div>
                <label class="field-label" for="conn-host">Host</label>
                <input
                  id="conn-host"
                  v-model="newConnectionHost"
                  type="text"
                  placeholder="localhost"
                />
              </div>
              <div>
                <label class="field-label" for="conn-port">Port</label>
                <input id="conn-port" v-model="newConnectionPort" type="text" placeholder="3306" />
              </div>
              <div>
                <label class="field-label" for="conn-database">Database</label>
                <input
                  id="conn-database"
                  v-model="newConnectionDatabase"
                  type="text"
                  placeholder="db name"
                />
              </div>
              <div>
                <label class="field-label" for="conn-user">Username</label>
                <input
                  id="conn-user"
                  v-model="newConnectionUser"
                  type="text"
                  placeholder="db user"
                />
              </div>
              <div>
                <label class="field-label" for="conn-password">Password</label>
                <input
                  id="conn-password"
                  v-model="newConnectionPassword"
                  type="password"
                  placeholder="db password"
                />
              </div>
            </div>

            <div class="test-row">
              <button
                class="ghost-button"
                :disabled="!canTestNewConnection"
                @click="testNewConnection"
              >
                {{ newConnectionTestStatus === 'testing' ? 'Testing...' : 'Test connection' }}
              </button>
              <span
                class="test-status"
                :class="{
                  'test-status-ok': newConnectionTestStatus === 'ok',
                  'test-status-error': newConnectionTestStatus === 'error'
                }"
              >
                <template v-if="newConnectionTestStatus === 'ok'">✓ Connected</template>
                <template v-else-if="newConnectionTestStatus === 'error'">
                  ✗ {{ newConnectionTestError }}
                </template>
                <template v-else-if="newConnectionTestStatus === 'testing'">Testing...</template>
                <template v-else>Not tested yet</template>
              </span>
            </div>

            <p v-if="!vaultUnlocked" class="hint">Unlock the vault above before saving.</p>

            <p v-if="connectionFormError" class="error-text">{{ connectionFormError }}</p>

            <div class="create-actions">
              <button class="ghost-button" @click="cancelConnectionForm">Cancel</button>
              <button
                class="primary-button"
                :disabled="
                  !newConnectionName.trim() ||
                  !vaultUnlocked ||
                  newConnectionTestStatus !== 'ok' ||
                  creatingConnection
                "
                :title="
                  !vaultUnlocked
                    ? 'Unlock the vault first'
                    : newConnectionTestStatus !== 'ok'
                      ? 'Test the connection before saving'
                      : ''
                "
                @click="submitConnection"
              >
                {{ creatingConnection ? 'Saving...' : 'Save' }}
              </button>
            </div>
          </div>
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
                  v-model.number="activateTtlMinutes"
                  type="number"
                  min="1"
                  title="TTL, minutes"
                  autofocus
                  @keydown.enter="submitActivate"
                />
                <button class="ghost-button" @click="cancelActivate">Cancel</button>
                <button class="primary-button" :disabled="activating" @click="submitActivate">
                  {{ activating ? 'Activating...' : 'Activate' }}
                </button>
              </div>
              <p v-if="activatingId === channel.ID && activateError" class="error-text">
                {{ activateError }}
              </p>
            </div>
          </div>

          <button
            v-if="!showCreateForm"
            class="primary-button new-channel-button"
            :disabled="connections.length === 0"
            :title="connections.length === 0 ? 'Add a connection first' : ''"
            @click="openCreateForm"
          >
            + New channel
          </button>

          <div v-else class="inline-section">
            <h4 class="inline-section-title">New channel</h4>
            <select v-model="newDbName">
              <option v-for="name in connections" :key="name" :value="name">{{ name }}</option>
            </select>
            <input v-model="newTables" type="text" placeholder="Tables (comma-separated)" />
            <input v-model="newOperations" type="text" placeholder="Operations (comma-separated)" />
            <input
              v-model.number="newTtlMinutes"
              type="number"
              min="1"
              title="TTL, minutes"
              @keydown.enter="submitCreate"
            />
            <p v-if="createError" class="error-text">{{ createError }}</p>
            <div class="create-actions">
              <button class="ghost-button" @click="cancelCreate">Cancel</button>
              <button
                class="primary-button"
                :disabled="!newDbName.trim() || creating"
                @click="submitCreate"
              >
                {{ creating ? 'Creating...' : 'Create' }}
              </button>
            </div>
          </div>
        </template>
      </template>
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
  max-width: 760px;
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

.status-capsule {
  align-self: flex-start;
  padding: 1px var(--space-2);
  border-radius: var(--radius-pill);
  font-size: var(--text-xs);
  background: var(--color-surface-1);
  color: var(--color-fg-muted);
}

.status-ok {
  background: color-mix(in srgb, var(--color-success) 18%, transparent);
  color: var(--color-success);
}

.status-error {
  background: color-mix(in srgb, var(--color-danger, #e5484d) 18%, transparent);
  color: var(--color-danger, #e5484d);
}

.status-verifying {
  color: var(--color-fg-muted);
}

.channel-actions {
  display: flex;
  gap: var(--space-2);
}

.inline-form {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
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

.inline-section {
  display: flex;
  flex-direction: column;
  gap: var(--space-3);
  padding: var(--space-4);
  background: var(--color-surface-2);
  border: 1px solid var(--color-border-default);
  border-radius: var(--radius-lg);
}

.inline-section-title {
  font-family: var(--font-display);
  font-size: var(--text-sm);
  font-weight: var(--weight-semibold);
  color: var(--color-fg-primary);
}

.field-label {
  display: block;
  margin-bottom: var(--space-1);
  color: var(--color-fg-muted);
  font-size: var(--text-xs);
  font-weight: var(--weight-medium);
}

.field-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: var(--space-3);
}

.field-grid input,
.field-grid select {
  width: 100%;
}

.test-row {
  display: flex;
  align-items: center;
  gap: var(--space-3);
  flex-wrap: wrap;
}

.test-status {
  font-size: var(--text-xs);
  color: var(--color-fg-muted);
}

.test-status-ok {
  color: var(--color-success);
}

.test-status-error {
  color: var(--color-danger, #e5484d);
}

.unlock-panel {
  display: flex;
  flex-direction: column;
  gap: var(--space-2);
  padding: var(--space-3) var(--space-4);
  background: var(--color-surface-2);
  border: 1px solid var(--color-border-default);
  border-radius: var(--radius-lg);
}

.unlock-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: var(--space-2);
}

.unlock-row input {
  flex: 1;
}

.unlock-status {
  color: var(--color-success);
  font-size: var(--text-sm);
  font-weight: var(--weight-medium);
}

.reset-link {
  align-self: flex-start;
}

.hint {
  color: var(--color-fg-muted);
  font-size: var(--text-xs);
  line-height: 1.4;
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

select {
  /* The native chevron sits in a fixed inset that padding doesn't reliably move -- drop it instead. */
  appearance: none;
}

.inline-section input,
.inline-section select {
  background: var(--color-surface-1);
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
