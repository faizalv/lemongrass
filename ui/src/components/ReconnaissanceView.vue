<template>
  <div style="flex:1;display:flex;flex-direction:column;overflow:hidden;background:#0A0A0A">

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

      <!-- Coverage pills -->
      <div :style="coverageRow">
        <template v-if="loadingCoverage">
          <div v-for="i in 2" :key="i" :style="coverageSkeleton" />
        </template>
        <template v-else>
          <div
            v-for="cov in coverage"
            :key="cov.language"
            :style="coveragePill(cov)"
          >
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
        Exploration happens inside <strong style="color:#E0E0E0;font-weight:600">Grooming</strong> — the model annotates symbols as a side effect of planning. Nothing to interact with here.
      </span>
    </div>

    <!-- Filter bar -->
    <div :style="filterBar">
      <!-- Language tabs -->
      <div :style="filterGroup">
        <button
          v-for="lang in ['all', ...coverage.map(c => c.language)]"
          :key="lang"
          :style="langTab(lang === activeLanguage)"
          @click="setLanguage(lang)"
        >{{ lang === 'all' ? 'All' : lang }}</button>
      </div>

      <div style="flex:1" />

      <!-- Kind select -->
      <select :style="filterSelect" v-model="activeKind" @change="fetchNodes()">
        <option value="">All kinds</option>
        <option v-for="k in kinds" :key="k" :value="k">{{ k }}</option>
      </select>

      <!-- Status toggle -->
      <div :style="filterGroup">
        <button
          v-for="s in statusOptions"
          :key="s.value"
          :style="statusTab(s.value === activeStatus)"
          @click="setStatus(s.value)"
        >{{ s.label }}</button>
      </div>
    </div>

    <!-- Body: list + detail -->
    <div style="flex:1;display:flex;overflow:hidden">

      <!-- Node list -->
      <div style="flex:1;overflow:auto;padding-bottom:40px">

        <!-- Loading -->
        <div v-if="loadingNodes" :style="centerState">
          <div class="spin" :style="spinner" />
          <span :style="stateHint">Loading symbols…</span>
        </div>

        <!-- Empty -->
        <div v-else-if="nodes.length === 0" :style="centerState">
          <span :style="stateHint">No symbols match the current filters.</span>
        </div>

        <!-- Rows -->
        <template v-else>
          <button
            v-for="node in nodes"
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
            <span :style="nodePath">
              {{ shortPath(node.file_path) }}<span style="color:#3A3A3A">:{{ node.line_start }}–{{ node.line_end }}</span>
            </span>
          </button>
        </template>
      </div>

      <!-- Detail panel -->
      <div :style="detailPanel">
        <div v-if="!selected" :style="detailEmpty">
          Click a symbol to inspect.
        </div>
        <template v-else>
          <div :style="detailSymbol">{{ selected.symbol }}</div>
          <div v-if="selected.receiver" :style="detailReceiver">on {{ selected.receiver }}</div>

          <div style="display:flex;align-items:center;gap:8px;margin-bottom:16px;flex-wrap:wrap">
            <span :style="kindBadge(selected.kind)">{{ selected.kind }}</span>
            <span :style="statusPill(selected.status)">
              <span :style="statusDot(selected.status)" />
              {{ selected.status }}
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

          <!-- Description (explored) -->
          <template v-if="selected.status === 'explored' && selected.description">
            <div :style="{ ...metaLabel, marginTop: '20px', marginBottom: '8px' }">Description</div>
            <div :style="descriptionBlock">{{ selected.description }}</div>
          </template>

          <!-- Signature (unexplored) -->
          <template v-else-if="selected.signature">
            <div :style="{ ...metaLabel, marginTop: '20px', marginBottom: '8px' }">Signature</div>
            <div :style="signatureBlock">{{ selected.symbol }}{{ selected.signature }}</div>
          </template>

          <!-- Neither -->
          <div v-else :style="notAnnotated">Not yet annotated.</div>
        </template>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, watch } from 'vue'
import type { Project, SemanticNode, LangCoverage } from '../types'
import AppIcon from './AppIcon.vue'

const props = defineProps<{ project: Project }>()

const coverage     = ref<LangCoverage[]>([])
const nodes        = ref<SemanticNode[]>([])
const selected     = ref<SemanticNode | null>(null)
const hovered      = ref('')
const loadingCoverage = ref(true)
const loadingNodes = ref(true)
const activeLanguage = ref('all')
const activeKind   = ref('')
const activeStatus = ref('')

const kinds = ['func','method','type','struct','interface','const','var','component','store','composable','plugin','class','hook','route']
const statusOptions = [
  { label: 'All',        value: '' },
  { label: 'Unexplored', value: 'unexplored' },
  { label: 'Explored',   value: 'explored' },
]

onMounted(async () => {
  await Promise.all([fetchCoverage(), fetchNodes()])
})

watch(() => props.project.id, async () => {
  selected.value = null
  activeLanguage.value = 'all'
  activeKind.value = ''
  activeStatus.value = ''
  await Promise.all([fetchCoverage(), fetchNodes()])
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
  selected.value = null
  try {
    const params = new URLSearchParams()
    if (activeLanguage.value && activeLanguage.value !== 'all') params.set('language', activeLanguage.value)
    if (activeKind.value)   params.set('kind',     activeKind.value)
    if (activeStatus.value) params.set('status',   activeStatus.value)
    const r = await fetch(`/api/recon/projects/${props.project.id}/nodes?${params}`)
    if (r.ok) nodes.value = await r.json()
  } catch { /* ignore */ }
  finally { loadingNodes.value = false }
}

function setLanguage(lang: string) {
  activeLanguage.value = lang
  fetchNodes()
}

function setStatus(s: string) {
  activeStatus.value = s
  fetchNodes()
}

function shortPath(p: string): string {
  const parts = p.split('/')
  if (parts.length <= 3) return p
  return '…/' + parts.slice(-2).join('/')
}

// ── Kind badge colours ───────────────────────────────────────────────────────

const kindColors: Record<string, { bg: string; color: string }> = {
  func:        { bg: 'rgba(96,165,250,0.12)',  color: '#60A5FA' },
  method:      { bg: 'rgba(96,165,250,0.12)',  color: '#60A5FA' },
  type:        { bg: 'rgba(167,139,250,0.12)', color: '#A78BFA' },
  struct:      { bg: 'rgba(167,139,250,0.12)', color: '#A78BFA' },
  interface:   { bg: 'rgba(167,139,250,0.12)', color: '#A78BFA' },
  const:       { bg: 'rgba(245,197,24,0.10)',  color: '#F5C518' },
  var:         { bg: 'rgba(245,197,24,0.10)',  color: '#F5C518' },
  component:   { bg: 'rgba(74,222,128,0.10)',  color: '#4ADE80' },
  store:       { bg: 'rgba(74,222,128,0.10)',  color: '#4ADE80' },
  composable:  { bg: 'rgba(74,222,128,0.10)',  color: '#4ADE80' },
  plugin:      { bg: 'rgba(74,222,128,0.10)',  color: '#4ADE80' },
  class:       { bg: 'rgba(251,146,60,0.10)',  color: '#FB923C' },
  hook:        { bg: 'rgba(74,222,128,0.10)',  color: '#4ADE80' },
  route:       { bg: 'rgba(251,146,60,0.10)',  color: '#FB923C' },
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

// ── Status ───────────────────────────────────────────────────────────────────

const statusColors: Record<string, string> = {
  unexplored: '#555',
  explored:   '#4ADE80',
  removed:    '#F87171',
}

function statusPill(status: string) {
  const color = statusColors[status] ?? '#555'
  return {
    display: 'inline-flex', alignItems: 'center', gap: '5px',
    padding: '3px 9px', borderRadius: '999px',
    background: `${color}15`, color,
    fontSize: '10.5px', fontWeight: 700, letterSpacing: '0.05em',
    textTransform: 'uppercase', fontFamily: "'DM Sans',sans-serif",
  }
}

function statusDot(status: string) {
  return {
    width: '5px', height: '5px', borderRadius: '50%',
    background: statusColors[status] ?? '#555', display: 'inline-block',
  }
}

// ── Row ──────────────────────────────────────────────────────────────────────

function nodeRow(node: SemanticNode) {
  const isSelected = selected.value?.id === node.id
  const isHovered  = hovered.value === node.id && !isSelected
  const leftColor  = node.status === 'explored' ? '#4ADE80' : 'transparent'
  return {
    display: 'flex', alignItems: 'center', gap: '10px',
    padding: '7px 20px 7px 18px',
    border: 'none', borderRadius: 0, cursor: 'pointer', width: '100%', textAlign: 'left',
    borderLeft: `2px solid ${isSelected ? '#F5C518' : leftColor}`,
    background: isSelected ? 'rgba(245,197,24,0.06)' : isHovered ? 'rgba(255,255,255,0.03)' : 'transparent',
    transition: 'background 80ms ease',
  }
}

// ── Coverage pill ─────────────────────────────────────────────────────────────

function coveragePill(cov: LangCoverage) {
  return {
    display: 'inline-flex', alignItems: 'center', gap: '8px',
    padding: '5px 12px', borderRadius: '6px',
    background: 'rgba(255,255,255,0.04)',
    border: '1px solid rgba(255,255,255,0.07)',
  }
}

// ── Tab helpers ───────────────────────────────────────────────────────────────

function langTab(active: boolean) {
  return {
    padding: '4px 10px', borderRadius: '5px', border: 'none', cursor: 'pointer',
    background: active ? 'rgba(245,197,24,0.12)' : 'transparent',
    color: active ? '#F5C518' : '#555',
    fontFamily: "'DM Sans',sans-serif", fontSize: '12px', fontWeight: 600,
    transition: 'all 100ms',
  }
}

function statusTab(active: boolean) {
  return {
    padding: '4px 10px', borderRadius: '5px', border: 'none', cursor: 'pointer',
    background: active ? 'rgba(255,255,255,0.08)' : 'transparent',
    color: active ? '#D4D4D4' : '#555',
    fontFamily: "'DM Sans',sans-serif", fontSize: '12px', fontWeight: 600,
    transition: 'all 100ms',
  }
}

// ── Static styles ─────────────────────────────────────────────────────────────

const header       = { padding: '22px 32px 18px', borderBottom: '1px solid rgba(255,255,255,0.06)', display: 'flex', alignItems: 'flex-end', gap: '24px', flexShrink: 0 }
const breadcrumb   = { display: 'flex', alignItems: 'center', gap: '8px', marginBottom: '6px', fontFamily: "'JetBrains Mono','Courier Prime',monospace", fontSize: '11px', color: '#555', whiteSpace: 'nowrap', overflow: 'hidden' }
const mainTitle    = { fontFamily: "'Comfortaa',sans-serif", fontSize: '28px', fontWeight: 700, color: '#fff', letterSpacing: '-0.02em' }
const coverageRow  = { display: 'flex', gap: '8px', alignItems: 'center', flexShrink: 0 }
const coverageSkeleton = { width: '90px', height: '30px', borderRadius: '6px', background: 'rgba(255,255,255,0.04)' }
const covLang      = { fontFamily: "'DM Sans',sans-serif", fontSize: '11px', fontWeight: 700, color: '#9A9A9A', textTransform: 'uppercase', letterSpacing: '0.06em' }
const covNumbers   = { fontFamily: "'JetBrains Mono','Courier Prime',monospace", fontSize: '12px' }
const infoBanner   = { padding: '10px 32px', background: 'rgba(96,165,250,0.04)', borderBottom: '1px solid rgba(96,165,250,0.12)', display: 'flex', alignItems: 'center', gap: '10px', fontFamily: "'DM Sans',sans-serif", fontSize: '12px', color: '#9A9A9A', flexShrink: 0 }
const filterBar    = { display: 'flex', alignItems: 'center', gap: '8px', padding: '8px 20px', borderBottom: '1px solid rgba(255,255,255,0.05)', flexShrink: 0, overflowX: 'auto' }
const filterGroup  = { display: 'flex', alignItems: 'center', gap: '2px', flexShrink: 0 }
const filterSelect = { padding: '4px 8px', borderRadius: '5px', border: '1px solid rgba(255,255,255,0.08)', background: '#111', color: '#9A9A9A', fontFamily: "'DM Sans',sans-serif", fontSize: '12px', cursor: 'pointer', outline: 'none' }
const centerState  = { display: 'flex', flexDirection: 'column', alignItems: 'center', justifyContent: 'center', height: '200px', gap: '12px' }
const stateHint    = { fontSize: '13px', fontFamily: "'DM Sans',sans-serif", color: '#3D3D3D' }
const spinner      = { width: '24px', height: '24px', borderRadius: '50%', border: '2px solid rgba(255,255,255,0.06)', borderTopColor: '#F5C518' }
const nodeName     = { fontFamily: "'JetBrains Mono','Courier Prime',monospace", fontSize: '13px', color: '#D4D4D4', fontWeight: 600, flexShrink: 0, minWidth: 0 }
const nodePath     = { marginLeft: 'auto', fontFamily: "'JetBrains Mono','Courier Prime',monospace", fontSize: '11px', color: '#3D3D3D', flexShrink: 0, whiteSpace: 'nowrap' }
const detailPanel  = { width: '340px', flexShrink: 0, borderLeft: '1px solid rgba(255,255,255,0.06)', background: '#0C0C0C', overflow: 'auto', padding: '20px 22px 28px', fontFamily: "'DM Sans',sans-serif" }
const detailEmpty  = { display: 'flex', alignItems: 'center', justifyContent: 'center', height: '100%', fontSize: '12px', color: '#3D3D3D' }
const detailSymbol = { fontFamily: "'JetBrains Mono','Courier Prime',monospace", fontSize: '16px', fontWeight: 700, color: '#E0E0E0', marginBottom: '4px', wordBreak: 'break-all' }
const detailReceiver = { fontFamily: "'DM Sans',sans-serif", fontSize: '11px', color: '#555', marginBottom: '12px' }
const detailMeta   = { paddingBottom: '10px', marginBottom: '10px', borderBottom: '1px solid rgba(255,255,255,0.05)' }
const metaLabel    = { fontSize: '10px', fontWeight: 700, letterSpacing: '0.10em', textTransform: 'uppercase', color: '#3D3D3D', marginBottom: '3px' }
const metaVal      = { fontFamily: "'JetBrains Mono','Courier Prime',monospace", fontSize: '12px', color: '#9A9A9A', wordBreak: 'break-all' }
const descriptionBlock = { fontSize: '13px', color: '#C0C0C0', lineHeight: 1.6, fontFamily: "'DM Sans',sans-serif" }
const signatureBlock   = { fontFamily: "'JetBrains Mono','Courier Prime',monospace", fontSize: '12px', color: '#9A9A9A', background: 'rgba(255,255,255,0.03)', border: '1px solid rgba(255,255,255,0.07)', borderRadius: '6px', padding: '10px 12px', lineHeight: 1.6, wordBreak: 'break-all' }
const notAnnotated = { marginTop: '16px', fontSize: '12px', color: '#3D3D3D', fontStyle: 'italic' }
</script>
