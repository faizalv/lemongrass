<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
import type {
  VaultChannel,
  VaultDomain,
  VaultDomainUser,
  VaultHTTPChannel,
  VaultHTTPScope
} from '../../../preload/types'

const props = defineProps<{
  visible: boolean
}>()

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

// One vault unlock at the top covers both kinds below, rather than a separate unlock session
// per kind -- Database and HTTP each keep their own pair of sub-tabs.
const activeKind = ref<'database' | 'http'>('database')
const activeDbTab = ref<'connections' | 'channels'>('connections')
const activeHttpTab = ref<'domains' | 'httpChannels'>('domains')

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
    await refreshDomains()
    await refreshHTTPChannels()
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
const shortIdByChannel = ref<Record<string, string>>({})

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
  refreshDomains()
  refreshHTTPChannels()
  document.addEventListener('click', onDocumentClickOutsideTablesDropdown, true)
})

// The panel now stays mounted behind v-show (so shortIdByChannel and the vault-unlock
// session survive closing it) instead of being destroyed on close -- reopening it doesn't
// remount, so the channel/connection lists need their own refresh to catch anything that
// changed while it was hidden (expiry, a row acted on elsewhere).
watch(
  () => props.visible,
  (visible) => {
    if (!visible) return
    refreshConnections()
    refresh()
    refreshDomains()
    refreshHTTPChannels()
  }
)

onUnmounted(() => {
  clearIdleTimer()
  if (copiedTimer) clearTimeout(copiedTimer)
  if (copiedHTTPTimer) clearTimeout(copiedHTTPTimer)
  document.removeEventListener('click', onDocumentClickOutsideTablesDropdown, true)
})

function statusFor(channel: VaultChannel): string {
  const expires = new Date(channel.ExpiresAt)
  if (Number.isNaN(expires.getTime()) || expires.getTime() <= Date.now()) return 'expired'
  const hh = String(expires.getHours()).padStart(2, '0')
  const mm = String(expires.getMinutes()).padStart(2, '0')
  return `open until ${hh}:${mm}`
}

const copiedChannelId = ref<string | null>(null)
let copiedTimer: ReturnType<typeof setTimeout> | null = null

async function copyShortId(id: string): Promise<void> {
  const value = shortIdByChannel.value[id]
  if (!value) return
  try {
    await navigator.clipboard.writeText(value)
  } catch {
    return
  }
  copiedChannelId.value = id
  if (copiedTimer) clearTimeout(copiedTimer)
  copiedTimer = setTimeout(() => {
    copiedChannelId.value = null
  }, 1500)
}

// -- Create channel --

// Fixed to the only kinds Scope.AllowStatement (vault/bouncer.go) actually recognizes -- write
// kinds are a separate, later PRD, not offered here.
const operationOptions = ['select', 'explain', 'show'] as const

const showCreateForm = ref(false)
const newChannelName = ref('')
const newDbName = ref('')
const newTables = ref<string[]>([])
const newOperations = ref<string[]>([])
const newTtlMinutes = ref(10)
const creating = ref(false)
const createError = ref('')

// -- Tables picker: populated from the chosen connection's own database, instead of free
// comma-text guessing at what's actually in there.

const availableTables = ref<string[]>([])
const tablesLoading = ref(false)
const tablesError = ref('')

async function loadTablesFor(dbName: string): Promise<void> {
  availableTables.value = []
  tablesError.value = ''
  if (!dbName || !vaultUnlocked.value) return
  tablesLoading.value = true
  try {
    availableTables.value = await window.api.vault.listTables(sessionPassphrase.value, dbName)
  } catch (err) {
    tablesError.value = describeError(err)
  } finally {
    tablesLoading.value = false
  }
}

watch(newDbName, (dbName) => {
  newTables.value = []
  loadTablesFor(dbName)
})

// -- Tables combobox: closed by default, showing a summary instead of every checkbox at
// once -- a flat list stops being usable somewhere past a couple dozen tables. Opens into a
// filterable list; selections show as removable chips regardless of whether it's open.

const tablesDropdownOpen = ref(false)
const tableFilter = ref('')
const tablesDropdownRef = ref<HTMLElement | null>(null)

const filteredTables = computed(() => {
  const query = tableFilter.value.trim().toLowerCase()
  if (!query) return availableTables.value
  return availableTables.value.filter((t) => t.toLowerCase().includes(query))
})

const tablesSummaryLabel = computed(() => {
  if (tablesLoading.value) return 'Loading tables...'
  if (newTables.value.length === 0) return 'Select tables'
  if (newTables.value.length === 1) return newTables.value[0]
  return `${newTables.value.length} tables selected`
})

function toggleTablesDropdown(): void {
  tablesDropdownOpen.value = !tablesDropdownOpen.value
  if (tablesDropdownOpen.value) tableFilter.value = ''
}

function removeSelectedTable(table: string): void {
  newTables.value = newTables.value.filter((t) => t !== table)
}

// Closes the dropdown on any click outside it, without disturbing clicks on the trigger
// itself (that's handled by its own @click toggle) or inside the popover (filtering,
// checking a table shouldn't close it).
function onDocumentClickOutsideTablesDropdown(event: MouseEvent): void {
  if (!tablesDropdownOpen.value) return
  if (tablesDropdownRef.value && !tablesDropdownRef.value.contains(event.target as Node)) {
    tablesDropdownOpen.value = false
  }
}

function openCreateForm(): void {
  newChannelName.value = ''
  newDbName.value = connections.value[0] ?? ''
  newTables.value = []
  newOperations.value = []
  newTtlMinutes.value = 10
  createError.value = ''
  showCreateForm.value = true
  loadTablesFor(newDbName.value)
}

function cancelCreate(): void {
  showCreateForm.value = false
}

async function submitCreate(): Promise<void> {
  if (
    !newChannelName.value.trim() ||
    !newDbName.value.trim() ||
    !vaultUnlocked.value ||
    creating.value
  )
    return
  creating.value = true
  createError.value = ''
  try {
    const { channel, shortId } = await window.api.vault.create(
      sessionPassphrase.value,
      newChannelName.value.trim(),
      newDbName.value.trim(),
      // Spread into plain arrays -- Electron's IPC clones arguments via structured clone,
      // which can't clone the Vue reactive Proxy the checkboxes' v-model leaves in .value.
      { Tables: [...newTables.value], Operations: [...newOperations.value] },
      Math.round(newTtlMinutes.value * 60)
    )
    shortIdByChannel.value[channel.ID] = shortId
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
const activateTtlMinutes = ref(10)
const activating = ref(false)
const activateError = ref('')

function openActivate(id: string): void {
  activatingId.value = id
  activateTtlMinutes.value = 10
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
    const { channel, shortId } = await window.api.vault.activate(
      sessionPassphrase.value,
      id,
      Math.round(activateTtlMinutes.value * 60)
    )
    shortIdByChannel.value[channel.ID] = shortId
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
    delete shortIdByChannel.value[id]
    await refresh()
  } catch (err) {
    listError.value = describeError(err)
  } finally {
    revokingId.value = null
  }
}

// -- Domains --

const domains = ref<string[]>([])
const domainsLoading = ref(false)
const domainsError = ref('')

async function refreshDomains(): Promise<void> {
  domainsLoading.value = true
  domainsError.value = ''
  try {
    domains.value = await window.api.vault.listDomains()
  } catch (err) {
    domainsError.value = describeError(err)
  } finally {
    domainsLoading.value = false
  }
}

const deletingDomainName = ref<string | null>(null)

async function deleteDomain(name: string): Promise<void> {
  deletingDomainName.value = name
  try {
    await window.api.vault.deleteDomain(name)
    await refreshDomains()
  } catch (err) {
    domainsError.value = describeError(err)
  } finally {
    deletingDomainName.value = null
  }
}

// -- New domain form: BaseURL/LoginEndpoint/TokenPath/TTLOrigin/TokenPlacement plus a
// growable Users list -- each user is either a login user (free-form field rows, since a
// domain's login shape varies per API) or a bring-your-own-token user (one pasted token, no
// login call). No saved-domain re-test (unlike a db connection, a domain has many users, not
// one pingable target) -- testing only happens per-user, inline, while building this form.

type TtlMode = 'field' | 'jwt' | 'fixed'
type UserTestStatus = 'idle' | 'testing' | 'ok' | 'error'

interface UserFieldDraft {
  key: string
  value: string
}

interface UserDraft {
  name: string
  isByot: boolean
  token: string
  tags: string
  fields: UserFieldDraft[]
  testStatus: UserTestStatus
  testError: string
}

const showDomainForm = ref(false)
const editingDomainName = ref<string | null>(null)
const loadingDomainName = ref<string | null>(null)
const newDomainName = ref('')
const newDomainBaseUrl = ref('')
const newDomainLoginEndpoint = ref('')
const newDomainTokenPath = ref('')
const newDomainTtlMode = ref<TtlMode>('fixed')
const newDomainTtlField = ref('')
const newDomainFixedTtlMinutes = ref(60)
const newDomainTokenKind = ref<'header' | 'cookie' | 'query'>('header')
const newDomainTokenName = ref('Authorization')
const newDomainTokenPrefix = ref('Bearer ')
const newDomainUsers = ref<UserDraft[]>([])
const domainFormError = ref('')
const creatingDomain = ref(false)

function openDomainForm(): void {
  editingDomainName.value = null
  newDomainName.value = ''
  newDomainBaseUrl.value = ''
  newDomainLoginEndpoint.value = ''
  newDomainTokenPath.value = ''
  newDomainTtlMode.value = 'fixed'
  newDomainTtlField.value = ''
  newDomainFixedTtlMinutes.value = 60
  newDomainTokenKind.value = 'header'
  newDomainTokenName.value = 'Authorization'
  newDomainTokenPrefix.value = 'Bearer '
  newDomainUsers.value = []
  domainFormError.value = ''
  showDomainForm.value = true
}

function cancelDomainForm(): void {
  showDomainForm.value = false
}

function userDraftFromDomainUser(user: VaultDomainUser): UserDraft {
  const fields = Object.entries(user.Fields ?? {}).map(([key, value]) => ({ key, value }))
  return {
    name: user.Name,
    isByot: user.Token !== '',
    token: user.Token,
    tags: (user.Tags ?? []).join(', '),
    fields: fields.length > 0 ? fields : [{ key: '', value: '' }],
    testStatus: 'idle',
    testError: ''
  }
}

async function openDomainEdit(name: string): Promise<void> {
  if (!vaultUnlocked.value || loadingDomainName.value) return
  loadingDomainName.value = name
  domainsError.value = ''
  try {
    const domain = await window.api.vault.getDomain(sessionPassphrase.value, name)
    editingDomainName.value = name
    newDomainName.value = name
    newDomainBaseUrl.value = domain.BaseURL
    newDomainLoginEndpoint.value = domain.LoginEndpoint
    newDomainTokenPath.value = domain.TokenPath
    if (domain.TTLOrigin === 'jwt-exp') {
      newDomainTtlMode.value = 'jwt'
      newDomainTtlField.value = ''
    } else if (domain.TTLOrigin !== '') {
      newDomainTtlMode.value = 'field'
      newDomainTtlField.value = domain.TTLOrigin
    } else {
      newDomainTtlMode.value = 'fixed'
      newDomainTtlField.value = ''
    }
    newDomainFixedTtlMinutes.value = domain.FixedTTLSeconds / 60
    newDomainTokenKind.value = domain.TokenPlacement.Kind
    newDomainTokenName.value = domain.TokenPlacement.Name
    newDomainTokenPrefix.value = domain.TokenPlacement.Prefix
    newDomainUsers.value = (domain.Users ?? []).map(userDraftFromDomainUser)
    domainFormError.value = ''
    showDomainForm.value = true
  } catch (err) {
    domainsError.value = describeError(err)
  } finally {
    loadingDomainName.value = null
  }
}

function addUserDraft(): void {
  newDomainUsers.value.push({
    name: '',
    isByot: false,
    token: '',
    tags: '',
    fields: [{ key: '', value: '' }],
    testStatus: 'idle',
    testError: ''
  })
}

function removeUserDraft(index: number): void {
  newDomainUsers.value.splice(index, 1)
}

function addUserField(userIndex: number): void {
  newDomainUsers.value[userIndex].fields.push({ key: '', value: '' })
}

function removeUserField(userIndex: number, fieldIndex: number): void {
  newDomainUsers.value[userIndex].fields.splice(fieldIndex, 1)
}

// Builds a plain (non-reactive) Domain object with an empty Users list -- Electron's IPC
// clones arguments via structured clone, which can't clone a Vue reactive Proxy, so every
// value read off a form ref is copied into a fresh plain object/array before crossing it.
function buildDomainDraft(): VaultDomain {
  const ttlOrigin =
    newDomainTtlMode.value === 'jwt'
      ? 'jwt-exp'
      : newDomainTtlMode.value === 'field'
        ? newDomainTtlField.value.trim()
        : ''
  return {
    BaseURL: newDomainBaseUrl.value.trim(),
    LoginEndpoint: newDomainLoginEndpoint.value.trim(),
    TokenPath: newDomainTokenPath.value.trim(),
    TTLOrigin: ttlOrigin,
    FixedTTLSeconds: Math.round(newDomainFixedTtlMinutes.value * 60),
    TokenPlacement: {
      Kind: newDomainTokenKind.value,
      Name: newDomainTokenName.value.trim(),
      Prefix: newDomainTokenPrefix.value
    },
    Users: []
  }
}

function buildUserPayload(draft: UserDraft): VaultDomainUser {
  const tags = draft.tags
    .split(',')
    .map((t) => t.trim())
    .filter((t) => t.length > 0)
  if (draft.isByot) {
    return { Name: draft.name.trim(), Fields: {}, Token: draft.token, Tags: tags }
  }
  const fields: Record<string, string> = {}
  for (const f of draft.fields) {
    if (f.key.trim()) fields[f.key.trim()] = f.value
  }
  return { Name: draft.name.trim(), Fields: fields, Token: '', Tags: tags }
}

async function testUserDraft(index: number): Promise<void> {
  const draft = newDomainUsers.value[index]
  draft.testStatus = 'testing'
  draft.testError = ''
  try {
    await window.api.vault.testDomainLogin(buildDomainDraft(), buildUserPayload(draft))
    draft.testStatus = 'ok'
  } catch (err) {
    draft.testStatus = 'error'
    draft.testError = describeError(err)
  }
}

const canSubmitDomain = computed(
  () =>
    newDomainName.value.trim().length > 0 &&
    newDomainBaseUrl.value.trim().length > 0 &&
    newDomainUsers.value.length > 0 &&
    newDomainUsers.value.every((u) => u.name.trim().length > 0)
)

async function submitDomain(): Promise<void> {
  if (!canSubmitDomain.value || !vaultUnlocked.value || creatingDomain.value) return
  creatingDomain.value = true
  domainFormError.value = ''
  try {
    const domain = buildDomainDraft()
    domain.Users = newDomainUsers.value.map(buildUserPayload)
    if (editingDomainName.value !== null) {
      await window.api.vault.updateDomain(sessionPassphrase.value, editingDomainName.value, domain)
    } else {
      await window.api.vault.putDomain(sessionPassphrase.value, newDomainName.value.trim(), domain)
    }
    showDomainForm.value = false
    await refreshDomains()
  } catch (err) {
    domainFormError.value = describeError(err)
  } finally {
    creatingDomain.value = false
  }
}

// -- HTTP Channels --

const httpChannels = ref<VaultHTTPChannel[]>([])
const httpChannelsLoading = ref(false)
const httpChannelsListError = ref('')
const shortIdByHTTPChannel = ref<Record<string, string>>({})

async function refreshHTTPChannels(): Promise<void> {
  httpChannelsLoading.value = true
  httpChannelsListError.value = ''
  try {
    httpChannels.value = await window.api.vault.listHTTPChannels()
  } catch (err) {
    httpChannelsListError.value = describeError(err)
  } finally {
    httpChannelsLoading.value = false
  }
}

// Fixed set -- there's no schema to read methods off the way db tables get read off a real
// database.
const methodOptions = ['GET', 'POST', 'PUT', 'PATCH', 'DELETE', 'HEAD', 'OPTIONS'] as const

interface ExclusionDraft {
  method: string
  pathPattern: string
}

const showHTTPCreateForm = ref(false)
const newHTTPChannelName = ref('')
const newHTTPChannelDomain = ref('')
const newHTTPMethods = ref<string[]>([])
const newHTTPExclusions = ref<ExclusionDraft[]>([])
const newHTTPTtlMinutes = ref(10)
const creatingHTTPChannel = ref(false)
const createHTTPError = ref('')

function openHTTPCreateForm(): void {
  newHTTPChannelName.value = ''
  newHTTPChannelDomain.value = domains.value[0] ?? ''
  newHTTPMethods.value = []
  newHTTPExclusions.value = []
  newHTTPTtlMinutes.value = 10
  createHTTPError.value = ''
  showHTTPCreateForm.value = true
}

function cancelHTTPCreate(): void {
  showHTTPCreateForm.value = false
}

function addExclusion(): void {
  newHTTPExclusions.value.push({ method: newHTTPMethods.value[0] ?? 'GET', pathPattern: '' })
}

function removeExclusion(index: number): void {
  newHTTPExclusions.value.splice(index, 1)
}

async function submitHTTPCreate(): Promise<void> {
  if (
    !newHTTPChannelName.value.trim() ||
    !newHTTPChannelDomain.value.trim() ||
    !vaultUnlocked.value ||
    creatingHTTPChannel.value
  )
    return
  creatingHTTPChannel.value = true
  createHTTPError.value = ''
  try {
    const scope: VaultHTTPScope = {
      Methods: [...newHTTPMethods.value],
      Exclusions: newHTTPExclusions.value
        .filter((e) => e.pathPattern.trim())
        .map((e) => ({ Method: e.method, PathPattern: e.pathPattern.trim() }))
    }
    const { channel, shortId } = await window.api.vault.createHTTPChannel(
      sessionPassphrase.value,
      newHTTPChannelName.value.trim(),
      newHTTPChannelDomain.value.trim(),
      scope,
      Math.round(newHTTPTtlMinutes.value * 60)
    )
    shortIdByHTTPChannel.value[channel.ID] = shortId
    showHTTPCreateForm.value = false
    await refreshHTTPChannels()
  } catch (err) {
    createHTTPError.value = describeError(err)
  } finally {
    creatingHTTPChannel.value = false
  }
}

// -- Activate / Rotate --

const activatingHTTPId = ref<string | null>(null)
const activateHTTPTtlMinutes = ref(10)
const activatingHTTP = ref(false)
const activateHTTPError = ref('')

function openHTTPActivate(id: string): void {
  activatingHTTPId.value = id
  activateHTTPTtlMinutes.value = 10
  activateHTTPError.value = ''
}

function cancelHTTPActivate(): void {
  activatingHTTPId.value = null
}

async function submitHTTPActivate(): Promise<void> {
  if (!activatingHTTPId.value || !vaultUnlocked.value || activatingHTTP.value) return
  activatingHTTP.value = true
  activateHTTPError.value = ''
  const id = activatingHTTPId.value
  try {
    const { channel, shortId } = await window.api.vault.activateHTTPChannel(
      sessionPassphrase.value,
      id,
      Math.round(activateHTTPTtlMinutes.value * 60)
    )
    shortIdByHTTPChannel.value[channel.ID] = shortId
    activatingHTTPId.value = null
    await refreshHTTPChannels()
  } catch (err) {
    activateHTTPError.value = describeError(err)
  } finally {
    activatingHTTP.value = false
  }
}

// -- Revoke --

const revokingHTTPId = ref<string | null>(null)

async function revokeHTTP(id: string): Promise<void> {
  revokingHTTPId.value = id
  try {
    await window.api.vault.revokeHTTPChannel(id)
    delete shortIdByHTTPChannel.value[id]
    await refreshHTTPChannels()
  } catch (err) {
    httpChannelsListError.value = describeError(err)
  } finally {
    revokingHTTPId.value = null
  }
}

function statusForHTTP(channel: VaultHTTPChannel): string {
  const expires = new Date(channel.ExpiresAt)
  if (Number.isNaN(expires.getTime()) || expires.getTime() <= Date.now()) return 'expired'
  const hh = String(expires.getHours()).padStart(2, '0')
  const mm = String(expires.getMinutes()).padStart(2, '0')
  return `open until ${hh}:${mm}`
}

const copiedHTTPChannelId = ref<string | null>(null)
let copiedHTTPTimer: ReturnType<typeof setTimeout> | null = null

async function copyHTTPShortId(id: string): Promise<void> {
  const value = shortIdByHTTPChannel.value[id]
  if (!value) return
  try {
    await navigator.clipboard.writeText(value)
  } catch {
    return
  }
  copiedHTTPChannelId.value = id
  if (copiedHTTPTimer) clearTimeout(copiedHTTPTimer)
  copiedHTTPTimer = setTimeout(() => {
    copiedHTTPChannelId.value = null
  }, 1500)
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
        <h3 class="channels-title">Connector</h3>
        <button class="close-button" title="Close" @click="emit('close')">&times;</button>
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
        <div class="tab-row kind-row">
          <button
            class="tab"
            :class="{ active: activeKind === 'database' }"
            @click="activeKind = 'database'"
          >
            Database
          </button>
          <button
            class="tab"
            :class="{ active: activeKind === 'http' }"
            @click="activeKind = 'http'"
          >
            HTTP
          </button>
        </div>

        <template v-if="activeKind === 'database'">
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

          <template v-if="activeDbTab === 'connections'">
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
                  <input
                    id="conn-port"
                    v-model="newConnectionPort"
                    type="text"
                    placeholder="3306"
                  />
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

          <template v-else-if="activeDbTab === 'channels'">
            <p v-if="listError" class="error-text">{{ listError }}</p>
            <p v-else-if="loading" class="empty">Loading...</p>
            <p v-else-if="channels.length === 0" class="empty">No channels yet.</p>

            <div v-else class="channel-list">
              <div
                v-for="channel in channels"
                :key="channel.ID"
                class="channel-row channel-row--stacked"
              >
                <div class="channel-top">
                  <div class="channel-info">
                    <span class="channel-name">{{
                      channel.Name || shortIdByChannel[channel.ID] || channel.ID
                    }}</span>
                    <span class="channel-db">{{ channel.DBName }}</span>
                    <div class="capsule-group">
                      <span class="capsule-group-label">Tables</span>
                      <div class="capsule-row">
                        <span
                          v-for="t in channel.Scope.Tables || []"
                          :key="t"
                          class="value-capsule"
                        >
                          {{ t }}
                        </span>
                        <span v-if="!channel.Scope.Tables?.length" class="value-capsule">none</span>
                      </div>
                    </div>
                    <div class="capsule-group">
                      <span class="capsule-group-label">Operations</span>
                      <div class="capsule-row">
                        <span
                          v-for="op in channel.Scope.Operations || []"
                          :key="op"
                          class="value-capsule"
                        >
                          {{ op }}
                        </span>
                        <span v-if="!channel.Scope.Operations?.length" class="value-capsule"
                          >none</span
                        >
                      </div>
                    </div>
                  </div>

                  <span
                    v-if="shortIdByChannel[channel.ID]"
                    class="channel-code"
                    :title="
                      copiedChannelId === channel.ID
                        ? 'Copied.'
                        : 'Click to copy. Paste it into the agent to use this channel.'
                    "
                    @click="copyShortId(channel.ID)"
                    >{{ shortIdByChannel[channel.ID] }}</span
                  >
                </div>

                <div class="channel-bottom">
                  <span
                    class="status-capsule"
                    :class="statusFor(channel) === 'expired' ? 'status-expired' : 'status-open'"
                  >
                    {{ statusFor(channel) }}
                  </span>
                  <div v-if="activatingId !== channel.ID" class="channel-actions">
                    <button class="ghost-button" @click="openActivate(channel.ID)">
                      {{ statusFor(channel) === 'expired' ? 'Activate' : 'Rotate' }}
                    </button>
                    <button
                      class="ghost-button danger"
                      :disabled="revokingId === channel.ID"
                      @click="revoke(channel.ID)"
                    >
                      {{ revokingId === channel.ID ? 'Removing...' : 'Remove' }}
                    </button>
                  </div>
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
                    {{
                      statusFor(channel) === 'expired'
                        ? activating
                          ? 'Activating...'
                          : 'Activate'
                        : activating
                          ? 'Rotating...'
                          : 'Rotate'
                    }}
                  </button>
                </div>
                <p v-if="activatingId === channel.ID" class="hint">
                  Channel stays open for {{ activateTtlMinutes }} minutes before it needs
                  reactivating.
                </p>
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

              <div class="field-block">
                <label class="field-label" for="new-channel-name">Name</label>
                <input
                  id="new-channel-name"
                  v-model="newChannelName"
                  type="text"
                  placeholder="e.g. debug X"
                />
              </div>

              <div class="field-block">
                <label class="field-label" for="new-channel-db">Connection</label>
                <select id="new-channel-db" v-model="newDbName">
                  <option v-for="name in connections" :key="name" :value="name">{{ name }}</option>
                </select>
              </div>

              <div class="field-block">
                <label class="field-label">Tables</label>
                <p v-if="!newDbName" class="hint">Choose a connection above first.</p>
                <p v-else-if="tablesError" class="error-text">{{ tablesError }}</p>
                <p v-else-if="!tablesLoading && availableTables.length === 0" class="hint">
                  No tables found in this database.
                </p>
                <div v-else ref="tablesDropdownRef" class="combobox">
                  <button
                    type="button"
                    class="combobox-trigger"
                    :disabled="tablesLoading"
                    @click="toggleTablesDropdown"
                  >
                    {{ tablesSummaryLabel }}
                  </button>

                  <div v-if="newTables.length > 0" class="chip-row">
                    <span v-for="table in newTables" :key="table" class="chip">
                      {{ table }}
                      <button
                        type="button"
                        class="chip-remove"
                        title="Remove"
                        @click="removeSelectedTable(table)"
                      >
                        ×
                      </button>
                    </span>
                  </div>

                  <div v-if="tablesDropdownOpen" class="combobox-popover">
                    <input
                      v-model="tableFilter"
                      type="text"
                      class="combobox-filter"
                      placeholder="Filter tables..."
                      autofocus
                    />
                    <div class="checkbox-grid combobox-list">
                      <label v-for="table in filteredTables" :key="table" class="checkbox-option">
                        <input v-model="newTables" type="checkbox" :value="table" />
                        {{ table }}
                      </label>
                      <p v-if="filteredTables.length === 0" class="hint">No tables match.</p>
                    </div>
                  </div>
                </div>
              </div>

              <div class="field-block">
                <label class="field-label">Operations</label>
                <div class="checkbox-grid">
                  <label v-for="op in operationOptions" :key="op" class="checkbox-option">
                    <input v-model="newOperations" type="checkbox" :value="op" />
                    {{ op }}
                  </label>
                </div>
              </div>

              <div class="field-block">
                <label class="field-label" for="new-channel-ttl">TTL</label>
                <input
                  id="new-channel-ttl"
                  v-model.number="newTtlMinutes"
                  type="number"
                  min="1"
                  @keydown.enter="submitCreate"
                />
                <p class="hint">
                  Channel stays open for {{ newTtlMinutes }} minutes before it needs reactivating.
                </p>
              </div>

              <p v-if="createError" class="error-text">{{ createError }}</p>
              <div class="create-actions">
                <button class="ghost-button" @click="cancelCreate">Cancel</button>
                <button
                  class="primary-button"
                  :disabled="!newChannelName.trim() || !newDbName.trim() || creating"
                  @click="submitCreate"
                >
                  {{ creating ? 'Creating...' : 'Create' }}
                </button>
              </div>
            </div>
          </template>
        </template>

        <template v-else-if="activeKind === 'http'">
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

          <template v-if="activeHttpTab === 'domains'">
            <p v-if="domainsError" class="error-text">{{ domainsError }}</p>
            <p v-else-if="domainsLoading" class="empty">Loading...</p>
            <p v-else-if="domains.length === 0" class="empty">No domains yet.</p>

            <div v-else class="channel-list">
              <div v-for="name in domains" :key="name" class="channel-row">
                <div class="channel-info">
                  <span class="channel-db">{{ name }}</span>
                </div>
                <div class="channel-actions">
                  <button
                    class="ghost-button"
                    :disabled="!vaultUnlocked || loadingDomainName === name"
                    :title="!vaultUnlocked ? 'Unlock the vault first' : ''"
                    @click="openDomainEdit(name)"
                  >
                    {{ loadingDomainName === name ? 'Loading...' : 'Edit' }}
                  </button>
                  <button
                    class="ghost-button danger"
                    :disabled="deletingDomainName === name"
                    @click="deleteDomain(name)"
                  >
                    {{ deletingDomainName === name ? 'Deleting...' : 'Delete' }}
                  </button>
                </div>
              </div>
            </div>

            <button
              v-if="!showDomainForm"
              class="primary-button new-channel-button"
              @click="openDomainForm"
            >
              + New domain
            </button>

            <div v-else class="inline-section">
              <h4 class="inline-section-title">
                {{ editingDomainName !== null ? 'Edit domain' : 'New domain' }}
              </h4>

              <div class="field-block">
                <label class="field-label" for="domain-name">Name</label>
                <input
                  id="domain-name"
                  v-model="newDomainName"
                  type="text"
                  placeholder="e.g. staging-api"
                  :disabled="editingDomainName !== null"
                  autofocus
                />
              </div>

              <div class="field-block">
                <label class="field-label" for="domain-base-url">Base URL</label>
                <input
                  id="domain-base-url"
                  v-model="newDomainBaseUrl"
                  type="text"
                  placeholder="https://api.example.com"
                />
              </div>

              <div class="field-block">
                <label class="field-label" for="domain-login-endpoint">Login endpoint</label>
                <input
                  id="domain-login-endpoint"
                  v-model="newDomainLoginEndpoint"
                  type="text"
                  placeholder="/auth/login (leave blank if every user is bring-your-own-token)"
                />
              </div>

              <div class="field-block">
                <label class="field-label" for="domain-token-path">Token path</label>
                <input
                  id="domain-token-path"
                  v-model="newDomainTokenPath"
                  type="text"
                  placeholder="e.g. access_token or data.token"
                />
                <p class="hint">
                  Where the access token lives in the login response, as a dotted JSON path.
                </p>
              </div>

              <div class="field-block">
                <label class="field-label">Token lifetime (TTLOrigin)</label>
                <div class="radio-row">
                  <label class="radio-option">
                    <input v-model="newDomainTtlMode" type="radio" value="fixed" />
                    Fixed
                  </label>
                  <label class="radio-option">
                    <input v-model="newDomainTtlMode" type="radio" value="field" />
                    From response field
                  </label>
                  <label class="radio-option">
                    <input v-model="newDomainTtlMode" type="radio" value="jwt" />
                    Decode as JWT (jwt-exp)
                  </label>
                </div>
                <input
                  v-if="newDomainTtlMode === 'fixed'"
                  v-model.number="newDomainFixedTtlMinutes"
                  type="number"
                  min="1"
                  title="Fixed TTL, minutes"
                />
                <input
                  v-if="newDomainTtlMode === 'field'"
                  v-model="newDomainTtlField"
                  type="text"
                  placeholder="e.g. expires_in"
                />
              </div>

              <div class="field-block">
                <label class="field-label">Token placement</label>
                <div class="field-grid">
                  <div>
                    <select v-model="newDomainTokenKind">
                      <option value="header">Header</option>
                      <option value="cookie">Cookie</option>
                      <option value="query">Query param</option>
                    </select>
                  </div>
                  <div>
                    <input
                      v-model="newDomainTokenName"
                      type="text"
                      placeholder="e.g. Authorization"
                    />
                  </div>
                </div>
                <input
                  v-if="newDomainTokenKind === 'header'"
                  v-model="newDomainTokenPrefix"
                  type="text"
                  placeholder="Prefix, e.g. 'Bearer '"
                />
              </div>

              <div class="field-block">
                <label class="field-label">Users</label>
                <div
                  v-for="(user, userIndex) in newDomainUsers"
                  :key="userIndex"
                  class="user-draft"
                >
                  <div class="user-name-row">
                    <input v-model="user.name" type="text" placeholder="User name" />
                    <label class="radio-option">
                      <input v-model="user.isByot" type="checkbox" />
                      Bring your own token
                    </label>
                  </div>

                  <input
                    v-model="user.tags"
                    type="text"
                    placeholder="Tags, comma separated, e.g. tenant x, low level"
                  />

                  <input
                    v-if="user.isByot"
                    v-model="user.token"
                    type="text"
                    placeholder="Paste the token"
                  />

                  <div v-else class="kv-list">
                    <div
                      v-for="(field, fieldIndex) in user.fields"
                      :key="fieldIndex"
                      class="kv-row"
                    >
                      <input v-model="field.key" type="text" placeholder="field name" />
                      <input v-model="field.value" type="text" placeholder="value" />
                      <button
                        type="button"
                        class="chip-remove"
                        title="Remove field"
                        @click="removeUserField(userIndex, fieldIndex)"
                      >
                        ×
                      </button>
                    </div>
                    <button type="button" class="ghost-button" @click="addUserField(userIndex)">
                      + Field
                    </button>
                  </div>

                  <div class="test-row">
                    <button
                      class="ghost-button"
                      :disabled="user.testStatus === 'testing'"
                      @click="testUserDraft(userIndex)"
                    >
                      {{ user.testStatus === 'testing' ? 'Testing...' : 'Test login' }}
                    </button>
                    <span
                      class="test-status"
                      :class="{
                        'test-status-ok': user.testStatus === 'ok',
                        'test-status-error': user.testStatus === 'error'
                      }"
                    >
                      <template v-if="user.testStatus === 'ok'">✓ OK</template>
                      <template v-else-if="user.testStatus === 'error'"
                        >✗ {{ user.testError }}</template
                      >
                      <template v-else-if="user.testStatus === 'testing'">Testing...</template>
                      <template v-else>Not tested yet</template>
                    </span>
                    <button
                      type="button"
                      class="ghost-button danger"
                      @click="removeUserDraft(userIndex)"
                    >
                      Remove user
                    </button>
                  </div>
                </div>
                <button type="button" class="ghost-button" @click="addUserDraft">+ User</button>
              </div>

              <p v-if="!vaultUnlocked" class="hint">Unlock the vault above before saving.</p>
              <p v-if="domainFormError" class="error-text">{{ domainFormError }}</p>

              <div class="create-actions">
                <button class="ghost-button" @click="cancelDomainForm">Cancel</button>
                <button
                  class="primary-button"
                  :disabled="!canSubmitDomain || creatingDomain"
                  @click="submitDomain"
                >
                  {{ creatingDomain ? 'Saving...' : 'Save' }}
                </button>
              </div>
            </div>
          </template>

          <template v-else>
            <p v-if="httpChannelsListError" class="error-text">{{ httpChannelsListError }}</p>
            <p v-else-if="httpChannelsLoading" class="empty">Loading...</p>
            <p v-else-if="httpChannels.length === 0" class="empty">No HTTP channels yet.</p>

            <div v-else class="channel-list">
              <div
                v-for="channel in httpChannels"
                :key="channel.ID"
                class="channel-row channel-row--stacked"
              >
                <div class="channel-top">
                  <div class="channel-info">
                    <span class="channel-name">{{
                      channel.Name || shortIdByHTTPChannel[channel.ID] || channel.ID
                    }}</span>
                    <span class="channel-db">{{ channel.Domain }}</span>
                    <div class="capsule-group">
                      <span class="capsule-group-label">Methods</span>
                      <div class="capsule-row">
                        <span
                          v-for="m in channel.Scope.Methods || []"
                          :key="m"
                          class="value-capsule"
                        >
                          {{ m }}
                        </span>
                        <span v-if="!channel.Scope.Methods?.length" class="value-capsule"
                          >none</span
                        >
                      </div>
                    </div>
                    <div v-if="channel.Scope.Exclusions?.length" class="capsule-group">
                      <span class="capsule-group-label">Exclusions</span>
                      <div class="capsule-row">
                        <span
                          v-for="(ex, i) in channel.Scope.Exclusions"
                          :key="i"
                          class="value-capsule"
                        >
                          {{ ex.Method }} {{ ex.PathPattern }}
                        </span>
                      </div>
                    </div>
                  </div>

                  <span
                    v-if="shortIdByHTTPChannel[channel.ID]"
                    class="channel-code"
                    :title="
                      copiedHTTPChannelId === channel.ID
                        ? 'Copied.'
                        : 'Click to copy. Paste it into the agent to use this channel.'
                    "
                    @click="copyHTTPShortId(channel.ID)"
                    >{{ shortIdByHTTPChannel[channel.ID] }}</span
                  >
                </div>

                <div class="channel-bottom">
                  <span
                    class="status-capsule"
                    :class="statusForHTTP(channel) === 'expired' ? 'status-expired' : 'status-open'"
                  >
                    {{ statusForHTTP(channel) }}
                  </span>
                  <div v-if="activatingHTTPId !== channel.ID" class="channel-actions">
                    <button class="ghost-button" @click="openHTTPActivate(channel.ID)">
                      {{ statusForHTTP(channel) === 'expired' ? 'Activate' : 'Rotate' }}
                    </button>
                    <button
                      class="ghost-button danger"
                      :disabled="revokingHTTPId === channel.ID"
                      @click="revokeHTTP(channel.ID)"
                    >
                      {{ revokingHTTPId === channel.ID ? 'Removing...' : 'Remove' }}
                    </button>
                  </div>
                </div>

                <div v-if="activatingHTTPId === channel.ID" class="inline-form">
                  <input
                    v-model.number="activateHTTPTtlMinutes"
                    type="number"
                    min="1"
                    title="TTL, minutes"
                    autofocus
                    @keydown.enter="submitHTTPActivate"
                  />
                  <button class="ghost-button" @click="cancelHTTPActivate">Cancel</button>
                  <button
                    class="primary-button"
                    :disabled="activatingHTTP"
                    @click="submitHTTPActivate"
                  >
                    {{
                      statusForHTTP(channel) === 'expired'
                        ? activatingHTTP
                          ? 'Activating...'
                          : 'Activate'
                        : activatingHTTP
                          ? 'Rotating...'
                          : 'Rotate'
                    }}
                  </button>
                </div>
                <p v-if="activatingHTTPId === channel.ID" class="hint">
                  Channel stays open for {{ activateHTTPTtlMinutes }} minutes before it needs
                  reactivating.
                </p>
                <p v-if="activatingHTTPId === channel.ID && activateHTTPError" class="error-text">
                  {{ activateHTTPError }}
                </p>
              </div>
            </div>

            <button
              v-if="!showHTTPCreateForm"
              class="primary-button new-channel-button"
              :disabled="domains.length === 0"
              :title="domains.length === 0 ? 'Add a domain first' : ''"
              @click="openHTTPCreateForm"
            >
              + New channel
            </button>

            <div v-else class="inline-section">
              <h4 class="inline-section-title">New HTTP channel</h4>

              <div class="field-block">
                <label class="field-label" for="new-http-channel-name">Name</label>
                <input
                  id="new-http-channel-name"
                  v-model="newHTTPChannelName"
                  type="text"
                  placeholder="e.g. debug X"
                />
              </div>

              <div class="field-block">
                <label class="field-label" for="new-http-channel-domain">Domain</label>
                <select id="new-http-channel-domain" v-model="newHTTPChannelDomain">
                  <option v-for="name in domains" :key="name" :value="name">{{ name }}</option>
                </select>
              </div>

              <div class="field-block">
                <label class="field-label">Methods</label>
                <div class="checkbox-grid">
                  <label v-for="m in methodOptions" :key="m" class="checkbox-option">
                    <input v-model="newHTTPMethods" type="checkbox" :value="m" />
                    {{ m }}
                  </label>
                </div>
              </div>

              <div class="field-block">
                <label class="field-label">Exclusions</label>
                <p class="hint">Overrides a granted method for one specific path pattern.</p>
                <div v-for="(ex, i) in newHTTPExclusions" :key="i" class="exclusion-row">
                  <select v-model="ex.method">
                    <option v-for="m in methodOptions" :key="m" :value="m">{{ m }}</option>
                  </select>
                  <input
                    v-model="ex.pathPattern"
                    type="text"
                    placeholder="/users/*/change-password"
                  />
                  <button
                    type="button"
                    class="chip-remove"
                    title="Remove exclusion"
                    @click="removeExclusion(i)"
                  >
                    ×
                  </button>
                </div>
                <button type="button" class="ghost-button" @click="addExclusion">
                  + Exclusion
                </button>
              </div>

              <div class="field-block">
                <label class="field-label" for="new-http-channel-ttl">TTL</label>
                <input
                  id="new-http-channel-ttl"
                  v-model.number="newHTTPTtlMinutes"
                  type="number"
                  min="1"
                  @keydown.enter="submitHTTPCreate"
                />
                <p class="hint">
                  Channel stays open for {{ newHTTPTtlMinutes }} minutes before it needs
                  reactivating.
                </p>
              </div>

              <p v-if="createHTTPError" class="error-text">{{ createHTTPError }}</p>
              <div class="create-actions">
                <button class="ghost-button" @click="cancelHTTPCreate">Cancel</button>
                <button
                  class="primary-button"
                  :disabled="
                    !newHTTPChannelName.trim() ||
                    !newHTTPChannelDomain.trim() ||
                    creatingHTTPChannel
                  "
                  @click="submitHTTPCreate"
                >
                  {{ creatingHTTPChannel ? 'Creating...' : 'Create' }}
                </button>
              </div>
            </div>
          </template>
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

.close-button {
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
  width: 28px;
  height: 28px;
  background: transparent;
  border: none;
  border-radius: var(--radius-pill);
  font-size: var(--text-lg);
  line-height: 1;
  color: var(--color-fg-muted);
  cursor: pointer;
  transition:
    background var(--duration-fast) var(--ease-out),
    color var(--duration-fast) var(--ease-out);
}

.close-button:hover {
  background: var(--color-surface-2);
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

.channel-code {
  flex-shrink: 0;
  cursor: pointer;
  padding: 1px var(--space-3);
  border-radius: var(--radius-pill);
  background: var(--color-amber);
  color: var(--color-black);
  font-family: var(--font-mono);
  font-weight: var(--weight-bold);
  font-size: var(--text-md);
  letter-spacing: 0.05em;
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

/* Channels tab only -- actions on their own line, right-justified, below the info panel. */
.channel-row--stacked {
  flex-direction: column;
  align-items: stretch;
}

.channel-top {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: var(--space-3);
}

.channel-info {
  display: flex;
  flex-direction: column;
  gap: var(--space-1);
  font-family: var(--font-body);
  font-size: var(--text-sm);
}

.channel-name {
  color: var(--color-fg-primary);
  font-weight: var(--weight-medium);
  font-size: var(--text-base);
}

.channel-db {
  color: var(--color-fg-secondary);
  font-size: var(--text-sm);
}

.capsule-group {
  display: flex;
  align-items: center;
  gap: var(--space-2);
}

.capsule-group-label {
  min-width: 64px;
  color: var(--color-fg-muted);
  font-size: var(--text-xs);
}

.capsule-row {
  display: flex;
  flex-wrap: wrap;
  gap: var(--space-1);
}

.value-capsule {
  padding: 1px var(--space-2);
  border-radius: var(--radius-pill);
  background: var(--color-surface-3);
  color: var(--color-fg-secondary);
  font-family: var(--font-mono);
  font-size: var(--text-xs);
}

.status-capsule {
  padding: 1px var(--space-2);
  border-radius: var(--radius-pill);
  font-size: var(--text-xs);
  background: var(--color-surface-1);
  color: var(--color-fg-muted);
}

.status-ok,
.status-open {
  background: color-mix(in srgb, var(--color-success) 18%, transparent);
  color: var(--color-success);
}

.status-error {
  background: color-mix(in srgb, var(--color-danger, #e5484d) 18%, transparent);
  color: var(--color-danger, #e5484d);
}

.status-verifying,
.status-expired {
  color: var(--color-fg-muted);
}

/* Connections tab: the capsule sits in a column flex, so it needs its own shrink-to-content
   alignment instead of stretching full width. */
.channel-info > .status-capsule {
  align-self: flex-start;
}

.channel-bottom {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: var(--space-2);
}

.channel-actions {
  display: flex;
  align-items: center;
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

.field-block {
  display: flex;
  flex-direction: column;
  gap: var(--space-1);
}

.field-block select {
  width: 100%;
}

.checkbox-grid {
  display: flex;
  flex-wrap: wrap;
  gap: var(--space-3);
  max-height: 160px;
  overflow-y: auto;
}

.checkbox-option {
  display: flex;
  align-items: center;
  gap: var(--space-1);
  color: var(--color-fg-primary);
  font-family: var(--font-body);
  font-size: var(--text-sm);
  cursor: pointer;
}

.combobox {
  display: flex;
  flex-direction: column;
  gap: var(--space-2);
}

.combobox-trigger {
  display: flex;
  align-items: center;
  justify-content: space-between;
  width: 100%;
  padding: var(--space-2) var(--space-3);
  background: var(--color-surface-1);
  border: 1px solid var(--color-border-default);
  border-radius: var(--radius-md);
  color: var(--color-fg-primary);
  font-family: var(--font-body);
  font-size: var(--text-sm);
  text-align: left;
  cursor: pointer;
}

.combobox-trigger:disabled {
  opacity: 0.5;
  cursor: default;
}

.combobox-popover {
  display: flex;
  flex-direction: column;
  gap: var(--space-2);
  padding: var(--space-2);
  background: var(--color-surface-1);
  border: 1px solid var(--color-border-default);
  border-radius: var(--radius-md);
}

.combobox-filter {
  width: 100%;
}

.combobox-list {
  flex-wrap: nowrap;
  flex-direction: column;
  max-height: 200px;
}

.chip-row {
  display: flex;
  flex-wrap: wrap;
  gap: var(--space-1);
}

.chip {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  padding: 2px var(--space-2);
  background: var(--color-surface-2);
  border-radius: var(--radius-pill);
  color: var(--color-fg-primary);
  font-size: var(--text-xs);
}

.chip-remove {
  background: transparent;
  border: none;
  color: var(--color-fg-muted);
  cursor: pointer;
  font-size: var(--text-sm);
  line-height: 1;
  padding: 0;
}

.chip-remove:hover {
  color: var(--color-fg-primary);
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

.kind-row {
  margin-bottom: var(--space-1);
}

.radio-row {
  display: flex;
  flex-wrap: wrap;
  gap: var(--space-3);
  margin-bottom: var(--space-2);
}

.radio-option {
  display: flex;
  align-items: center;
  flex-shrink: 0;
  gap: var(--space-1);
  color: var(--color-fg-primary);
  font-family: var(--font-body);
  font-size: var(--text-sm);
  white-space: nowrap;
  cursor: pointer;
}

.user-name-row {
  display: flex;
  align-items: center;
  gap: var(--space-3);
}

.user-name-row input[type='text'] {
  flex: 1;
  min-width: 0;
}

.user-draft {
  display: flex;
  flex-direction: column;
  gap: var(--space-2);
  padding: var(--space-3);
  margin-bottom: var(--space-2);
  background: var(--color-surface-1);
  border: 1px solid var(--color-border-default);
  border-radius: var(--radius-md);
}

.kv-list {
  display: flex;
  flex-direction: column;
  gap: var(--space-1);
}

.kv-row,
.exclusion-row {
  display: flex;
  align-items: center;
  gap: var(--space-2);
  margin-bottom: var(--space-1);
}

.kv-row input,
.exclusion-row input {
  flex: 1;
}
</style>
