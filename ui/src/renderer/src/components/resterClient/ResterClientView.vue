<!-- SPDX-License-Identifier: AGPL-3.0-only -->
<!-- Copyright (C) 2026 The Lemongrass Authors -->

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, reactive, ref, watch } from 'vue'
import '../../resterClient.css'
import AppButton from '../AppButton.vue'
import AppSwitcher from '../AppSwitcher.vue'
import VaultUnlockGate from '../VaultUnlockGate.vue'
import RequestList from './RequestList.vue'
import RequestEditor from './RequestEditor.vue'
import ResponsePanel from './ResponsePanel.vue'
import { describeError, noteVaultActivity, vaultSession } from '../../vaultSession'
import { plainCopy } from '../connector/rester/resterDraft'
import { looksLikeCurl, matchBaseURL, parseCurl, type BaseURLTarget } from '../../curlRequest'
import { apiUsers, buildCall } from '../../resterClientCall'
import type { ProjectRef } from '../../workspace'
import type {
  ResterClientDownload,
  ResterClientRequest,
  ResterClientResult,
  VaultRester
} from '../../../../preload/types'

const props = defineProps<{
  project: ProjectRef
}>()

const METHODS = ['GET', 'POST', 'PUT', 'PATCH', 'DELETE', 'HEAD', 'OPTIONS']
const SAVE_MS = 400

interface Outcome {
  result: ResterClientResult | null
  download: ResterClientDownload | null
  error: string
  sending: boolean
  notice: string
}

const requests = ref<ResterClientRequest[]>([])
const selectedId = ref<string | null>(null)
const loaded = ref(false)
const outcomes = reactive<Record<string, Outcome>>({})

const resters = ref<Record<string, VaultRester>>({})
const restersError = ref('')
const restersLoading = ref(false)

const selected = computed(() => requests.value.find((r) => r.id === selectedId.value) ?? null)

function outcomeOf(id: string): Outcome {
  if (!outcomes[id]) {
    outcomes[id] = { result: null, download: null, error: '', sending: false, notice: '' }
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

const targetItems = computed(() =>
  targets.value.map((t) => ({
    id: targetId(t.rester, t.environment),
    label: `${t.rester} · ${t.environment}`,
    title: t.baseURL
  }))
)

const selectedTarget = computed(() =>
  selected.value?.rester ? targetId(selected.value.rester, selected.value.environment) : null
)

const baseURL = computed(
  () =>
    targets.value.find(
      (t) => t.rester === selected.value?.rester && t.environment === selected.value?.environment
    )?.baseURL ?? ''
)

const userItems = computed(() => {
  const rester = selected.value ? resters.value[selected.value.rester] : undefined
  if (!rester || !selected.value) return []
  return apiUsers(rester, selected.value.environment).map((user) => ({
    id: user.Name,
    label: user.Tags?.length ? `${user.Name} (${user.Tags.join(', ')})` : user.Name
  }))
})

const resolvedURL = computed(() => {
  const url = selected.value?.url.trim() ?? ''
  if (!url || url.includes('://') || !baseURL.value) return ''
  return baseURL.value.replace(/\/+$/, '') + (url.startsWith('/') ? url : '/' + url)
})

const canSend = computed(
  () =>
    !!selected.value &&
    !!selected.value.rester &&
    !!selected.value.user &&
    !!selected.value.url.trim() &&
    !outcome.value?.sending
)

async function loadResters(): Promise<void> {
  restersLoading.value = true
  restersError.value = ''
  try {
    const names = await window.api.vault.listResters()
    const loadedResters: Record<string, VaultRester> = {}
    for (const name of names) {
      loadedResters[name] = await window.api.vault.getRester(vaultSession.passphrase, name)
    }
    resters.value = loadedResters
  } catch (err) {
    restersError.value = describeError(err)
  } finally {
    restersLoading.value = false
  }
}

watch(
  () => vaultSession.unlocked,
  (unlocked) => {
    if (unlocked) loadResters()
  },
  { immediate: true }
)

onMounted(async () => {
  requests.value = await window.api.resterClient.load(props.project.id)
  selectedId.value = requests.value[0]?.id ?? null
  loaded.value = true
})

let saveTimer: ReturnType<typeof setTimeout> | null = null

function saveNow(): void {
  if (saveTimer) clearTimeout(saveTimer)
  saveTimer = null
  window.api.resterClient.save(props.project.id, plainCopy(requests.value))
}

watch(
  requests,
  () => {
    if (!loaded.value) return
    if (saveTimer) clearTimeout(saveTimer)
    saveTimer = setTimeout(saveNow, SAVE_MS)
  },
  { deep: true }
)

onBeforeUnmount(() => {
  if (saveTimer) saveNow()
})

function addRequest(): void {
  const current = selected.value
  const request: ResterClientRequest = {
    id: crypto.randomUUID(),
    name: '',
    rester: current?.rester ?? targets.value[0]?.rester ?? '',
    environment: current?.environment ?? targets.value[0]?.environment ?? '',
    user: current?.user ?? '',
    method: 'GET',
    url: '',
    headers: [],
    bodyKind: 'none',
    json: '',
    form: []
  }
  requests.value.push(request)
  selectedId.value = request.id
  if (!request.user) pickDefaultUser(request)
}

function removeRequest(id: string): void {
  const index = requests.value.findIndex((r) => r.id === id)
  if (index < 0) return
  requests.value.splice(index, 1)
  delete outcomes[id]
  if (selectedId.value === id) {
    selectedId.value = requests.value[Math.min(index, requests.value.length - 1)]?.id ?? null
  }
}

function renameRequest(id: string, name: string): void {
  const request = requests.value.find((r) => r.id === id)
  if (request) request.name = name
}

// Keeps the request's user when the new environment has it, else takes the first usable one.
function pickDefaultUser(request: ResterClientRequest): void {
  const rester = resters.value[request.rester]
  const users = rester ? apiUsers(rester, request.environment) : []
  if (!users.some((u) => u.Name === request.user)) request.user = users[0]?.Name ?? ''
}

function pickTarget(id: string): void {
  if (!selected.value) return
  const [rester, environment] = JSON.parse(id) as [string, string]
  selected.value.rester = rester
  selected.value.environment = environment
  pickDefaultUser(selected.value)
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
  const match = matchBaseURL(parseCurl(text).url, targets.value)
  if (match) {
    request.rester = match.target.rester
    request.environment = match.target.environment
    pickDefaultUser(request)
  }
  const parsed = parseCurl(text, tokenHeaderOf(request))
  request.method = parsed.method
  request.url = match ? match.path : parsed.url
  request.headers = parsed.headers
  request.bodyKind = parsed.bodyKind
  request.json = parsed.json
  request.form = parsed.form

  const notes: string[] = []
  if (parsed.dropped.length) {
    notes.push(`Left out ${parsed.dropped.join(', ')}, since the chosen user supplies auth.`)
  }
  if (match) {
    notes.push(`Matched ${match.target.rester} · ${match.target.environment}.`)
  } else if (parsed.url.includes('://')) {
    notes.push('The URL is under no rester environment, so it cannot be sent as it is.')
  }
  notes.push(...parsed.notes)
  outcomeOf(request.id).notice = notes.join(' ')
}

async function send(download: boolean): Promise<void> {
  const request = selected.value
  if (!request || !canSend.value) return
  const state = outcomeOf(request.id)
  state.sending = true
  state.error = ''
  state.notice = ''
  noteVaultActivity()
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
    <div v-if="!vaultSession.unlocked" class="vault-tab">
      <div class="vault-frame vault-frame--gate">
        <VaultUnlockGate />
      </div>
    </div>

    <div v-else class="client-frame">
      <RequestList
        :requests="requests"
        :selected-id="selectedId"
        @select="selectedId = $event"
        @add="addRequest"
        @remove="removeRequest"
        @rename="renameRequest"
      />

      <section v-if="selected && outcome" class="client-main">
        <div class="target-row">
          <div class="target-picker">
            <AppSwitcher
              framed
              :items="targetItems"
              :active-id="selectedTarget"
              :label="selected.rester ? undefined : 'Pick a rester'"
              placeholder="Pick a rester"
              :empty-text="
                restersLoading ? 'Loading resters...' : 'No rester has an API environment yet.'
              "
              @select="pickTarget"
            />
          </div>
          <div class="user-picker">
            <AppSwitcher
              framed
              :items="userItems"
              :active-id="selected.user || null"
              placeholder="Pick a user"
              empty-text="No user can call this environment."
              @select="selected.user = $event"
            />
          </div>
          <AppButton
            :disabled="!selected.user"
            title="Drop this user's cached token"
            @click="flush"
          >
            Flush token
          </AppButton>
        </div>
        <p v-if="restersError" class="error-text">{{ restersError }}</p>

        <div class="url-row">
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
          <input
            v-model="selected.url"
            class="url-input"
            type="text"
            spellcheck="false"
            placeholder="/path, a full URL, or paste a curl command"
            @paste="onUrlPaste"
            @keydown="onUrlKeydown"
          />
          <AppButton variant="primary" :disabled="!canSend" @click="send(false)">Send</AppButton>
          <AppButton :disabled="!canSend" @click="send(true)">Send and download</AppButton>
        </div>
        <p v-if="resolvedURL" class="hint resolved">{{ resolvedURL }}</p>
        <p v-if="outcome.notice" class="hint notice">{{ outcome.notice }}</p>

        <div class="panes">
          <div class="pane">
            <RequestEditor :model-value="selected" />
          </div>
          <div class="pane">
            <ResponsePanel
              :result="outcome.result"
              :download="outcome.download"
              :error="outcome.error"
              :sending="outcome.sending"
            />
          </div>
        </div>
      </section>

      <section v-else class="client-main client-empty">
        <p class="hint">
          Add a request to start. Paste a curl command into its URL box to fill it.
        </p>
        <AppButton variant="primary" @click="addRequest">+ Request</AppButton>
      </section>
    </div>
  </div>
</template>
