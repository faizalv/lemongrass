<template>
  <div style="flex:1;display:flex;flex-direction:column;overflow:hidden">

    <!-- Top bar -->
    <div :style="topBar">
      <div style="padding:22px 32px 0">
        <div :style="breadcrumb">
          <AppIcon name="archive" :size="11" color="var(--color-amber)" :extra-style="{ flexShrink: 0 }" />
          <span>Artifacts</span>
        </div>
        <div :style="mainTitle">{{ tabs.find(t => t.id === activeTab)?.label }}</div>
      </div>
      <div style="display:flex;gap:4px;padding:0 28px;margin-top:12px">
        <button v-for="tab in tabs" :key="tab.id" :style="tabBtn(tab.id === activeTab)" @click="activeTab = tab.id">
          {{ tab.label }}
          <span v-if="tab.id === activeTab" :style="tabUnderline" />
        </button>
      </div>
    </div>

    <!-- Archive tab: master-detail -->
    <div v-if="activeTab === 'archive'" style="flex:1;display:flex;overflow:hidden">
      <!-- Sidebar -->
      <div :style="sidebar">
        <div v-if="wsLoading" :style="sideEmpty">Loading…</div>
        <div v-else-if="workspaces.length === 0" :style="sideEmpty">No workspaces yet.</div>
        <button
          v-else
          v-for="ws in workspaces"
          :key="ws.id"
          :style="sideRow(ws.id === selectedWsId)"
          @click="selectWs(ws.id)"
        >
          <div style="flex:1;min-width:0">
            <div :style="sideTitle">{{ ws.name }}</div>
            <div :style="sideMeta">{{ formatDate(ws.created_at) }}</div>
          </div>
          <span :style="statusPip(ws.status)" />
        </button>
      </div>

      <!-- Detail panel -->
      <div :style="detail">
        <div v-if="!selectedWsId" :style="detailEmpty">Select a workspace to view its history.</div>
        <div v-else-if="detailLoading" :style="detailEmpty">Loading…</div>
        <template v-else-if="selectedWs">
          <!-- Status + name -->
          <div style="display:flex;align-items:center;gap:10px;margin-bottom:24px">
            <span :style="wsNameLarge">{{ selectedWs.name }}</span>
            <span :style="statusBadge(selectedWs.status)">{{ selectedWs.status }}</span>
            <span :style="detailMeta" style="margin-left:auto">{{ formatDate(selectedWs.created_at) }}</span>
          </div>

          <!-- Requirements -->
          <div :style="sectionLabel">REQUIREMENTS</div>
          <div v-if="selectedWs.requirements.length === 0" :style="emptyInline">No requirements.</div>
          <div v-else style="display:flex;flex-direction:column;gap:6px;margin-bottom:20px">
            <div v-for="r in selectedWs.requirements" :key="r.id" :style="reqCard">
              <AppIcon
                :name="r.type === 'text' ? 'file-text' : r.type === 'image' ? 'eye' : 'file'"
                :size="13"
                :extra-style="{ color: 'var(--color-gray-500)', flexShrink: 0 }"
              />
              <div style="flex:1;min-width:0">
                <div v-if="r.type === 'text'" style="display:flex;flex-direction:column;gap:4px">
                  <button :style="reqToggleBtn" @click="toggleReq(r.id)">
                    <span :style="reqPreview">{{ expandedReqs.has(r.id) ? '' : (r.text_content ?? '').slice(0, 120) + ((r.text_content?.length ?? 0) > 120 ? '…' : '') }}</span>
                    <span :style="reqAction">{{ expandedReqs.has(r.id) ? 'Collapse' : 'Expand' }}</span>
                  </button>
                  <div v-if="expandedReqs.has(r.id)" :style="reqFullText">{{ r.text_content }}</div>
                </div>
                <div v-else style="display:flex;align-items:center;justify-content:space-between;gap:8px">
                  <span :style="reqPreview">{{ r.file_name }}</span>
                  <a :href="`/api/workspaces/${selectedWs.id}/requirements/${r.id}/file`" target="_blank" :style="reqAction">Open</a>
                </div>
                <div v-if="r.type === 'image'" style="margin-top:6px">
                  <img :src="`/api/workspaces/${selectedWs.id}/requirements/${r.id}/file`" :style="reqThumb" />
                </div>
              </div>
            </div>
          </div>

          <!-- Approved tasks -->
          <template v-if="wsData[selectedWsId]?.tasks?.length">
            <div :style="sectionLabel">APPROVED TASKS</div>
            <div style="display:flex;flex-direction:column;gap:4px;margin-bottom:20px">
              <div v-for="(t, i) in wsData[selectedWsId].tasks" :key="t.id" :style="taskRow">
                <span :style="taskIdx">{{ i + 1 }}</span>
                <span :style="taskTitle">{{ t.title }}</span>
              </div>
            </div>
          </template>

          <!-- Execution diff -->
          <template v-if="wsData[selectedWsId]?.diff?.length">
            <div :style="sectionLabel">EXECUTION DIFF</div>
            <div style="display:flex;flex-direction:column;gap:6px">
              <div v-for="f in wsData[selectedWsId].diff" :key="f.file_path" :style="diffCard">
                <button :style="diffHeader" @click="toggleDiffFile(selectedWsId + f.file_path)">
                  <span :style="diffPath">{{ f.file_path }}</span>
                  <span v-if="f.is_new" :style="newBadge">NEW</span>
                  <span :style="diffStats"><span style="color:#4ade80">+{{ f.lines_added }}</span> <span style="color:var(--color-gray-600)">/</span> <span style="color:#f87171">-{{ f.lines_removed }}</span></span>
                  <AppIcon :name="expandedDiffFiles.has(selectedWsId + f.file_path) ? 'chevron-down' : 'chevrons-up-down'" :size="11" :extra-style="{ color: 'var(--color-gray-600)' }" />
                </button>
                <div v-if="expandedDiffFiles.has(selectedWsId + f.file_path)" :style="diffBody">
                  <div v-for="(line, li) in f.diff.split('\n')" :key="li" :style="diffLine(line)">{{ line }}</div>
                </div>
              </div>
            </div>
          </template>
        </template>
      </div>
    </div>

    <!-- Knowledge tab: 3-column grid -->
    <div v-else-if="activeTab === 'knowledge'" :style="gridWrap">
      <div v-if="knowledgeLoading" :style="tabEmpty">Loading…</div>
      <div v-else-if="knowledge.length === 0" :style="tabEmpty">No knowledge saved yet.</div>
      <div v-else :style="grid">
        <div v-for="k in knowledge" :key="k.key" :style="gridCard">
          <div style="display:flex;align-items:center;justify-content:space-between;gap:8px;margin-bottom:8px">
            <span :style="knowledgeKey">{{ k.key }}</span>
            <span :style="detailMeta">{{ formatDate(k.updated_at) }}</span>
          </div>
          <pre :style="knowledgeContent">{{ k.content }}</pre>
        </div>
      </div>
    </div>

    <!-- Outputs tab: 3-column grid -->
    <div v-else-if="activeTab === 'outputs'" :style="gridWrap">
      <div v-if="artifactsLoading" :style="tabEmpty">Loading…</div>
      <div v-else-if="artifacts.length === 0" :style="tabEmpty">No outputs yet.</div>
      <div v-else :style="grid">
        <div v-for="a in artifacts" :key="a.id" :style="gridCard">
          <div style="display:flex;align-items:center;gap:8px;margin-bottom:8px;flex-wrap:wrap">
            <span :style="artifactName">{{ a.name }}</span>
            <span :style="artifactType">{{ a.type }}</span>
            <span :style="versionBadge">v{{ a.version }}</span>
            <span :style="detailMeta" style="margin-left:auto">{{ formatDate(a.created_at) }}</span>
          </div>
          <template v-if="a.type === 'type-definition'">
            <div :style="codeWrap">
              <pre :style="codeBlock">{{ a.content }}</pre>
              <button :style="copyBtn" @click="copyContent(a.id, a.content)">{{ copiedId === a.id ? 'Copied' : 'Copy' }}</button>
            </div>
          </template>
          <template v-else>
            <div :style="genericContent">{{ a.content }}</div>
          </template>
        </div>
      </div>
    </div>

  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, watch } from 'vue'
import type { WorkspaceWithRequirements, ProjectArtifact, ApiTask, KnowledgeEntry } from '../types'
import AppIcon from './AppIcon.vue'

const props = defineProps<{ projectId: string }>()

interface FileDiff { file_path: string; diff: string; is_new: boolean; lines_added: number; lines_removed: number }
interface WsData { tasks: ApiTask[]; diff: FileDiff[] }

const tabs = [
  { id: 'archive',   label: 'Archive' },
  { id: 'knowledge', label: 'Knowledge' },
  { id: 'outputs',   label: 'Outputs' },
]

const activeTab     = ref('archive')
const selectedWsId  = ref('')
const workspaces    = ref<WorkspaceWithRequirements[]>([])
const artifacts     = ref<ProjectArtifact[]>([])
const knowledge     = ref<KnowledgeEntry[]>([])
const wsData        = ref<Record<string, WsData>>({})
const wsLoading     = ref(true)
const artifactsLoading = ref(true)
const knowledgeLoading = ref(true)
const detailLoading = ref(false)
const expandedReqs  = ref(new Set<string>())
const expandedDiffFiles = ref(new Set<string>())
const copiedId      = ref('')

const selectedWs = computed(() => workspaces.value.find(w => w.id === selectedWsId.value) ?? null)

onMounted(() => { load() })
watch(() => props.projectId, () => { reset(); load() })

function reset() {
  workspaces.value = []
  artifacts.value = []
  knowledge.value = []
  wsData.value = {}
  selectedWsId.value = ''
  expandedReqs.value = new Set()
  expandedDiffFiles.value = new Set()
  wsLoading.value = true
  artifactsLoading.value = true
  knowledgeLoading.value = true
}

async function load() {
  const [wsRes, artRes, knRes] = await Promise.allSettled([
    fetch(`/api/workspaces?project_id=${props.projectId}&include_deleted=true`),
    fetch(`/api/fs/projects/${props.projectId}/artifacts`),
    fetch(`/api/recon/projects/${props.projectId}/knowledge`),
  ])
  if (wsRes.status === 'fulfilled' && wsRes.value.ok) {
    workspaces.value = await wsRes.value.json()
    if (workspaces.value.length > 0) selectWs(workspaces.value[0].id)
  }
  wsLoading.value = false
  if (artRes.status === 'fulfilled' && artRes.value.ok) artifacts.value = await artRes.value.json()
  artifactsLoading.value = false
  if (knRes.status === 'fulfilled' && knRes.value.ok) knowledge.value = await knRes.value.json()
  knowledgeLoading.value = false
}

async function selectWs(id: string) {
  selectedWsId.value = id
  if (wsData.value[id]) return
  detailLoading.value = true
  const [tasksRes, diffRes] = await Promise.allSettled([
    fetch(`/api/workspaces/${id}/tasks`),
    fetch(`/api/lg/execution-diff?session=${id}`),
  ])
  const tasks: ApiTask[] = tasksRes.status === 'fulfilled' && tasksRes.value.ok
    ? (await tasksRes.value.json()).filter((t: ApiTask) => t.status === 'approved')
    : []
  const diffData = diffRes.status === 'fulfilled' && diffRes.value.ok ? await diffRes.value.json() : null
  wsData.value = { ...wsData.value, [id]: { tasks, diff: diffData?.files ?? [] } }
  detailLoading.value = false
}

function toggleReq(id: string) {
  const s = new Set(expandedReqs.value)
  s.has(id) ? s.delete(id) : s.add(id)
  expandedReqs.value = s
}

function toggleDiffFile(key: string) {
  const s = new Set(expandedDiffFiles.value)
  s.has(key) ? s.delete(key) : s.add(key)
  expandedDiffFiles.value = s
}

function formatDate(iso: string): string {
  return new Date(iso).toLocaleDateString('en-US', { year: 'numeric', month: 'short', day: 'numeric' })
}

async function copyContent(id: string, content: string) {
  await navigator.clipboard.writeText(content).catch(() => {})
  copiedId.value = id
  setTimeout(() => { copiedId.value = '' }, 2000)
}

function diffLine(line: string) {
  let bg = 'transparent', color = 'var(--color-gray-500)'
  if (line.startsWith('+') && !line.startsWith('+++')) { bg = 'rgba(74,222,128,0.08)'; color = '#4ade80' }
  else if (line.startsWith('-') && !line.startsWith('---')) { bg = 'rgba(248,113,113,0.08)'; color = '#f87171' }
  else if (line.startsWith('@@')) color = 'var(--color-info)'
  return { display: 'block', padding: '0 12px', fontFamily: 'var(--font-mono)', fontSize: '11px', lineHeight: 1.6, whiteSpace: 'pre' as const, background: bg, color }
}

const statusColors: Record<string, string> = {
  idle: 'rgba(255,255,255,0.10)', grooming: 'rgba(96,165,250,0.20)',
  awaiting_execution: 'rgba(245,197,24,0.20)', executing: 'rgba(74,222,128,0.20)',
  done: 'rgba(74,222,128,0.12)', deleted: 'rgba(255,255,255,0.05)', amending: 'rgba(167,139,250,0.20)',
}
const statusTextColors: Record<string, string> = {
  idle: 'var(--color-gray-300)', grooming: 'var(--color-info)',
  awaiting_execution: 'var(--color-amber)', executing: 'var(--color-success)',
  done: 'var(--color-success)', deleted: 'var(--color-gray-600)', amending: '#a78bfa',
}
const statusPipColors: Record<string, string> = {
  idle: 'var(--color-gray-600)', grooming: 'var(--color-info)',
  awaiting_execution: 'var(--color-amber)', executing: 'var(--color-success)',
  done: 'var(--color-success)', deleted: 'var(--color-gray-700)', amending: '#a78bfa',
}

function statusBadge(status: string) {
  return { display: 'inline-flex', alignItems: 'center', padding: '2px 8px', borderRadius: '999px', background: statusColors[status] ?? 'rgba(255,255,255,0.06)', color: statusTextColors[status] ?? 'var(--color-gray-500)', fontSize: '10px', fontWeight: 700, letterSpacing: '0.06em', textTransform: 'uppercase' as const, fontFamily: 'var(--font-body)' }
}
function statusPip(status: string) {
  return { width: '6px', height: '6px', borderRadius: '50%', background: statusPipColors[status] ?? 'var(--color-gray-600)', flexShrink: 0 }
}

const topBar        = { borderBottom: '1px solid rgba(255,255,255,0.06)', background: 'var(--color-surface-0)', flexShrink: 0 }
const breadcrumb    = { display: 'flex', alignItems: 'center', gap: '8px', marginBottom: '6px', fontFamily: 'var(--font-mono)', fontSize: '11px', color: 'var(--color-gray-500)', whiteSpace: 'nowrap' as const }
const mainTitle     = { fontFamily: 'var(--font-display)', fontSize: '28px', fontWeight: 700, color: 'var(--color-fg-primary)', letterSpacing: '-0.02em' }
const tabBtn        = (active: boolean) => ({ position: 'relative', display: 'inline-flex', alignItems: 'center', gap: '7px', padding: '10px 14px 12px', background: 'transparent', border: 'none', cursor: 'pointer', color: active ? 'var(--color-amber)' : 'var(--color-gray-300)', fontFamily: 'var(--font-body)', fontSize: '13.5px', fontWeight: active ? 600 : 500 } as Record<string, any>)
const tabUnderline  = { position: 'absolute', left: '8px', right: '8px', bottom: '-1px', height: '2px', background: 'var(--color-amber)', borderRadius: '2px' } as Record<string, any>

const sidebar       = { width: '260px', flexShrink: 0, borderRight: '1px solid rgba(255,255,255,0.06)', overflowY: 'auto' as const, display: 'flex', flexDirection: 'column' as const }
const sideRow       = (active: boolean) => ({ width: '100%', display: 'flex', alignItems: 'center', gap: '10px', padding: '12px 16px', background: active ? 'rgba(255,255,255,0.05)' : 'transparent', border: 'none', borderLeft: active ? '2px solid var(--color-amber)' : '2px solid transparent', cursor: 'pointer', textAlign: 'left' as const } as Record<string, any>)
const sideTitle     = { fontFamily: 'var(--font-body)', fontSize: '13px', fontWeight: 500, color: 'var(--color-fg-primary)', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' as const }
const sideMeta      = { fontFamily: 'var(--font-mono)', fontSize: '10px', color: 'var(--color-gray-600)', marginTop: '2px' }
const sideEmpty     = { padding: '20px 16px', fontSize: '12px', color: 'var(--color-gray-600)', fontFamily: 'var(--font-body)' }

const detail        = { flex: 1, overflowY: 'auto' as const, padding: '28px 32px' }
const detailEmpty   = { fontSize: '13px', color: 'var(--color-gray-600)', fontFamily: 'var(--font-body)' }
const detailMeta    = { fontSize: '11px', color: 'var(--color-gray-600)', fontFamily: 'var(--font-mono)', flexShrink: 0 }
const wsNameLarge   = { fontFamily: 'var(--font-display)', fontSize: '16px', fontWeight: 600, color: 'var(--color-fg-primary)', letterSpacing: '-0.01em' }
const sectionLabel  = { fontSize: '10px', fontWeight: 700, letterSpacing: '0.12em', color: 'var(--color-gray-600)', fontFamily: 'var(--font-body)', textTransform: 'uppercase' as const, marginBottom: '8px' }
const emptyInline   = { fontSize: '12px', color: 'var(--color-gray-700)', fontFamily: 'var(--font-body)', paddingBottom: '16px' }

const reqCard       = { background: 'var(--color-surface-0)', border: '1px solid rgba(255,255,255,0.05)', borderRadius: '6px', padding: '8px 12px', display: 'flex', alignItems: 'flex-start', gap: '8px' }
const reqToggleBtn  = { width: '100%', background: 'transparent', border: 'none', padding: 0, cursor: 'pointer', textAlign: 'left' as const, display: 'flex', alignItems: 'center', justifyContent: 'space-between', gap: '8px' }
const reqPreview    = { flex: 1, minWidth: 0, fontSize: '12px', color: 'var(--color-gray-400)', fontFamily: 'var(--font-body)', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' as const }
const reqAction     = { fontSize: '11px', color: 'var(--color-amber)', fontFamily: 'var(--font-body)', fontWeight: 600, flexShrink: 0, textDecoration: 'none', cursor: 'pointer', background: 'transparent', border: 'none' }
const reqFullText   = { fontSize: '12px', color: 'var(--color-gray-300)', fontFamily: 'var(--font-body)', lineHeight: 1.6, whiteSpace: 'pre-wrap' as const, marginTop: '6px' }
const reqThumb      = { maxWidth: '200px', maxHeight: '120px', borderRadius: '4px', objectFit: 'cover' as const, border: '1px solid rgba(255,255,255,0.06)' }

const taskRow   = { display: 'flex', alignItems: 'flex-start', gap: '10px', padding: '4px 0' }
const taskIdx   = { width: '16px', height: '16px', borderRadius: '4px', background: 'rgba(255,255,255,0.06)', display: 'inline-flex', alignItems: 'center', justifyContent: 'center', fontFamily: 'var(--font-mono)', fontSize: '10px', color: 'var(--color-gray-500)', flexShrink: 0, marginTop: '1px' }
const taskTitle = { fontSize: '12.5px', color: 'var(--color-gray-300)', fontFamily: 'var(--font-body)' }

const diffCard   = { border: '1px solid rgba(255,255,255,0.06)', borderRadius: '7px', overflow: 'hidden' }
const diffHeader = { width: '100%', display: 'flex', alignItems: 'center', gap: '8px', padding: '8px 12px', background: 'var(--color-surface-0)', border: 'none', cursor: 'pointer', textAlign: 'left' as const }
const diffPath   = { fontFamily: 'var(--font-mono)', fontSize: '11px', color: 'var(--color-gray-200)', flex: 1, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' as const }
const diffStats  = { fontFamily: 'var(--font-mono)', fontSize: '11px', flexShrink: 0 }
const newBadge   = { fontFamily: 'var(--font-mono)', fontSize: '10px', fontWeight: 700, color: '#4ade80', flexShrink: 0 }
const diffBody   = { background: 'rgba(0,0,0,0.3)', maxHeight: '320px', overflowY: 'auto' as const, overflowX: 'auto' as const }

const gridWrap  = { flex: 1, overflowY: 'auto' as const, padding: '28px 32px' }
const tabEmpty  = { fontSize: '13px', color: 'var(--color-gray-600)', fontFamily: 'var(--font-body)' }
const grid      = { display: 'grid', gridTemplateColumns: 'repeat(3, 1fr)', gap: '12px', alignItems: 'start' } as Record<string, any>
const gridCard  = { background: 'var(--color-surface-1)', border: '1px solid rgba(255,255,255,0.07)', borderRadius: '10px', padding: '16px 18px' }

const knowledgeKey     = { fontFamily: 'var(--font-mono)', fontSize: '12px', fontWeight: 700, color: 'var(--color-info)', letterSpacing: '0.04em' }
const knowledgeContent = { margin: 0, fontFamily: 'var(--font-mono)', fontSize: '11.5px', color: 'var(--color-gray-300)', lineHeight: 1.6, whiteSpace: 'pre-wrap' as const, background: 'rgba(0,0,0,0.25)', border: '1px solid rgba(255,255,255,0.05)', borderRadius: '6px', padding: '10px 12px' }

const artifactName    = { fontFamily: 'var(--font-body)', fontSize: '13.5px', fontWeight: 600, color: 'var(--color-fg-primary)' }
const artifactType    = { fontSize: '10px', fontWeight: 700, letterSpacing: '0.08em', color: 'var(--color-gray-500)', fontFamily: 'var(--font-body)', textTransform: 'uppercase' as const, background: 'rgba(255,255,255,0.05)', padding: '2px 7px', borderRadius: '4px' }
const versionBadge    = { fontSize: '10px', color: 'var(--color-amber)', fontFamily: 'var(--font-mono)', background: 'rgba(245,197,24,0.10)', padding: '2px 7px', borderRadius: '4px' }
const codeWrap        = { position: 'relative' } as Record<string, any>
const codeBlock       = { margin: 0, padding: '12px', background: 'rgba(0,0,0,0.3)', border: '1px solid rgba(255,255,255,0.06)', borderRadius: '6px', fontFamily: 'var(--font-mono)', fontSize: '12px', color: 'var(--color-gray-200)', overflowX: 'auto' as const, whiteSpace: 'pre' as const, lineHeight: 1.6 }
const copyBtn         = { position: 'absolute', top: '8px', right: '8px', padding: '4px 10px', background: 'rgba(255,255,255,0.08)', border: '1px solid rgba(255,255,255,0.12)', borderRadius: '4px', color: 'var(--color-gray-300)', fontFamily: 'var(--font-body)', fontSize: '11px', cursor: 'pointer' } as Record<string, any>
const genericContent  = { fontSize: '12.5px', color: 'var(--color-gray-300)', fontFamily: 'var(--font-body)', lineHeight: 1.6, whiteSpace: 'pre-wrap' as const }
</script>
