<template>
  <div :style="root">

    <!-- Header -->
    <div :style="header">
      <div style="flex:1;min-width:0">
        <div :style="breadcrumb">
          <AppIcon name="radar" :size="11" color="#F5C518" :extra-style="{ flexShrink: 0 }" />
          <span>Reconnaissance</span>
          <span style="color:#2A2A2A">·</span>
          <span>{{ project.shortPath }}</span>
          <span style="color:#2A2A2A">·</span>
          <span>{{ project.branch }}</span>
        </div>
        <div :style="mainTitle">Semantic map</div>
      </div>
      <div :style="coverageRow">
        <template v-if="loadingCoverage">
          <div v-for="i in 2" :key="i" :style="coverageSkeleton" />
        </template>
        <template v-else>
          <div v-for="cov in coverage" :key="cov.language" :style="coveragePill">
            <span :style="covLang">{{ cov.language }}</span>
            <span :style="covNumbers">
              <span :style="{ color: cov.explored > 0 ? '#4ADE80' : '#555', fontWeight: 600 }">{{ cov.explored }}</span>
              <span style="color:#3A3A3A"> / </span>
              <span style="color:#717171">{{ cov.total }}</span>
            </span>
          </div>
        </template>
      </div>
    </div>

    <!-- Info banner -->
    <div :style="infoBanner">
      <AppIcon name="info" :size="13" color="#60A5FA" :extra-style="{ flexShrink: 0 }" />
      <span>
        Exploration happens inside <strong style="color:#E0E0E0;font-weight:600">Grooming</strong> — the model annotates symbols as a side effect of planning.
      </span>
    </div>

    <!-- .lgignore section -->
    <div :style="ignoreBar">
      <button :style="ignoreToggle" @click="ignoreOpen = !ignoreOpen">
        <AppIcon name="file" :size="11" color="#555" :extra-style="{ flexShrink: 0 }" />
        <span style="color:#555;font-weight:700;letter-spacing:0.04em">.lgignore</span>
        <span v-if="!loadingIgnore" style="color:#3A3A3A">
          {{ ignorePatterns.length === 0 ? 'no file' : ignorePatterns.length + ' pattern' + (ignorePatterns.length === 1 ? '' : 's') }}
        </span>
        <svg
          :style="{ transform: ignoreOpen ? 'rotate(90deg)' : 'rotate(0deg)', transition: 'transform 140ms ease', display: 'block', marginLeft: 'auto', flexShrink: 0 }"
          width="10" height="10" viewBox="0 0 24 24" fill="none"
          stroke="#555" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round"
        >
          <polyline points="9 18 15 12 9 6" />
        </svg>
      </button>
      <div v-if="ignoreOpen" :style="ignoreList">
        <div v-if="loadingIgnore" style="color:#3A3A3A;font-size:12px">Loading…</div>
        <div v-else-if="ignorePatterns.length === 0" style="color:#3A3A3A;font-size:12px;font-style:italic">No .lgignore file found — only defaults apply.</div>
        <div v-else v-for="p in ignorePatterns" :key="p" :style="ignorePattern">{{ p }}</div>
      </div>
    </div>

    <!-- Body: tree | symbols | detail -->
    <div style="flex:1;display:flex;overflow:hidden">

      <!-- File tree panel -->
      <div :style="treePanel">
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
              <span v-if="node.receiver" style="color:#3D3D3D;font-weight:400"> · {{ node.receiver }}</span>
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
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, watch } from 'vue'
import type { Project, SemanticNode, LangCoverage, ReconTreeNode } from '../types'
import AppIcon from './AppIcon.vue'
import ReconFileNode from './ReconFileNode.vue'

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
const selectedFile    = ref<string | null>(null)
const activeKind      = ref('')
const activeStatus    = ref('')

const kinds = ['func','method','type','struct','interface','const','var','component','store','composable','plugin','class','hook','route']
const statusOptions = [
  { label: 'All',        value: '' },
  { label: 'Unexplored', value: 'unexplored' },
  { label: 'Explored',   value: 'explored' },
]

// ── Tree building ─────────────────────────────────────────────────────────────

interface MutableNode {
  name:        string
  path:        string
  isDir:       boolean
  childrenMap: Map<string, MutableNode>
  explored:    number
  total:       number
}

function buildNestedTree(nodes: SemanticNode[]): ReconTreeNode[] {
  const fileStats = new Map<string, { explored: number; total: number }>()
  for (const n of nodes) {
    const s = fileStats.get(n.file_path) ?? { explored: 0, total: 0 }
    s.total++
    if (n.status === 'explored') s.explored++
    fileStats.set(n.file_path, s)
  }

  const rootMap = new Map<string, MutableNode>()
  for (const [filePath, stats] of fileStats) {
    const parts = filePath.split('/')
    let cur = rootMap
    for (let i = 0; i < parts.length; i++) {
      const part    = parts[i]
      const fp      = parts.slice(0, i + 1).join('/')
      const isLast  = i === parts.length - 1
      if (!cur.has(part)) cur.set(part, { name: part, path: fp, isDir: !isLast, childrenMap: new Map(), explored: 0, total: 0 })
      const node = cur.get(part)!
      if (isLast) { node.explored = stats.explored; node.total = stats.total }
      cur = node.childrenMap
    }
  }

  function toTree(map: Map<string, MutableNode>): ReconTreeNode[] {
    const result: ReconTreeNode[] = []
    for (const n of map.values()) {
      const children = toTree(n.childrenMap)
      const explored = n.isDir ? children.reduce((s, c) => s + c.explored, 0) : n.explored
      const total    = n.isDir ? children.reduce((s, c) => s + c.total,    0) : n.total
      result.push({ name: n.name, path: n.path, isDir: n.isDir, children, explored, total })
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

onMounted(() => { Promise.all([fetchCoverage(), fetchNodes(), fetchLgIgnore()]) })

watch(() => props.project.id, () => {
  selected.value = null; selectedFile.value = null; treeFilter.value = ''
  activeKind.value = ''; activeStatus.value = ''
  ignoreOpen.value = false
  Promise.all([fetchCoverage(), fetchNodes(), fetchLgIgnore()])
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

// ── Helpers ───────────────────────────────────────────────────────────────────

function lastName(path: string): string { return path.split('/').pop() ?? path }

// ── Kind badges ───────────────────────────────────────────────────────────────

const kindColors: Record<string, { bg: string; color: string }> = {
  func:       { bg: 'rgba(96,165,250,0.12)',  color: '#60A5FA' },
  method:     { bg: 'rgba(96,165,250,0.12)',  color: '#60A5FA' },
  type:       { bg: 'rgba(167,139,250,0.12)', color: '#A78BFA' },
  struct:     { bg: 'rgba(167,139,250,0.12)', color: '#A78BFA' },
  interface:  { bg: 'rgba(167,139,250,0.12)', color: '#A78BFA' },
  const:      { bg: 'rgba(245,197,24,0.10)',  color: '#F5C518' },
  var:        { bg: 'rgba(245,197,24,0.10)',  color: '#F5C518' },
  component:  { bg: 'rgba(74,222,128,0.10)',  color: '#4ADE80' },
  store:      { bg: 'rgba(74,222,128,0.10)',  color: '#4ADE80' },
  composable: { bg: 'rgba(74,222,128,0.10)',  color: '#4ADE80' },
  plugin:     { bg: 'rgba(74,222,128,0.10)',  color: '#4ADE80' },
  class:      { bg: 'rgba(251,146,60,0.10)',  color: '#FB923C' },
  hook:       { bg: 'rgba(74,222,128,0.10)',  color: '#4ADE80' },
  route:      { bg: 'rgba(251,146,60,0.10)',  color: '#FB923C' },
}

function kindBadge(kind: string) {
  const c = kindColors[kind] ?? { bg: 'rgba(255,255,255,0.06)', color: '#9A9A9A' }
  return {
    display: 'inline-flex', alignItems: 'center',
    padding: '2px 7px', borderRadius: '4px',
    background: c.bg, color: c.color,
    fontSize: '10.5px', fontWeight: 700, letterSpacing: '0.04em',
    fontFamily: "'DM Sans',sans-serif", flexShrink: 0,
  }
}

// ── Status ────────────────────────────────────────────────────────────────────

const statusColors: Record<string, string> = { unexplored: '#555', explored: '#4ADE80', removed: '#F87171' }

function statusPill(status: string) {
  const color = statusColors[status] ?? '#555'
  return {
    display: 'inline-flex', alignItems: 'center', gap: '5px',
    padding: '3px 9px', borderRadius: '999px',
    background: `${color}15`, color,
    fontSize: '10.5px', fontWeight: 700, letterSpacing: '0.05em',
    textTransform: 'uppercase' as const, fontFamily: "'DM Sans',sans-serif",
  }
}

function statusDot(status: string) {
  return { width: '5px', height: '5px', borderRadius: '50%', background: statusColors[status] ?? '#555', display: 'inline-block' }
}

function statusTab(active: boolean) {
  return {
    padding: '4px 8px', borderRadius: '5px', border: 'none', cursor: 'pointer',
    background: active ? 'rgba(255,255,255,0.08)' : 'transparent',
    color: active ? '#D4D4D4' : '#555',
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
    borderLeft: `2px solid ${isSel ? '#F5C518' : node.status === 'explored' ? '#4ADE80' : 'transparent'}`,
    background: isSel ? 'rgba(245,197,24,0.06)' : isHov ? 'rgba(255,255,255,0.03)' : 'transparent',
    transition: 'background 80ms ease',
  }
}

// ── Static styles ─────────────────────────────────────────────────────────────

const root             = { flex: 1, display: 'flex', flexDirection: 'column' as const, overflow: 'hidden', background: '#0A0A0A' }
const header           = { padding: '22px 32px 18px', borderBottom: '1px solid rgba(255,255,255,0.06)', display: 'flex', alignItems: 'flex-end', gap: '24px', flexShrink: 0 }
const breadcrumb       = { display: 'flex', alignItems: 'center', gap: '8px', marginBottom: '6px', fontFamily: "'JetBrains Mono','Courier Prime',monospace", fontSize: '11px', color: '#555', whiteSpace: 'nowrap' as const, overflow: 'hidden' }
const mainTitle        = { fontFamily: "'Comfortaa',sans-serif", fontSize: '28px', fontWeight: 700, color: '#fff', letterSpacing: '-0.02em' }
const coverageRow      = { display: 'flex', gap: '8px', alignItems: 'center', flexShrink: 0 }
const coverageSkeleton = { width: '90px', height: '30px', borderRadius: '6px', background: 'rgba(255,255,255,0.04)' }
const coveragePill     = { display: 'inline-flex', alignItems: 'center', gap: '8px', padding: '5px 12px', borderRadius: '6px', background: 'rgba(255,255,255,0.04)', border: '1px solid rgba(255,255,255,0.07)' }
const covLang          = { fontFamily: "'DM Sans',sans-serif", fontSize: '11px', fontWeight: 700, color: '#9A9A9A', textTransform: 'uppercase' as const, letterSpacing: '0.06em' }
const covNumbers       = { fontFamily: "'JetBrains Mono','Courier Prime',monospace", fontSize: '12px' }
const infoBanner       = { padding: '10px 32px', background: 'rgba(96,165,250,0.04)', borderBottom: '1px solid rgba(96,165,250,0.12)', display: 'flex', alignItems: 'center', gap: '10px', fontFamily: "'DM Sans',sans-serif", fontSize: '12px', color: '#9A9A9A', flexShrink: 0 }
// Tree panel
const treePanel        = { width: '260px', flexShrink: 0, borderRight: '1px solid rgba(255,255,255,0.06)', display: 'flex', flexDirection: 'column' as const, overflow: 'hidden', background: '#0D0D0D' }
const treeSearchWrap   = { padding: '10px 10px 6px', flexShrink: 0 }
const treeSearchInput  = { width: '100%', padding: '6px 10px', borderRadius: '6px', border: '1px solid rgba(255,255,255,0.08)', background: '#0A0A0A', color: '#9A9A9A', fontFamily: "'DM Sans',sans-serif", fontSize: '12px', outline: 'none', boxSizing: 'border-box' as const }
// treeWrap matches AddProjectModal exactly
const treeWrap         = { background: '#0A0A0A', border: '1px solid rgba(255,255,255,0.08)', borderRadius: '8px', padding: '8px' }
const treeLoading      = { display: 'flex', alignItems: 'center', justifyContent: 'center', height: '80px' }
const spinnerSm        = { width: '16px', height: '16px', borderRadius: '50%', border: '2px solid rgba(255,255,255,0.06)', borderTopColor: '#F5C518' }
// Symbol list
const symFilterBar     = { display: 'flex', alignItems: 'center', gap: '8px', padding: '6px 12px', borderBottom: '1px solid rgba(255,255,255,0.05)', flexShrink: 0 }
const symFileLabel     = { fontFamily: "'JetBrains Mono',monospace", fontSize: '12px', color: '#555', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' as const, minWidth: 0 }
const filterGroup      = { display: 'flex', alignItems: 'center', gap: '2px', flexShrink: 0 }
const filterSelect     = { padding: '4px 8px', borderRadius: '5px', border: '1px solid rgba(255,255,255,0.08)', background: '#111', color: '#9A9A9A', fontFamily: "'DM Sans',sans-serif", fontSize: '12px', cursor: 'pointer', outline: 'none', flexShrink: 0 }
const centerState      = { display: 'flex', alignItems: 'center', justifyContent: 'center', height: '200px' }
const stateHint        = { fontSize: '13px', fontFamily: "'DM Sans',sans-serif", color: '#3D3D3D' }
const nodeName         = { fontFamily: "'JetBrains Mono','Courier Prime',monospace", fontSize: '13px', color: '#D4D4D4', fontWeight: 600, flexShrink: 0 }
const nodeLines        = { marginLeft: 'auto', fontFamily: "'JetBrains Mono',monospace", fontSize: '11px', color: '#3A3A3A', flexShrink: 0, whiteSpace: 'nowrap' as const }
// Detail panel
const detailPanel      = { width: '320px', flexShrink: 0, background: '#0C0C0C', overflow: 'auto', padding: '20px 22px 28px', fontFamily: "'DM Sans',sans-serif" }
const detailEmpty      = { display: 'flex', alignItems: 'center', justifyContent: 'center', height: '100%', fontSize: '12px', color: '#3D3D3D' }
const detailSymbol     = { fontFamily: "'JetBrains Mono','Courier Prime',monospace", fontSize: '16px', fontWeight: 700, color: '#E0E0E0', marginBottom: '4px', wordBreak: 'break-all' as const }
const detailReceiver   = { fontFamily: "'DM Sans',sans-serif", fontSize: '11px', color: '#555', marginBottom: '12px' }
const detailMeta       = { paddingBottom: '10px', marginBottom: '10px', borderBottom: '1px solid rgba(255,255,255,0.05)' }
const metaLabel        = { fontSize: '10px', fontWeight: 700, letterSpacing: '0.10em', textTransform: 'uppercase' as const, color: '#3D3D3D', marginBottom: '3px' }
const metaVal          = { fontFamily: "'JetBrains Mono','Courier Prime',monospace", fontSize: '12px', color: '#9A9A9A', wordBreak: 'break-all' as const }
const descriptionBlock = { fontSize: '13px', color: '#C0C0C0', lineHeight: 1.6, fontFamily: "'DM Sans',sans-serif" }
const signatureBlock   = { fontFamily: "'JetBrains Mono','Courier Prime',monospace", fontSize: '12px', color: '#9A9A9A', background: 'rgba(255,255,255,0.03)', border: '1px solid rgba(255,255,255,0.07)', borderRadius: '6px', padding: '10px 12px', lineHeight: 1.6, wordBreak: 'break-all' as const }
const notAnnotated     = { marginTop: '16px', fontSize: '12px', color: '#3D3D3D', fontStyle: 'italic' }
// .lgignore section
const ignoreBar        = { borderBottom: '1px solid rgba(255,255,255,0.05)', flexShrink: 0 }
const ignoreToggle     = { display: 'flex', alignItems: 'center', gap: '8px', padding: '7px 32px', background: 'transparent', border: 'none', cursor: 'pointer', width: '100%', textAlign: 'left' as const, fontFamily: "'JetBrains Mono','Courier Prime',monospace", fontSize: '11px' }
const ignoreList       = { padding: '6px 32px 10px', display: 'flex', flexDirection: 'column' as const, gap: '3px' }
const ignorePattern    = { fontFamily: "'JetBrains Mono','Courier Prime',monospace", fontSize: '11.5px', color: '#717171', padding: '2px 8px', borderRadius: '4px', background: 'rgba(255,255,255,0.03)' }
</script>
