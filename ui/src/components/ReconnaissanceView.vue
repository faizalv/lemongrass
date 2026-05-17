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
      <div :style="statsRow">
        <span :style="statItem">
          <span style="width:7px;height:7px;border-radius:50%;background:#4ADE80;display:inline-block"></span>
          <span style="color:#D4D4D4;font-weight:600">{{ indexedFiles }}</span>
          <span style="color:#555">/ {{ totalFiles }} files</span>
        </span>
        <span v-if="staleCount > 0" :style="{ ...statItem, color: '#F5C518' }">
          <span style="width:7px;height:7px;border-radius:50%;background:#F5C518;display:inline-block"></span>
          <span style="font-weight:600">{{ staleCount }}</span>
          <span style="color:#555">stale</span>
        </span>
        <span v-if="missingCount > 0" :style="{ ...statItem, color: '#717171' }">
          <span style="width:7px;height:7px;border-radius:50%;background:#5A5A5A;display:inline-block"></span>
          <span style="font-weight:600">{{ missingCount }}</span>
          <span style="color:#555">unexplored</span>
        </span>
      </div>
    </div>

    <!-- Info banner -->
    <div :style="infoBanner">
      <AppIcon name="info" :size="13" color="#60A5FA" :extra-style="{ flexShrink: 0 }" />
      <span>
        Recon runs through <strong style="color:#E0E0E0;font-weight:600">Grooming</strong> — when a workspace needs a module that's stale or unexplored, you'll be asked to approve. Nothing happens here directly.
      </span>
    </div>

    <!-- Body: tree + detail -->
    <div style="flex:1;display:flex;overflow:hidden">
      <!-- Tree -->
      <div style="flex:1;overflow:auto;padding:14px 0 40px;font-family:'JetBrains Mono','Courier Prime',monospace">
        <!-- Root row -->
        <button
          :style="rowBg('', rootRow)"
          @click="selected = ''"
          @mouseenter="hovered = ''"
          @mouseleave="hovered = null"
        >
          <AppIcon name="git-branch" :size="12" color="#F5C518" :extra-style="{ flexShrink: 0 }" />
          <span :style="{ color: statusColors[rootStatus], fontWeight: 700, whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis', minWidth: 0 }">{{ project.shortPath }}/</span>
          <span style="color:#3D3D3D;font-size:11px;margin-left:8px;flex-shrink:0">{{ project.branch }}</span>
        </button>

        <!-- Modules -->
        <template v-for="(m, i) in modules" :key="m.path">
          <button
            :style="rowBg(m.path, moduleRow(m))"
            @click="selected = m.path"
            @mouseenter="hovered = m.path"
            @mouseleave="hovered = null"
          >
            <span style="color:#2F2F2F;white-space:pre;flex-shrink:0">{{ treePrefix(i) }}</span>
            <span :style="{ color: m.status === 'missing' ? '#5A5A5A' : statusColors[m.status], fontWeight: 600 }">{{ m.path }}</span>
            <span :style="moduleMeta">
              <span style="color:#555">{{ m.files }} files</span>
              <template v-if="m.loc !== '—'"> · {{ typeof m.loc === 'number' ? m.loc.toLocaleString() : m.loc }} LoC</template>
              <span> · {{ m.updated }}</span>
              <span v-if="m.note" style="color:#F5C518;font-style:italic"> · {{ m.note }}</span>
            </span>
            <span :style="statusTag(m.status)">
              <span :style="{ width: '6px', height: '6px', borderRadius: '50%', background: statusColors[m.status], boxShadow: m.status === 'fresh' ? `0 0 0 3px ${statusColors[m.status]}1A` : 'none', display: 'inline-block' }"></span>
              {{ statusLabels[m.status] }}
            </span>
          </button>
        </template>

        <!-- Legend -->
        <div :style="legend">
          <span style="font-weight:700;letter-spacing:0.08em;text-transform:uppercase">Legend</span>
          <LegendItem color="#4ADE80" label="indexed (MD5 matches)" />
          <LegendItem color="#F5C518" label="stale (file hash drifted)" />
          <LegendItem color="#5A5A5A" label="not explored yet" />
        </div>
      </div>

      <!-- Detail panel -->
      <div :style="detailPanel">
        <div v-if="!selectedModule" style="display:flex;align-items:center;justify-content:center;height:100%;font-size:12px;color:#3D3D3D;font-family:'DM Sans',sans-serif">
          Click a path to inspect.
        </div>
        <template v-else>
          <div :style="detailBreadcrumb">{{ selectedModule.path }}</div>
          <div :style="statusPill(selectedModule.status)">
            <span :style="{ width: '5px', height: '5px', borderRadius: '50%', background: statusColors[selectedModule.status], display: 'inline-block' }"></span>
            {{ statusLabels[selectedModule.status] }}
          </div>
          <div :style="statsGrid">
            <DetailStat label="Files" :value="String(selectedModule.files)" />
            <DetailStat label="LoC" :value="typeof selectedModule.loc === 'number' ? selectedModule.loc.toLocaleString() : selectedModule.loc" />
            <DetailStat label="Functions" :value="String(selectedModule.funcs)" />
            <DetailStat label="Routes" :value="String(selectedModule.routes)" />
          </div>
          <div :style="detailRow">
            <span style="color:#717171">Last indexed</span>
            <span :style="detailVal">{{ selectedModule.updated }}</span>
          </div>
          <div :style="detailRow">
            <span style="color:#717171">Hash</span>
            <span :style="detailVal">{{ selectedModule.hash }}</span>
          </div>
          <div v-if="selectedModule.reason" :style="staleReason">
            <span style="color:#F5C518;font-weight:600">Why stale:</span> {{ selectedModule.reason }}
          </div>
          <div v-if="selectedModule.status === 'missing'" :style="missingNote">
            Lemongrass hasn't indexed this module yet. The next grooming session that touches it will ask for permission to scan.
          </div>
        </template>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, defineComponent, h } from 'vue'
import type { Project } from '../types'
import AppIcon from './AppIcon.vue'

defineProps<{ project: Project }>()

interface ReconModule {
  path: string; files: number; loc: number | string; hash: string
  status: 'fresh' | 'stale' | 'missing'; updated: string
  funcs: number | string; routes: number | string
  note?: string; reason?: string
}

const modules: ReconModule[] = [
  { path: 'cmd/server/',          files: 4,  loc: 412,   hash: 'a91…', status: 'fresh',   updated: '2h ago',     funcs: 18, routes: 0  },
  { path: 'internal/auth/',       files: 11, loc: 1342,  hash: 'fc2…', status: 'fresh',   updated: 'today',      funcs: 62, routes: 4  },
  { path: 'internal/middleware/', files: 8,  loc: 706,   hash: 'd44…', status: 'stale',   updated: '3 days ago', funcs: 22, routes: 0, reason: '2 files changed since last index' },
  { path: 'internal/handlers/',   files: 22, loc: 3104,  hash: '9b1…', status: 'fresh',   updated: 'yesterday',  funcs: 88, routes: 31, note: '1 file modified — re-index queued' },
  { path: 'internal/storage/',    files: 14, loc: 2018,  hash: 'b06…', status: 'fresh',   updated: '4 days ago', funcs: 71, routes: 0  },
  { path: 'internal/transport/',  files: 14, loc: '—',   hash: '—',    status: 'missing', updated: 'never',      funcs: '—', routes: '—' },
  { path: 'pkg/redis/',           files: 3,  loc: 196,   hash: 'e88…', status: 'fresh',   updated: '5 days ago', funcs: 14, routes: 0  },
  { path: 'pkg/log/',             files: 2,  loc: 88,    hash: '11a…', status: 'fresh',   updated: '1 week ago', funcs: 8,  routes: 0  },
]

const statusColors: Record<string, string> = { fresh: '#4ADE80', stale: '#F5C518', missing: '#5A5A5A' }
const statusLabels: Record<string, string> = { fresh: 'indexed', stale: 'stale', missing: 'not indexed' }

const selected = ref('internal/middleware/')
const hovered = ref<string | null>(null)

function rowBg(path: string, base: Record<string, any>) {
  const isSelected = selected.value === path
  const isHovered = hovered.value === path && !isSelected
  return {
    ...base,
    background: isSelected ? 'rgba(245,197,24,0.07)' : isHovered ? 'rgba(255,255,255,0.04)' : 'transparent',
    borderLeft: isSelected ? '2px solid #F5C518' : '2px solid transparent',
  }
}

const selectedModule = computed(() => modules.find(m => m.path === selected.value) ?? null)
const rootStatus = computed(() => modules.some(m => m.status === 'missing') ? 'stale' : 'fresh')
const totalFiles = computed(() => modules.reduce((a, m) => a + m.files, 0))
const indexedFiles = computed(() => modules.filter(m => m.status !== 'missing').reduce((a, m) => a + m.files, 0))
const staleCount = computed(() => modules.filter(m => m.status === 'stale').length)
const missingCount = computed(() => modules.filter(m => m.status === 'missing').length)

function treePrefix(i: number) {
  return i === modules.length - 1 ? '└─ ' : '├─ '
}

function moduleRow(_m: ReconModule) {
  return {
    display: 'flex', alignItems: 'center', gap: '0',
    padding: '4px 14px',
    border: 'none', borderRadius: '0', cursor: 'pointer',
    width: '100%', textAlign: 'left',
    fontFamily: "'JetBrains Mono','Courier Prime',monospace", fontSize: '13px', lineHeight: 1.6,
    transition: 'background 80ms ease',
  }
}

function statusPill(status: string) {
  const color = statusColors[status]
  return {
    display: 'inline-flex', alignItems: 'center', gap: '6px',
    padding: '4px 10px', borderRadius: '999px',
    background: status === 'missing' ? 'rgba(90,90,90,0.10)' : `${color}15`,
    color, marginBottom: '14px',
    fontSize: '10.5px', fontWeight: 700, letterSpacing: '0.06em', textTransform: 'uppercase',
    whiteSpace: 'nowrap',
  }
}

function statusTag(status: string) {
  return {
    marginLeft: 'auto', display: 'inline-flex', alignItems: 'center', gap: '5px',
    color: statusColors[status], fontSize: '11px',
    fontFamily: "'DM Sans',sans-serif", fontWeight: 600, whiteSpace: 'nowrap',
  }
}

const header = { padding: '22px 32px 18px', borderBottom: '1px solid rgba(255,255,255,0.06)', display: 'flex', alignItems: 'flex-end', gap: '24px' }
const breadcrumb = { display: 'flex', alignItems: 'center', gap: '8px', marginBottom: '6px', fontFamily: "'JetBrains Mono','Courier Prime',monospace", fontSize: '11px', color: '#555', whiteSpace: 'nowrap', overflow: 'hidden' }
const mainTitle = { fontFamily: "'Comfortaa', sans-serif", fontSize: '28px', fontWeight: 700, color: '#fff', letterSpacing: '-0.02em' }
const statsRow = { display: 'flex', gap: '18px', alignItems: 'center', fontFamily: "'JetBrains Mono','Courier Prime',monospace", fontSize: '12px', color: '#717171' }
const statItem = { display: 'inline-flex', alignItems: 'center', gap: '6px' }
const infoBanner = { padding: '10px 32px', background: 'rgba(96,165,250,0.04)', borderBottom: '1px solid rgba(96,165,250,0.12)', display: 'flex', alignItems: 'center', gap: '10px', fontFamily: "'DM Sans',sans-serif", fontSize: '12px', color: '#9A9A9A', flexShrink: 0 }
const rootRow = { display: 'flex', alignItems: 'center', gap: '8px', padding: '6px 14px', width: '100%', textAlign: 'left', border: 'none', cursor: 'pointer', fontFamily: "'JetBrains Mono','Courier Prime',monospace", fontSize: '13px', lineHeight: 1.6, whiteSpace: 'nowrap', overflow: 'hidden' }
const moduleMeta = { marginLeft: '14px', color: '#3D3D3D', fontSize: '11.5px', display: 'inline-flex', alignItems: 'center', gap: '10px', whiteSpace: 'nowrap' }
const legend = { margin: '28px 32px 0', display: 'flex', alignItems: 'center', gap: '18px', fontFamily: "'DM Sans',sans-serif", fontSize: '11px', color: '#555' }
const detailPanel = { width: '340px', flexShrink: 0, borderLeft: '1px solid rgba(255,255,255,0.06)', background: '#0C0C0C', overflow: 'auto', padding: '20px 22px 28px', fontFamily: "'DM Sans',sans-serif" }
const detailBreadcrumb = { fontFamily: "'JetBrains Mono','Courier Prime',monospace", fontSize: '11px', color: '#555', marginBottom: '12px' }
const statsGrid = { display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '12px', marginBottom: '20px' }
const detailRow = { display: 'flex', alignItems: 'center', justifyContent: 'space-between', padding: '8px 0', borderTop: '1px solid rgba(255,255,255,0.05)', fontSize: '12px' }
const detailVal = { color: '#D4D4D4', fontFamily: "'JetBrains Mono','Courier Prime',monospace" }
const staleReason = { marginTop: '14px', padding: '10px 12px', background: 'rgba(245,197,24,0.06)', border: '1px solid rgba(245,197,24,0.20)', borderRadius: '6px', color: '#E0E0E0', fontSize: '12px', lineHeight: 1.5 }
const missingNote = { marginTop: '14px', padding: '12px 14px', background: 'rgba(255,255,255,0.02)', border: '1px dashed rgba(255,255,255,0.10)', borderRadius: '6px', color: '#9A9A9A', fontSize: '12px', lineHeight: 1.55 }

const DetailStat = defineComponent({
  props: ['label', 'value'],
  setup(props) {
    return () => h('div', {}, [
      h('div', { style: { fontSize: '10px', fontWeight: 700, letterSpacing: '0.10em', textTransform: 'uppercase', color: '#555', marginBottom: '3px' } }, props.label),
      h('div', { style: { fontFamily: "'JetBrains Mono','Courier Prime',monospace", fontSize: '18px', fontWeight: 700, color: '#E0E0E0' } }, props.value),
    ])
  },
})

const LegendItem = defineComponent({
  props: ['color', 'label'],
  setup(props) {
    return () => h('span', { style: { display: 'inline-flex', alignItems: 'center', gap: '6px' } }, [
      h('span', { style: { width: '8px', height: '8px', borderRadius: '50%', background: props.color, boxShadow: props.color === '#4ADE80' ? `0 0 0 3px ${props.color}1A` : 'none', display: 'inline-block' } }),
      h('span', { style: { color: '#9A9A9A' } }, props.label),
    ])
  },
})
</script>
