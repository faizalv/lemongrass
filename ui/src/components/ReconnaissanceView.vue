<template>
  <div :style="root">

    <!-- Header -->
    <div :style="header">
      <div style="flex:1;min-width:0">
        <div :style="breadcrumb">
          <AppIcon name="radar" :size="11" color="var(--color-amber)" :extra-style="{ flexShrink: 0 }" />
          <span>Reconnaissance</span>
          <span style="color:var(--color-gray-700)">·</span>
          <span>{{ project.shortPath }}</span>
          <span style="color:var(--color-gray-700)">·</span>
          <span>{{ project.branch }}</span>
        </div>
        <div :style="mainTitle">Semantic map</div>
      </div>
      <div :style="coverageRow">
        <!-- Sync status -->
        <div :style="syncStatus">
          <template v-if="syncing">
            <div class="spin" :style="spinnerSm" />
            <span style="color:var(--color-gray-500);font-size:11px;font-family:'DM Sans',sans-serif">Syncing filesystem…</span>
          </template>
          <template v-else-if="lastSyncedLabel">
            <span style="color:var(--color-gray-600);font-size:11px;font-family:'DM Sans',sans-serif">{{ lastSyncedLabel }}</span>
          </template>
          <button :style="refreshBtn" title="Re-sync" @click="activate">
            <svg width="11" height="11" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round">
              <polyline points="23 4 23 10 17 10"/><path d="M20.49 15a9 9 0 1 1-2.12-9.36L23 10"/>
            </svg>
          </button>
          <select :style="filterSelect" v-model="syncInterval" @change="saveSyncInterval">
            <option value="off">No auto-sync</option>
            <option value="5m">Every 5 min</option>
            <option value="15m">Every 15 min</option>
            <option value="30m">Every 30 min</option>
            <option value="1h">Every hour</option>
          </select>
        </div>

        <template v-if="loadingCoverage">
          <div v-for="i in 2" :key="i" :style="coverageSkeleton" />
        </template>
        <template v-else>
          <div v-for="cov in coverage" :key="cov.language" :style="{ ...coveragePill, opacity: syncing ? 0.4 : 1 }">
            <span :style="covLang">{{ cov.language }}</span>
            <span :style="covNumbers">
              <span :style="{ color: 'var(--color-success)', fontWeight: 600 }">{{ cov.explored }}</span>
              <template v-if="cov.stale > 0">
                <span style="color:var(--color-gray-600)"> · </span>
                <span style="color:#F59E0B;font-weight:600">{{ cov.stale }}</span>
              </template>
              <span style="color:var(--color-gray-600)"> · </span>
              <span style="color:var(--color-gray-500)">{{ cov.total - cov.explored - cov.stale }}</span>
              <span style="color:var(--color-gray-600)"> / {{ cov.total }}</span>
            </span>
          </div>
        </template>
      </div>
    </div>

    <!-- Info banner -->
    <div :style="infoBanner">
      <AppIcon name="info" :size="13" color="var(--color-info)" :extra-style="{ flexShrink: 0 }" />
      <span>
        Exploration happens inside <strong style="color:var(--color-gray-100);font-weight:600">Grooming</strong>. The model annotates symbols as a side effect of planning.
      </span>
    </div>

    <!-- .lgignore section -->
    <div :style="ignoreBar">
      <button :style="ignoreToggle" @click="ignoreOpen = !ignoreOpen">
        <AppIcon name="file" :size="11" color="var(--color-gray-500)" :extra-style="{ flexShrink: 0 }" />
        <span style="color:var(--color-gray-500);font-weight:700;letter-spacing:0.04em">.lgignore</span>
        <span v-if="!loadingIgnore" style="color:var(--color-gray-600)">
          {{ ignorePatterns.length === 0 ? 'no file' : ignorePatterns.length + ' pattern' + (ignorePatterns.length === 1 ? '' : 's') }}
        </span>
        <svg
          :style="{ transform: ignoreOpen ? 'rotate(90deg)' : 'rotate(0deg)', transition: 'transform 140ms ease', display: 'block', marginLeft: 'auto', flexShrink: 0 }"
          width="10" height="10" viewBox="0 0 24 24" fill="none"
          stroke="var(--color-gray-500)" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round"
        >
          <polyline points="9 18 15 12 9 6" />
        </svg>
      </button>
      <div v-if="ignoreOpen" :style="ignoreList">
        <div v-if="loadingIgnore" style="color:var(--color-gray-600);font-size:12px">Loading…</div>
        <div v-else-if="ignorePatterns.length === 0" style="color:var(--color-gray-600);font-size:12px;font-style:italic">No .lgignore file found — only defaults apply.</div>
        <div v-else v-for="p in ignorePatterns" :key="p" :style="ignorePattern">{{ p }}</div>
      </div>
    </div>

    <!-- Body: tree | symbols | detail -->
    <div style="flex:1;display:flex;overflow:hidden">

      <!-- File tree panel -->
      <div :style="{ ...treePanel, opacity: syncing ? 0.4 : 1, pointerEvents: syncing ? 'none' : 'auto' }">
        <div :style="treeSearchWrap">
          <input :style="treeSearchInput" v-model="treeFilter" placeholder="Filter files…" spellcheck="false" />
        </div>

        <div style="flex:1;overflow:auto;padding:8px">
          <div v-if="loadingNodes" :style="treeLoading">
            <div class="spin" :style="spinnerSm" />
          </div>
          <template v-else>
            <!-- Same treeWrap container style as AddProjectModal -->
            <div :style="treeWrap">
              <ReconFileNode
                v-for="node in filteredTree"
                :key="node.path"
                :node="node"
                :selected-file="selectedFile ?? ''"
                :force-open="!!treeFilter"
                :default-open="true"
                @select="onFileSelect"
              />
            </div>
          </template>
        </div>
      </div>

      <!-- Symbol list -->
      <div style="flex:1;display:flex;flex-direction:column;overflow:hidden">

        <!-- Symbol filter bar -->
        <div :style="symFilterBar">
          <span :style="symFileLabel">{{ selectedFile ? lastName(selectedFile) : 'Select a file' }}</span>
          <div style="flex:1" />
          <select :style="filterSelect" v-model="activeKind">
            <option value="">All kinds</option>
            <option v-for="k in kinds" :key="k" :value="k">{{ k }}</option>
          </select>
          <div :style="filterGroup">
            <button
              v-for="s in statusOptions"
              :key="s.value"
              :style="statusTab(s.value === activeStatus)"
              @click="activeStatus = s.value"
            >{{ s.label }}</button>
          </div>
        </div>

        <div style="flex:1;overflow:auto;padding-bottom:40px">
          <div v-if="!selectedFile" :style="centerState">
            <span :style="stateHint">Pick a file from the tree.</span>
          </div>
          <div v-else-if="fileSymbols.length === 0" :style="centerState">
            <span :style="stateHint">No symbols match the current filters.</span>
          </div>
          <button
            v-for="node in fileSymbols"
            :key="node.id"
            :style="nodeRow(node)"
            @click="selected = node"
            @mouseenter="hovered = node.id"
            @mouseleave="hovered = ''"
          >
            <span :style="nodeName">
              {{ node.symbol }}
              <span v-if="node.receiver" style="color:var(--color-gray-600);font-weight:400"> · {{ node.receiver }}</span>
            </span>
            <span :style="kindBadge(node.kind)">{{ node.kind }}</span>
            <span :style="nodeLines">:{{ node.line_start }}–{{ node.line_end }}</span>
          </button>
        </div>
      </div>

      <!-- Detail panel -->
      <div :style="detailPanel">
        <div v-if="!selected" :style="detailEmpty">Click a symbol to inspect.</div>
        <template v-else>
          <div :style="detailSymbol">{{ selected.symbol }}</div>
          <div v-if="selected.receiver" :style="detailReceiver">on {{ selected.receiver }}</div>

          <div style="display:flex;align-items:center;gap:8px;margin-bottom:16px;flex-wrap:wrap">
            <span :style="kindBadge(selected.kind)">{{ selected.kind }}</span>
            <span :style="statusPill(selected.status)">
              <span :style="statusDot(selected.status)" />{{ selected.status }}
            </span>
          </div>

          <div :style="detailMeta">
            <div :style="metaLabel">Package</div>
            <div :style="metaVal">{{ selected.package }}</div>
          </div>
          <div :style="detailMeta">
            <div :style="metaLabel">Location</div>
            <div :style="metaVal">{{ selected.file_path }}:{{ selected.line_start }}–{{ selected.line_end }}</div>
          </div>

          <template v-if="selected.status === 'explored' && selected.description">
            <div :style="{ ...metaLabel, marginTop: '20px', marginBottom: '8px' }">Description</div>
            <div :style="descriptionBlock">{{ selected.description }}</div>
          </template>
          <template v-else-if="selected.signature">
            <div :style="{ ...metaLabel, marginTop: '20px', marginBottom: '8px' }">Signature</div>
            <div :style="signatureBlock">{{ selected.symbol }}{{ selected.signature }}</div>
          </template>
          <div v-else :style="notAnnotated">Not yet annotated.</div>
        </template>
      </div>

    </div>
    <GitPanel :project-id="String(project.id)" />
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted, watch } from 'vue'
import type { Project, SemanticNode, LangCoverage, ReconTreeNode } from '../types'
import AppIcon from './AppIcon.vue'
import ReconFileNode from './ReconFileNode.vue'
import GitPanel from './GitPanel.vue'

const props = defineProps<{ project: Project }>()

// ── State ─────────────────────────────────────────────────────────────────────

const coverage        = ref<LangCoverage[]>([])
const allNodes        = ref<SemanticNode[]>([])
const selected        = ref<SemanticNode | null>(null)
const hovered         = ref('')
const loadingCoverage = ref(true)
const loadingNodes    = ref(true)
const loadingIgnore   = ref(true)
const ignorePatterns  = ref<string[]>([])
const ignoreOpen      = ref(false)
const treeFilter      = ref('')
const syncing         = ref(false)
const lastSyncedNano  = ref<number | null>(null)
const syncInterval    = ref('off')
let   syncPollTimer   = 0
const selectedFile    = ref<string | null>(null)
const activeKind      = ref('')
const activeStatus    = ref('')

const kinds = ['func','method','type','struct','interface','const','var','component','store','composable','plugin','class','hook','route']
const statusOptions = [
  { label: 'All',        value: '' },
  { label: 'Unexplored', value: 'unexplored' },
  { label: 'Stale',      value: 'stale' },
  { label: 'Explored',   value: 'explored' },
]

// ── Tree building ─────────────────────────────────────────────────────────────

interface MutableNode {
  name:        string
  path:        string
  isDir:       boolean
  childrenMap: Map<string, MutableNode>
  explored:    number
  stale:       number
  total:       number
}

function buildNestedTree(nodes: SemanticNode[]): ReconTreeNode[] {
  const fileStats = new Map<string, { explored: number; stale: number; total: number }>()
  for (const n of nodes) {
    const s = fileStats.get(n.file_path) ?? { explored: 0, stale: 0, total: 0 }
    s.total++
    if (n.status === 'explored') s.explored++
    if (n.status === 'stale')    s.stale++
    fileStats.set(n.file_path, s)
  }

  const rootMap = new Map<string, MutableNode>()
  for (const [filePath, stats] of fileStats) {
    const parts = filePath.split('/')
    let cur = rootMap
    for (let i = 0; i < parts.length; i++) {
      const part   = parts[i]
      const fp     = parts.slice(0, i + 1).join('/')
      const isLast = i === parts.length - 1
      if (!cur.has(part)) cur.set(part, { name: part, path: fp, isDir: !isLast, childrenMap: new Map(), explored: 0, stale: 0, total: 0 })
      const node = cur.get(part)!
      if (isLast) { node.explored = stats.explored; node.stale = stats.stale; node.total = stats.total }
      cur = node.childrenMap
    }
  }

  function toTree(map: Map<string, MutableNode>): ReconTreeNode[] {
    const result: ReconTreeNode[] = []
    for (const n of map.values()) {
      const children = toTree(n.childrenMap)
      const explored = n.isDir ? children.reduce((s, c) => s + c.explored, 0) : n.explored
      const stale    = n.isDir ? children.reduce((s, c) => s + c.stale,    0) : n.stale
      const total    = n.isDir ? children.reduce((s, c) => s + c.total,    0) : n.total
      result.push({ name: n.name, path: n.path, isDir: n.isDir, children, explored, stale, total })
    }
    return result.sort((a, b) => {
      if (a.isDir !== b.isDir) return a.isDir ? -1 : 1
      return a.name.localeCompare(b.name)
    })
  }

  return toTree(rootMap)
}

function filterTree(nodes: ReconTreeNode[], q: string): ReconTreeNode[] {
  return nodes.flatMap(node => {
    if (!node.isDir) return node.path.toLowerCase().includes(q) ? [node] : []
    const children = filterTree(node.children, q)
    return children.length > 0 || node.path.toLowerCase().includes(q)
      ? [{ ...node, children }]
      : []
  })
}

// ── Computed ──────────────────────────────────────────────────────────────────

const nestedTree = computed(() => buildNestedTree(allNodes.value))

const filteredTree = computed(() =>
  treeFilter.value ? filterTree(nestedTree.value, treeFilter.value.toLowerCase()) : nestedTree.value
)

const fileSymbols = computed(() => {
  if (!selectedFile.value) return []
  return allNodes.value
    .filter(n => n.file_path === selectedFile.value)
    .filter(n => !activeKind.value   || n.kind   === activeKind.value)
    .filter(n => !activeStatus.value || n.status === activeStatus.value)
    .sort((a, b) => a.line_start - b.line_start)
})

// ── Actions ───────────────────────────────────────────────────────────────────

function onFileSelect(path: string) {
  selectedFile.value = path
  selected.value = null
}

// ── Fetch ─────────────────────────────────────────────────────────────────────

const lastSyncedLabel = computed(() => {
  if (syncing.value) return ''
  if (!lastSyncedNano.value) return ''
  const ms = Date.now() - lastSyncedNano.value / 1e6
  if (ms < 10000) return 'Synced just now'
  if (ms < 60000) return `Synced ${Math.floor(ms / 1000)}s ago`
  if (ms < 3600000) return `Synced ${Math.floor(ms / 60000)}m ago`
  return 'Synced a while ago'
})

onMounted(() => { Promise.all([fetchCoverage(), fetchNodes(), fetchLgIgnore(), activate()]) })
onUnmounted(() => { clearInterval(syncPollTimer) })

watch(() => props.project.id, () => {
  selected.value = null; selectedFile.value = null; treeFilter.value = ''
  activeKind.value = ''; activeStatus.value = ''
  ignoreOpen.value = false; syncing.value = false; lastSyncedNano.value = null
  clearInterval(syncPollTimer)
  Promise.all([fetchCoverage(), fetchNodes(), fetchLgIgnore(), activate()])
})

async function fetchCoverage() {
  loadingCoverage.value = true
  try {
    const r = await fetch(`/api/recon/projects/${props.project.id}/coverage`)
    if (r.ok) coverage.value = await r.json()
  } catch { /* ignore */ }
  finally { loadingCoverage.value = false }
}

async function fetchNodes() {
  loadingNodes.value = true
  try {
    const r = await fetch(`/api/recon/projects/${props.project.id}/nodes`)
    if (r.ok) allNodes.value = await r.json()
  } catch { /* ignore */ }
  finally { loadingNodes.value = false }
}

async function activate() {
  try {
    await fetch(`/api/recon/projects/${props.project.id}/activate`, { method: 'POST' })
    syncing.value = true
    clearInterval(syncPollTimer)
    syncPollTimer = window.setInterval(pollSyncStatus, 1500)
  } catch { /* ignore */ }
}

async function pollSyncStatus() {
  try {
    const r = await fetch(`/api/recon/projects/${props.project.id}/sync-status`)
    if (!r.ok) return
    const data = await r.json()
    syncing.value = data.syncing
    syncInterval.value = data.sync_interval ?? 'off'
    if (data.last_synced) lastSyncedNano.value = data.last_synced
    if (!data.syncing) {
      clearInterval(syncPollTimer)
      await Promise.all([fetchCoverage(), fetchNodes()])
    }
  } catch { /* ignore */ }
}

async function fetchLgIgnore() {
  loadingIgnore.value = true
  try {
    const r = await fetch(`/api/recon/projects/${props.project.id}/lgignore`)
    if (r.ok) {
      const data = await r.json()
      ignorePatterns.value = data.patterns ?? []
    }
  } catch { /* ignore */ }
  finally { loadingIgnore.value = false }
}

async function saveSyncInterval() {
  try {
    await fetch(`/api/recon/projects/${props.project.id}/sync-interval`, {
      method: 'PATCH',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ sync_interval: syncInterval.value }),
    })
  } catch { /* ignore */ }
}

// ── Helpers ───────────────────────────────────────────────────────────────────

function lastName(path: string): string { return path.split('/').pop() ?? path }

// ── Kind badges ───────────────────────────────────────────────────────────────

const kindColors: Record<string, { bg: string; color: string }> = {
  func:       { bg: 'rgba(96,165,250,0.12)',  color: 'var(--color-info)' },
  method:     { bg: 'rgba(96,165,250,0.12)',  color: 'var(--color-info)' },
  type:       { bg: 'rgba(167,139,250,0.12)', color: 'var(--color-violet)' },
  struct:     { bg: 'rgba(167,139,250,0.12)', color: 'var(--color-violet)' },
  interface:  { bg: 'rgba(167,139,250,0.12)', color: 'var(--color-violet)' },
  const:      { bg: 'rgba(245,197,24,0.10)',  color: 'var(--color-amber)' },
  var:        { bg: 'rgba(245,197,24,0.10)',  color: 'var(--color-amber)' },
  component:  { bg: 'rgba(74,222,128,0.10)',  color: 'var(--color-success)' },
  store:      { bg: 'rgba(74,222,128,0.10)',  color: 'var(--color-success)' },
  composable: { bg: 'rgba(74,222,128,0.10)',  color: 'var(--color-success)' },
  plugin:     { bg: 'rgba(74,222,128,0.10)',  color: 'var(--color-success)' },
  class:      { bg: 'rgba(251,146,60,0.10)',  color: 'var(--color-coral)' },
  hook:       { bg: 'rgba(74,222,128,0.10)',  color: 'var(--color-success)' },
  route:      { bg: 'rgba(251,146,60,0.10)',  color: 'var(--color-coral)' },
}

function kindBadge(kind: string) {
  const c = kindColors[kind] ?? { bg: 'rgba(255,255,255,0.06)', color: 'var(--color-gray-300)' }
  return {
    display: 'inline-flex', alignItems: 'center',
    padding: '2px 7px', borderRadius: '4px',
    background: c.bg, color: c.color,
    fontSize: '10.5px', fontWeight: 700, letterSpacing: '0.04em',
    fontFamily: "'DM Sans',sans-serif", flexShrink: 0,
  }
}

// ── Status ────────────────────────────────────────────────────────────────────

const statusColors: Record<string, string> = { unexplored: 'var(--color-gray-500)', explored: 'var(--color-success)', stale: '#F59E0B', removed: 'var(--color-error)' }

function statusPill(status: string) {
  const color = statusColors[status] ?? 'var(--color-gray-500)'
  return {
    display: 'inline-flex', alignItems: 'center', gap: '5px',
    padding: '3px 9px', borderRadius: '999px',
    background: `${color}15`, color,
    fontSize: '10.5px', fontWeight: 700, letterSpacing: '0.05em',
    textTransform: 'uppercase' as const, fontFamily: "'DM Sans',sans-serif",
  }
}

function statusDot(status: string) {
  return { width: '5px', height: '5px', borderRadius: '50%', background: statusColors[status] ?? 'var(--color-gray-500)', display: 'inline-block' }
}

function statusTab(active: boolean) {
  return {
    padding: '4px 8px', borderRadius: '5px', border: 'none', cursor: 'pointer',
    background: active ? 'rgba(255,255,255,0.08)' : 'transparent',
    color: active ? 'var(--color-gray-100)' : 'var(--color-gray-500)',
    fontFamily: "'DM Sans',sans-serif", fontSize: '11px', fontWeight: 600,
    transition: 'all 100ms',
  }
}

// ── Symbol row ────────────────────────────────────────────────────────────────

function nodeRow(node: SemanticNode) {
  const isSel = selected.value?.id === node.id
  const isHov = hovered.value === node.id && !isSel
  return {
    display: 'flex', alignItems: 'center', gap: '10px',
    padding: '7px 20px 7px 18px',
    border: 'none', borderRadius: 0, cursor: 'pointer', width: '100%', textAlign: 'left' as const,
    borderLeft: `2px solid ${isSel ? 'var(--color-amber)' : node.status === 'explored' ? 'var(--color-success)' : node.status === 'stale' ? '#F59E0B' : 'transparent'}`,
    background: isSel ? 'rgba(245,197,24,0.06)' : isHov ? 'rgba(255,255,255,0.03)' : 'transparent',
    transition: 'background 80ms ease',
  }
}

// ── Static styles ─────────────────────────────────────────────────────────────

const root             = { flex: 1, display: 'flex', flexDirection: 'column' as const, overflow: 'hidden', background: 'var(--color-surface-0)' }
const header           = { padding: '22px 32px 18px', borderBottom: '1px solid rgba(255,255,255,0.06)', display: 'flex', alignItems: 'flex-end', gap: '24px', flexShrink: 0 }
const breadcrumb       = { display: 'flex', alignItems: 'center', gap: '8px', marginBottom: '6px', fontFamily: 'var(--font-mono)', fontSize: '11px', color: 'var(--color-gray-500)', whiteSpace: 'nowrap' as const, overflow: 'hidden' }
const mainTitle        = { fontFamily: 'var(--font-display)', fontSize: '28px', fontWeight: 700, color: 'var(--color-fg-primary)', letterSpacing: '-0.02em' }
const coverageRow      = { display: 'flex', gap: '8px', alignItems: 'center', flexShrink: 0 }
const syncStatus       = { display: 'flex', alignItems: 'center', gap: '6px' }
const refreshBtn       = { background: 'transparent', border: 'none', cursor: 'pointer', padding: '4px', color: 'var(--color-gray-600)', display: 'flex', alignItems: 'center', borderRadius: '4px' }
const coverageSkeleton = { width: '90px', height: '30px', borderRadius: '6px', background: 'rgba(255,255,255,0.04)' }
const coveragePill     = { display: 'inline-flex', alignItems: 'center', gap: '8px', padding: '5px 12px', borderRadius: '6px', background: 'rgba(255,255,255,0.04)', border: '1px solid rgba(255,255,255,0.07)' }
const covLang          = { fontFamily: 'var(--font-body)', fontSize: '11px', fontWeight: 700, color: 'var(--color-gray-300)', textTransform: 'uppercase' as const, letterSpacing: '0.06em' }
const covNumbers       = { fontFamily: 'var(--font-mono)', fontSize: '12px' }
const infoBanner       = { padding: '10px 32px', background: 'rgba(96,165,250,0.04)', borderBottom: '1px solid rgba(96,165,250,0.12)', display: 'flex', alignItems: 'center', gap: '10px', fontFamily: 'var(--font-body)', fontSize: '12px', color: 'var(--color-gray-300)', flexShrink: 0 }
// Tree panel
const treePanel        = { width: '260px', flexShrink: 0, borderRight: '1px solid rgba(255,255,255,0.06)', display: 'flex', flexDirection: 'column' as const, overflow: 'hidden', background: 'var(--color-surface-0)' }
const treeSearchWrap   = { padding: '10px 10px 6px', flexShrink: 0 }
const treeSearchInput  = { width: '100%', padding: '6px 10px', borderRadius: '6px', border: '1px solid rgba(255,255,255,0.08)', background: 'var(--color-surface-0)', color: 'var(--color-gray-300)', fontFamily: 'var(--font-body)', fontSize: '12px', outline: 'none', boxSizing: 'border-box' as const }
// treeWrap matches AddProjectModal exactly
const treeWrap         = { background: 'var(--color-surface-0)', border: '1px solid rgba(255,255,255,0.08)', borderRadius: '8px', padding: '8px' }
const treeLoading      = { display: 'flex', alignItems: 'center', justifyContent: 'center', height: '80px' }
const spinnerSm        = { width: '16px', height: '16px', borderRadius: '50%', border: '2px solid rgba(255,255,255,0.06)', borderTopColor: 'var(--color-amber)' }
// Symbol list
const symFilterBar     = { display: 'flex', alignItems: 'center', gap: '8px', padding: '6px 12px', borderBottom: '1px solid rgba(255,255,255,0.05)', flexShrink: 0 }
const symFileLabel     = { fontFamily: 'var(--font-mono)', fontSize: '12px', color: 'var(--color-gray-500)', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' as const, minWidth: 0 }
const filterGroup      = { display: 'flex', alignItems: 'center', gap: '2px', flexShrink: 0 }
const filterSelect     = { padding: '4px 8px', borderRadius: '5px', border: '1px solid rgba(255,255,255,0.08)', background: 'var(--color-gray-900)', color: 'var(--color-gray-300)', fontFamily: 'var(--font-body)', fontSize: '12px', cursor: 'pointer', outline: 'none', flexShrink: 0 }
const centerState      = { display: 'flex', alignItems: 'center', justifyContent: 'center', height: '200px' }
const stateHint        = { fontSize: '13px', fontFamily: 'var(--font-body)', color: 'var(--color-gray-600)' }
const nodeName         = { fontFamily: 'var(--font-mono)', fontSize: '13px', color: 'var(--color-gray-100)', fontWeight: 600, flexShrink: 0 }
const nodeLines        = { marginLeft: 'auto', fontFamily: 'var(--font-mono)', fontSize: '11px', color: 'var(--color-gray-600)', flexShrink: 0, whiteSpace: 'nowrap' as const }
// Detail panel
const detailPanel      = { width: '320px', flexShrink: 0, background: 'var(--color-surface-0)', overflow: 'auto', padding: '20px 22px 28px', fontFamily: 'var(--font-body)' }
const detailEmpty      = { display: 'flex', alignItems: 'center', justifyContent: 'center', height: '100%', fontSize: '12px', color: 'var(--color-gray-600)' }
const detailSymbol     = { fontFamily: 'var(--font-mono)', fontSize: '16px', fontWeight: 700, color: 'var(--color-gray-100)', marginBottom: '4px', wordBreak: 'break-all' as const }
const detailReceiver   = { fontFamily: 'var(--font-body)', fontSize: '11px', color: 'var(--color-gray-500)', marginBottom: '12px' }
const detailMeta       = { paddingBottom: '10px', marginBottom: '10px', borderBottom: '1px solid rgba(255,255,255,0.05)' }
const metaLabel        = { fontSize: '10px', fontWeight: 700, letterSpacing: '0.10em', textTransform: 'uppercase' as const, color: 'var(--color-gray-600)', marginBottom: '3px' }
const metaVal          = { fontFamily: 'var(--font-mono)', fontSize: '12px', color: 'var(--color-gray-300)', wordBreak: 'break-all' as const }
const descriptionBlock = { fontSize: '13px', color: 'var(--color-gray-100)', lineHeight: 1.6, fontFamily: 'var(--font-body)' }
const signatureBlock   = { fontFamily: 'var(--font-mono)', fontSize: '12px', color: 'var(--color-gray-300)', background: 'rgba(255,255,255,0.03)', border: '1px solid rgba(255,255,255,0.07)', borderRadius: '6px', padding: '10px 12px', lineHeight: 1.6, wordBreak: 'break-all' as const }
const notAnnotated     = { marginTop: '16px', fontSize: '12px', color: 'var(--color-gray-600)', fontStyle: 'italic' }
// .lgignore section
const ignoreBar        = { borderBottom: '1px solid rgba(255,255,255,0.05)', flexShrink: 0 }
const ignoreToggle     = { display: 'flex', alignItems: 'center', gap: '8px', padding: '7px 32px', background: 'transparent', border: 'none', cursor: 'pointer', width: '100%', textAlign: 'left' as const, fontFamily: 'var(--font-mono)', fontSize: '11px' }
const ignoreList       = { padding: '6px 32px 10px', display: 'flex', flexDirection: 'column' as const, gap: '3px' }
const ignorePattern    = { fontFamily: 'var(--font-mono)', fontSize: '11.5px', color: 'var(--color-gray-400)', padding: '2px 8px', borderRadius: '4px', background: 'rgba(255,255,255,0.03)' }
</script>
