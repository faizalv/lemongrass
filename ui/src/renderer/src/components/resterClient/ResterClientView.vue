<!-- SPDX-License-Identifier: MPL-2.0 -->
<!-- Copyright (C) 2026 The Lemongrass Authors -->

<script setup lang="ts">
import { computed, onMounted, reactive, ref, watch } from 'vue'
import { AppButton, AppEmptyState, AppIcon, AppIconButton, AppInput, AppSwitcher } from '@sankasten'
import '../../resterClient.css'
import VaultUnlockInline from '../VaultUnlockInline.vue'
import RequestList from './RequestList.vue'
import RequestEditor from './RequestEditor.vue'
import ResponsePanel from './ResponsePanel.vue'
import { describeError, noteVaultActivity, vaultSession } from '../../vaultSession'
import { plainCopy } from '../connector/rester/resterDraft'
import { looksLikeCurl, matchBaseURL, parseCurl, type BaseURLTarget } from '../../curlRequest'
import { apiUsers, buildCall } from '../../resterClientCall'
import {
  clearHistory,
  deleteRun,
  loadHistory,
  loadResterClient,
  recordRun,
  removeRequest as removeStoredRequest,
  resterClientStore
} from '../../resterClientStore'
import type {
  ResterClientDownload,
  ResterClientRequest,
  ResterClientResult,
  ResterClientRun,
  VaultRester
} from '../../../../preload/types'
import { noUserIcon, plainRequestIcon, resterClientIcon } from '../../icons'

const METHODS = ['GET', 'POST', 'PUT', 'PATCH', 'DELETE', 'HEAD', 'OPTIONS']
// Where the last selected request is remembered, so reopening the app shows it again.
const SELECTED_KEY = 'resterClient.selectedId'
// Where the Request box's share of the split and the minimized box are remembered.
const SPLIT_KEY = 'resterClient.split'
const COLLAPSED_KEY = 'resterClient.collapsed'
// Each box keeps this width while both are open, so a drag can never squeeze one away.
const BOX_MIN_PX = 280
// The picked rester and environment, which decide the group the list shows, and the request last
// selected in each group, by rester.
const SCOPE_KEY = 'resterClient.scope'
const LAST_KEY = 'resterClient.lastByRester'
const SIDEBAR_KEY = 'resterClient.sidebarWidth'
const SIDEBAR_DEFAULT_PX = 240
const SIDEBAR_MIN_PX = 180
const SIDEBAR_MAX_PX = 480
// A request with no rester is Plain: a full URL sent as written, from main and not through the
// vault, since no credential is involved. Its group is Plain.
const PLAIN_ID = targetId('', '')
// The user picker's id for No user, which a request saves as an empty user. A NUL never appears in
// a user name, so it cannot collide with one.
const NO_USER_ID = '\u0000no-user'

interface Scope {
  rester: string
  environment: string
}

type Box = 'request' | 'response'

interface Outcome {
  result: ResterClientResult | null
  download: ResterClientDownload | null
  error: string
  sending: boolean
  notice: string
  // The run the outcome shows and when it came back, null before any run.
  runId: string | null
  at: string | null
  bodyCut: boolean
  // Set once the newest stored run has been looked up for this request.
  restored: boolean
}

const requests = computed(() => resterClientStore.requests)
const selectedId = ref<string | null>(null)
const outcomes = reactive<Record<string, Outcome>>({})

const runs = computed(() =>
  selectedId.value ? (resterClientStore.histories[selectedId.value] ?? []) : []
)

function storedSplit(): number {
  const value = Number(localStorage.getItem(SPLIT_KEY))
  return value > 0 && value < 1 ? value : 0.5
}

function storedCollapsed(): Box | null {
  const value = localStorage.getItem(COLLAPSED_KEY)
  return value === 'request' || value === 'response' ? value : null
}

function storedScope(): Scope | null {
  try {
    const value = JSON.parse(localStorage.getItem(SCOPE_KEY) ?? 'null')
    return typeof value?.rester === 'string' && typeof value?.environment === 'string'
      ? { rester: value.rester, environment: value.environment }
      : null
  } catch {
    return null
  }
}

function storedLast(): Record<string, string> {
  try {
    const value = JSON.parse(localStorage.getItem(LAST_KEY) ?? '{}')
    return value && typeof value === 'object' ? value : {}
  } catch {
    return {}
  }
}

function storedSidebarWidth(): number {
  const value = Number(localStorage.getItem(SIDEBAR_KEY))
  return value >= SIDEBAR_MIN_PX && value <= SIDEBAR_MAX_PX ? value : SIDEBAR_DEFAULT_PX
}

const split = ref(storedSplit())
// At most one box is minimized, so one is always open.
const collapsed = ref<Box | null>(storedCollapsed())
const panesEl = ref<HTMLDivElement>()
let dragging = false

watch(collapsed, (value) => {
  if (value) localStorage.setItem(COLLAPSED_KEY, value)
  else localStorage.removeItem(COLLAPSED_KEY)
})

// The gutter captures the pointer so the release always reaches it, even outside the window.
function onGutterDown(event: PointerEvent): void {
  if (event.button !== 0) return
  ;(event.currentTarget as HTMLElement).setPointerCapture(event.pointerId)
  dragging = true
}

function onGutterMove(event: PointerEvent): void {
  if (!dragging || !panesEl.value) return
  // The boxes share the width left after the gutter and the two gaps beside it.
  const gutter = event.currentTarget as HTMLElement
  const rect = panesEl.value.getBoundingClientRect()
  const gap = parseFloat(getComputedStyle(panesEl.value).columnGap) || 0
  const taken = gutter.offsetWidth + 2 * gap
  const usable = rect.width - taken
  if (usable <= 0) return
  const min = Math.min(0.5, BOX_MIN_PX / usable)
  const ratio = (event.clientX - rect.left - taken / 2) / usable
  split.value = Math.max(min, Math.min(1 - min, ratio))
}

function onGutterUp(): void {
  if (!dragging) return
  dragging = false
  localStorage.setItem(SPLIT_KEY, String(split.value))
}

function resetSplit(): void {
  split.value = 0.5
  localStorage.setItem(SPLIT_KEY, '0.5')
}

const sidebarWidth = ref(storedSidebarWidth())
let sidebarDrag: { x: number; width: number } | null = null

function onSidebarDown(event: PointerEvent): void {
  if (event.button !== 0) return
  ;(event.currentTarget as HTMLElement).setPointerCapture(event.pointerId)
  sidebarDrag = { x: event.clientX, width: sidebarWidth.value }
}

function onSidebarMove(event: PointerEvent): void {
  if (!sidebarDrag) return
  const width = sidebarDrag.width + event.clientX - sidebarDrag.x
  sidebarWidth.value = Math.max(SIDEBAR_MIN_PX, Math.min(SIDEBAR_MAX_PX, width))
}

function onSidebarUp(): void {
  if (!sidebarDrag) return
  sidebarDrag = null
  localStorage.setItem(SIDEBAR_KEY, String(sidebarWidth.value))
}

function resetSidebar(): void {
  sidebarWidth.value = SIDEBAR_DEFAULT_PX
  localStorage.setItem(SIDEBAR_KEY, String(SIDEBAR_DEFAULT_PX))
}

const resters = ref<Record<string, VaultRester>>({})
const restersError = ref('')
const restersLoading = ref(false)
// Set after the first load, so a request's rester is called missing only once the vault answered.
const restersLoaded = ref(false)

const selected = computed(() => requests.value.find((r) => r.id === selectedId.value) ?? null)

function outcomeOf(id: string): Outcome {
  if (!outcomes[id]) {
    outcomes[id] = {
      result: null,
      download: null,
      error: '',
      sending: false,
      notice: '',
      runId: null,
      at: null,
      bodyCut: false,
      restored: false
    }
  }
  return outcomes[id]
}

const outcome = computed(() => (selected.value ? outcomeOf(selected.value.id) : null))

// Every rester environment with an api side, which is what a request can target.
const targets = computed<BaseURLTarget[]>(() =>
  Object.entries(resters.value).flatMap(([name, rester]) =>
    rester.API
      ? rester.Environments.filter((env) => env.BaseURL).map((env) => ({
          rester: name,
          environment: env.Name,
          baseURL: env.BaseURL
        }))
      : []
  )
)

function targetId(rester: string, environment: string): string {
  return JSON.stringify([rester, environment])
}

const pickedScope = ref<Scope | null>(storedScope())
const lastByRester = storedLast()

// Plain never reaches the vault, so a locked vault leaves the tab usable with Plain only.
const locked = computed(() => !vaultSession.unlocked)

// While locked the scope is Plain, whatever was picked, so unlocking returns to the picked group.
// Before anything is picked: the first request's group, else the first target, else Plain.
const scope = computed<Scope>(() => {
  if (locked.value) return { rester: '', environment: '' }
  if (pickedScope.value) return pickedScope.value
  const first = requests.value[0] ?? targets.value[0]
  return first
    ? { rester: first.rester, environment: first.environment }
    : { rester: '', environment: '' }
})

function setScope(next: Scope): void {
  if (locked.value) return
  pickedScope.value = next
  localStorage.setItem(SCOPE_KEY, JSON.stringify(next))
}

const isPlain = computed(() => scope.value.rester === '')
const groupRequests = computed(() => requests.value.filter((r) => r.rester === scope.value.rester))

// Plain is pinned on top. A target a saved request names but the vault no longer has stays
// listed, so the requests under it remain reachable.
const targetItems = computed(() => {
  const items: {
    id: string
    label: string
    title?: string
    hint?: string
    icon?: typeof plainRequestIcon
    pinned?: boolean
  }[] = [
    {
      id: PLAIN_ID,
      label: 'Plain',
      title: 'A full URL sent as written, with no rester and no user',
      icon: plainRequestIcon,
      pinned: true
    }
  ]
  const listed = new Set([PLAIN_ID])
  for (const t of targets.value) {
    const id = targetId(t.rester, t.environment)
    listed.add(id)
    items.push({ id, label: `${t.rester} · ${t.environment}`, title: t.baseURL })
  }
  if (restersLoaded.value) {
    for (const r of requests.value) {
      const id = targetId(r.rester, r.environment)
      if (listed.has(id)) continue
      listed.add(id)
      items.push({ id, label: `${r.rester} · ${r.environment}`, hint: 'not in the vault' })
    }
  }
  return items
})

const selectedTarget = computed(() => targetId(scope.value.rester, scope.value.environment))

const baseURL = computed(
  () =>
    targets.value.find(
      (t) => t.rester === selected.value?.rester && t.environment === selected.value?.environment
    )?.baseURL ?? ''
)

const userItems = computed(() => {
  const rester = selected.value ? resters.value[selected.value.rester] : undefined
  if (!rester || !selected.value) return []
  return [
    {
      id: NO_USER_ID,
      label: 'No user',
      title: 'Sends no token. Auth headers you write go out as written.',
      icon: noUserIcon,
      pinned: true
    },
    ...apiUsers(rester, selected.value.environment).map((user) => ({
      id: user.Name,
      label: user.Tags?.length ? `${user.Name} (${user.Tags.join(', ')})` : user.Name,
      search: user.Name
    }))
  ]
})

const resolvedURL = computed(() => {
  const url = selected.value?.url.trim() ?? ''
  if (!url || url.includes('://') || !baseURL.value) return ''
  return baseURL.value.replace(/\/+$/, '') + (url.startsWith('/') ? url : '/' + url)
})

const canSend = computed(
  () => !!selected.value && !!selected.value.url.trim() && !outcome.value?.sending
)

// Loads can overlap when resters change in quick succession; only the newest one is applied.
let restersLoad = 0

async function loadResters(): Promise<void> {
  const load = ++restersLoad
  restersLoading.value = true
  restersError.value = ''
  try {
    const names = await window.api.vault.listResters()
    const loadedResters: Record<string, VaultRester> = {}
    for (const name of names) {
      loadedResters[name] = await window.api.vault.getRester(vaultSession.passphrase, name)
    }
    if (load === restersLoad) {
      resters.value = loadedResters
      restersLoaded.value = true
    }
  } catch (err) {
    if (load === restersLoad) restersError.value = describeError(err)
  } finally {
    if (load === restersLoad) restersLoading.value = false
  }
}

// The tab is keep-alive, so it reloads on unlock and after every rester change made in the Vault.
// A lock drops the resters, since they hold user fields, and cancels a load still running.
watch(
  () => [vaultSession.unlocked, vaultSession.resterVersion] as const,
  ([unlocked]) => {
    if (unlocked) {
      loadResters()
      return
    }
    restersLoad++
    resters.value = {}
    restersLoaded.value = false
    restersLoading.value = false
    restersError.value = ''
  },
  { immediate: true }
)

// Keeps the selection in the group the scope allows: Plain while locked, the picked group once
// unlocked.
function selectForScope(): void {
  const rester = scope.value.rester
  if (selected.value?.rester === rester) return
  selectedId.value = rememberedIn(rester)?.id ?? null
}

watch(locked, selectForScope)

onMounted(async () => {
  await loadResterClient()
  const remembered = localStorage.getItem(SELECTED_KEY)
  if (selectedId.value) return
  if (requests.value.some((r) => r.id === remembered)) selectedId.value = remembered
  else selectedId.value = rememberedIn(scope.value.rester)?.id ?? null
  if (locked.value) selectForScope()
})

// The request last selected in a rester's group, else its first.
function rememberedIn(rester: string): ResterClientRequest | undefined {
  const group = requests.value.filter((r) => r.rester === rester)
  return group.find((r) => r.id === lastByRester[rester]) ?? group[0]
}

// The scope follows the selected request, so a curl paste that moves it to another rester, or a
// run that put back another environment, moves the list with it.
watch(
  () =>
    selected.value &&
    ([selected.value.id, selected.value.rester, selected.value.environment] as const),
  (now) => {
    if (!now) return
    const [id, rester, environment] = now
    if (rester !== scope.value.rester || environment !== scope.value.environment) {
      setScope({ rester, environment })
    }
    if (lastByRester[rester] !== id) {
      lastByRester[rester] = id
      localStorage.setItem(LAST_KEY, JSON.stringify(lastByRester))
    }
  },
  { immediate: true }
)

watch(selectedId, async (id) => {
  if (!id) return
  localStorage.setItem(SELECTED_KEY, id)
  await loadHistory(id)
  const state = outcomeOf(id)
  if (state.restored) return
  state.restored = true
  const newest = resterClientStore.histories[id]?.[0]
  // A send that started before the history arrived already shows its own run.
  if (newest && !state.runId && !state.sending) showRun(state, newest)
})

function addRequest(): void {
  const current = selected.value
  const { rester, environment } = scope.value
  const request: ResterClientRequest = {
    id: crypto.randomUUID(),
    name: '',
    rester,
    environment,
    user: rester && current?.rester === rester ? current.user : '',
    method: 'GET',
    url: '',
    headers: [],
    bodyKind: 'none',
    json: '',
    form: [],
    multipart: [],
    file: { path: '', size: 0, contentType: '' }
  }
  resterClientStore.requests.push(request)
  selectedId.value = request.id
  if (rester && current?.rester !== rester) pickDefaultUser(request, false)
}

function removeRequest(id: string): void {
  const index = groupRequests.value.findIndex((r) => r.id === id)
  if (index < 0) return
  removeStoredRequest(id)
  delete outcomes[id]
  if (selectedId.value === id) {
    const group = groupRequests.value
    selectedId.value = group[Math.min(index, group.length - 1)]?.id ?? null
  }
}

function renameRequest(id: string, name: string): void {
  const request = requests.value.find((r) => r.id === id)
  if (request) request.name = name
}

// Keeps the request's user when the new environment has it, else takes the first usable one.
// keepNoUser keeps a deliberate No user (an empty user), for a request that stays in its rester.
function pickDefaultUser(request: ResterClientRequest, keepNoUser: boolean): void {
  if (keepNoUser && request.user === '') return
  const rester = resters.value[request.rester]
  const users = rester ? apiUsers(rester, request.environment) : []
  if (!users.some((u) => u.Name === request.user)) request.user = users[0]?.Name ?? ''
}

// Another environment of the same rester retargets the selected request. Another rester switches
// the list to its group and reselects the request last used there, moved to the picked environment.
function pickTarget(id: string): void {
  const [rester, environment] = JSON.parse(id) as [string, string]
  const sameGroup = rester === scope.value.rester
  setScope({ rester, environment })
  const request = sameGroup ? selected.value : (rememberedIn(rester) ?? null)
  if (!sameGroup) selectedId.value = request?.id ?? null
  // A rester missing from the vault has no users to pick, so its requests are left as saved.
  if (!request || !rester || !resters.value[rester]) return
  request.environment = environment
  pickDefaultUser(request, true)
}

function tokenHeaderOf(request: ResterClientRequest): string {
  const placement = resters.value[request.rester]?.API?.TokenPlacement
  return placement?.Kind === 'header' ? placement.Name : ''
}

// A pasted curl command fills the selected request in place rather than adding a new one.
function onUrlPaste(event: ClipboardEvent): void {
  const text = event.clipboardData?.getData('text/plain') ?? ''
  if (!selected.value || !looksLikeCurl(text)) return
  event.preventDefault()
  const request = selected.value
  const pastedURL = parseCurl(text).url
  const match = matchBaseURL(pastedURL, targets.value)
  // A full URL under no rester environment makes the request Plain, which keeps its auth.
  const becamePlain = !match && request.rester !== '' && pastedURL.includes('://')
  if (match) {
    const sameRester = request.rester === match.target.rester
    request.rester = match.target.rester
    request.environment = match.target.environment
    pickDefaultUser(request, sameRester)
  } else if (becamePlain) {
    request.rester = ''
    request.environment = ''
    request.user = ''
  }
  const parsed =
    request.rester === '' || request.user === ''
      ? parseCurl(text, '', true)
      : parseCurl(text, tokenHeaderOf(request))
  request.method = parsed.method
  request.url = match ? match.path : parsed.url
  request.headers = parsed.headers
  request.bodyKind = parsed.bodyKind
  request.json = parsed.json
  request.form = parsed.form
  request.multipart = parsed.multipart
  request.file = parsed.file

  const notes: string[] = []
  if (parsed.dropped.length) {
    notes.push(`Left out ${parsed.dropped.join(', ')}, since the chosen user supplies auth.`)
  }
  if (match) {
    notes.push(`Matched ${match.target.rester} · ${match.target.environment}.`)
  } else if (becamePlain) {
    notes.push('The URL is under no rester environment, so it is now a Plain request.')
  } else if (locked.value && pastedURL.includes('://')) {
    notes.push('Unlock the vault to match the URL to a rester.')
  }
  notes.push(...parsed.notes)
  outcomeOf(request.id).notice = notes.join(' ')
}

function snapshot(request: ResterClientRequest): ResterClientRun['request'] {
  const copy = plainCopy(request) as Partial<ResterClientRequest>
  delete copy.id
  delete copy.name
  return copy as ResterClientRun['request']
}

async function send(download: boolean): Promise<void> {
  const request = selected.value
  if (!request || !canSend.value) return
  const state = outcomeOf(request.id)
  state.sending = true
  state.error = ''
  state.notice = ''
  state.runId = crypto.randomUUID()
  state.at = null
  state.bodyCut = false
  noteVaultActivity()
  const sent = snapshot(request)
  const runId = state.runId
  const at = new Date().toISOString()
  try {
    const call = plainCopy(buildCall(request))
    if (download) {
      const result = await window.api.resterClient.download(vaultSession.passphrase, call)
      state.download = result.kind === 'inline' ? null : result
      state.result = result.kind === 'inline' ? result.result : null
    } else {
      state.result = await window.api.resterClient.request(vaultSession.passphrase, call)
      state.download = null
    }
  } catch (err) {
    state.error = describeError(err)
    state.result = null
    state.download = null
  } finally {
    state.sending = false
  }
  if (state.download?.kind === 'cancelled') return
  state.at = at
  recordRun(request.id, {
    id: runId,
    at,
    request: sent,
    result: state.result,
    download: state.download,
    error: state.error,
    bodyCut: false
  }).catch((err) => (state.notice = `The run was not kept in history: ${describeError(err)}`))
}

function showRun(state: Outcome, run: ResterClientRun): void {
  state.result = run.result
  state.download = run.download
  state.error = run.error
  state.notice = ''
  state.runId = run.id
  state.at = run.at
  state.bodyCut = run.bodyCut
}

// Loads a run's parameters into its request and shows the response it got.
function pickRun(run: ResterClientRun): void {
  const request = selected.value
  if (!request) return
  Object.assign(request, plainCopy(run.request))
  showRun(outcomeOf(request.id), run)
}

function removeRun(runId: string): void {
  if (selectedId.value) deleteRun(selectedId.value, runId)
}

function clearRuns(): void {
  if (selectedId.value) clearHistory(selectedId.value)
}

async function flush(): Promise<void> {
  const request = selected.value
  if (!request?.rester || !request.user) return
  const state = outcomeOf(request.id)
  try {
    await window.api.resterClient.flush(request.rester, request.environment, request.user)
    state.notice = `Dropped ${request.user}'s token. The next send logs in again.`
  } catch (err) {
    state.notice = describeError(err)
  }
}

function onUrlKeydown(event: KeyboardEvent): void {
  if (event.key === 'Enter') send(event.shiftKey)
}
</script>

<template>
  <div
    class="rester-client"
    @click.capture="noteVaultActivity"
    @keydown.capture="noteVaultActivity"
  >
    <div class="client-frame">
      <nav class="client-sidebar" :style="{ width: `${sidebarWidth}px` }">
        <div class="sidebar-actions">
          <span class="sidebar-label">Requests</span>
          <AppButton size="sm" @click="addRequest">+ Request</AppButton>
        </div>
        <RequestList
          :requests="groupRequests"
          :selected-id="selectedId"
          @select="selectedId = $event"
          @remove="removeRequest"
          @rename="renameRequest"
        />
      </nav>
      <div
        class="sidebar-gutter"
        title="Drag to resize. Double-click to reset."
        @pointerdown="onSidebarDown"
        @pointermove="onSidebarMove"
        @pointerup="onSidebarUp"
        @pointercancel="onSidebarUp"
        @dblclick="resetSidebar"
      />

      <section class="client-main">
        <div class="target-row">
          <div class="target-picker">
            <AppSwitcher
              framed
              :searchable="!locked"
              search-placeholder="Search resters"
              :items="targetItems"
              :active-id="selectedTarget"
              placeholder="Pick a rester"
              :empty-text="
                locked
                  ? 'Unlock the vault to use a rester.'
                  : restersLoading
                    ? 'Loading resters...'
                    : 'No rester has an API environment yet.'
              "
              @select="pickTarget"
            >
              <template v-if="locked" #footer>
                <VaultUnlockInline purpose="use a rester" />
              </template>
            </AppSwitcher>
          </div>
          <div v-if="selected && !isPlain" class="user-picker">
            <AppSwitcher
              framed
              searchable
              search-placeholder="Search users"
              :items="userItems"
              :active-id="selected.user || NO_USER_ID"
              placeholder="Pick a user"
              empty-text="No user can call this environment."
              @select="selected.user = $event === NO_USER_ID ? '' : $event"
            />
          </div>
          <AppButton
            v-if="selected && !isPlain && selected.user"
            title="Drop this user's cached token"
            @click="flush"
          >
            Flush token
          </AppButton>
        </div>
        <p v-if="restersError" class="error-text">{{ restersError }}</p>

        <AppEmptyState
          v-if="!selected || !outcome"
          layout="fill"
          :icon="resterClientIcon"
          title="Add a request to start."
          detail="Paste a curl command into its URL box to fill it."
        />

        <div v-if="selected && outcome" class="url-row">
          <div class="method-picker">
            <AppSwitcher
              framed
              :items="METHODS.map((m) => ({ id: m, label: m }))"
              :active-id="selected.method"
              placeholder="GET"
              empty-text=""
              @select="selected.method = $event"
            />
          </div>
          <AppInput
            v-model="selected.url"
            mono
            class="url-input"
            type="text"
            spellcheck="false"
            :placeholder="
              isPlain
                ? 'https://example.com/path, or paste a curl command'
                : '/path, a full URL, or paste a curl command'
            "
            @paste="onUrlPaste"
            @keydown="onUrlKeydown"
          />
          <AppButton variant="primary" :disabled="!canSend" @click="send(false)">Send</AppButton>
          <AppButton :disabled="!canSend" @click="send(true)">Send and download</AppButton>
        </div>
        <p v-if="resolvedURL" class="hint resolved">{{ resolvedURL }}</p>
        <p v-if="outcome?.notice" class="hint notice">{{ outcome.notice }}</p>

        <div v-if="selected && outcome" ref="panesEl" class="panes">
          <div v-if="collapsed === 'request'" class="box-rail" @click="collapsed = null">
            <AppIconButton size="sm" label="Show Request" @click="collapsed = null">
              <AppIcon name="chevronRight" />
            </AppIconButton>
            <span class="rail-title">Request</span>
          </div>
          <RequestEditor
            v-show="collapsed !== 'request'"
            :model-value="selected"
            :style="{ flexGrow: collapsed ? 1 : split }"
            @collapse="collapsed = 'request'"
          />
          <div
            v-if="!collapsed"
            class="gutter"
            title="Drag to resize. Double-click to split evenly."
            @pointerdown="onGutterDown"
            @pointermove="onGutterMove"
            @pointerup="onGutterUp"
            @pointercancel="onGutterUp"
            @dblclick="resetSplit"
          />
          <ResponsePanel
            v-show="collapsed !== 'response'"
            :style="{ flexGrow: collapsed ? 1 : 1 - split }"
            :result="outcome.result"
            :download="outcome.download"
            :error="outcome.error"
            :sending="outcome.sending"
            :body-cut="outcome.bodyCut"
            :at="outcome.at"
            :runs="runs"
            :run-id="outcome.runId"
            @pick-run="pickRun"
            @remove-run="removeRun"
            @clear-history="clearRuns"
            @collapse="collapsed = 'response'"
          />
          <div v-if="collapsed === 'response'" class="box-rail" @click="collapsed = null">
            <AppIconButton size="sm" label="Show Response" @click="collapsed = null">
              <AppIcon name="chevronRight" class="flip" />
            </AppIconButton>
            <span class="rail-title">Response</span>
          </div>
        </div>
      </section>
    </div>
  </div>
</template>
