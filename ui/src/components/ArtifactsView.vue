<template>
  <div class="fade-in" :style="wrap">

    <!-- Workspace history -->
    <div>
      <div :style="sectionLabel">WORKSPACE HISTORY</div>
      <div v-if="wsLoading" :style="emptyState">Loading…</div>
      <div v-else-if="workspaces.length === 0" :style="emptyState">No workspaces yet for this project.</div>
      <div v-else style="display:flex;flex-direction:column;gap:10px">
        <div v-for="ws in workspaces" :key="ws.id" :style="wsCard">
          <!-- Card header -->
          <button :style="wsHeader" @click="toggleWs(ws.id)">
            <div style="flex:1;min-width:0;display:flex;align-items:center;gap:10px">
              <span :style="wsName">{{ ws.name }}</span>
              <span :style="statusBadge(ws.status)">{{ ws.status }}</span>
            </div>
            <span :style="metaText">{{ formatDate(ws.created_at) }}</span>
            <AppIcon :name="expandedWs.has(ws.id) ? 'chevron-down' : 'chevrons-up-down'" :size="12" :extra-style="{ color: 'var(--color-gray-600)', flexShrink: 0 }" />
          </button>

          <!-- Expanded content -->
          <div v-if="expandedWs.has(ws.id)" :style="wsBody">

            <!-- Requirements -->
            <div v-if="ws.requirements.length > 0">
              <div :style="subLabel">REQUIREMENTS</div>
              <div style="display:flex;flex-direction:column;gap:6px">
                <div v-for="r in ws.requirements" :key="r.id" :style="reqCard">
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
                      <a :href="`/api/workspaces/${ws.id}/requirements/${r.id}/file`" target="_blank" :style="reqAction">Open</a>
                    </div>
                    <div v-if="r.type === 'image'" style="margin-top:6px">
                      <img :src="`/api/workspaces/${ws.id}/requirements/${r.id}/file`" :style="reqThumb" />
                    </div>
                  </div>
                </div>
              </div>
            </div>
            <div v-else :style="emptyInline">No requirements.</div>

            <!-- Approved tasks -->
            <div v-if="wsData[ws.id]?.tasks?.length">
              <div :style="subLabel">APPROVED TASKS</div>
              <div style="display:flex;flex-direction:column;gap:4px">
                <div v-for="(t, i) in wsData[ws.id].tasks" :key="t.id" :style="taskRow">
                  <span :style="taskIdx">{{ i + 1 }}</span>
                  <span :style="taskTitle">{{ t.title }}</span>
                </div>
              </div>
            </div>

            <!-- Execution diff -->
            <div v-if="wsData[ws.id]?.diff?.length">
              <div :style="subLabel">EXECUTION DIFF</div>
              <div style="display:flex;flex-direction:column;gap:6px">
                <div v-for="f in wsData[ws.id].diff" :key="f.file_path" :style="diffCard">
                  <button :style="diffHeader" @click="toggleDiffFile(ws.id + f.file_path)">
                    <span :style="diffPath">{{ f.file_path }}</span>
                    <span v-if="f.is_new" :style="newBadge">NEW</span>
                    <span :style="diffStats"><span style="color:#4ade80">+{{ f.lines_added }}</span> <span style="color:var(--color-gray-600)">/</span> <span style="color:#f87171">-{{ f.lines_removed }}</span></span>
                    <AppIcon :name="expandedDiffFiles.has(ws.id + f.file_path) ? 'chevron-down' : 'chevrons-up-down'" :size="11" :extra-style="{ color: 'var(--color-gray-600)' }" />
                  </button>
                  <div v-if="expandedDiffFiles.has(ws.id + f.file_path)" :style="diffBody">
                    <div v-for="(line, li) in f.diff.split('\n')" :key="li" :style="diffLine(line)">{{ line }}</div>
                  </div>
                </div>
              </div>
            </div>

          </div>
        </div>
      </div>
    </div>

    <!-- Divider -->
    <div :style="divider" />

    <!-- Project artifacts -->
    <div>
      <div :style="sectionLabel">PROJECT ARTIFACTS</div>
      <div v-if="artifactsLoading" :style="emptyState">Loading…</div>
      <div v-else-if="artifacts.length === 0" :style="emptyState">No project artifacts yet.</div>
      <div v-else style="display:flex;flex-direction:column;gap:8px">
        <div v-for="a in artifacts" :key="a.id" :style="artifactCard">
          <div style="display:flex;align-items:center;gap:8px;margin-bottom:8px">
            <span :style="artifactName">{{ a.name }}</span>
            <span :style="artifactType">{{ a.type }}</span>
            <span :style="versionBadge">v{{ a.version }}</span>
            <span :style="metaText" style="margin-left:auto">{{ formatDate(a.created_at) }}</span>
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
import { ref, onMounted, watch } from 'vue'
import type { WorkspaceWithRequirements, ProjectArtifact, ApiTask } from '../types'
import AppIcon from './AppIcon.vue'

const props = defineProps<{ projectId: string }>()

interface FileDiff { file_path: string; diff: string; is_new: boolean; lines_added: number; lines_removed: number }
interface WsData { tasks: ApiTask[]; diff: FileDiff[] }

const workspaces = ref<WorkspaceWithRequirements[]>([])
const artifacts = ref<ProjectArtifact[]>([])
const wsData = ref<Record<string, WsData>>({})
const wsLoading = ref(true)
const artifactsLoading = ref(true)

const expandedWs = ref(new Set<string>())
const expandedReqs = ref(new Set<string>())
const expandedDiffFiles = ref(new Set<string>())
const copiedId = ref('')

onMounted(() => { load() })
watch(() => props.projectId, () => { reset(); load() })

function reset() {
  workspaces.value = []
  artifacts.value = []
  wsData.value = {}
  expandedWs.value = new Set()
  expandedReqs.value = new Set()
  expandedDiffFiles.value = new Set()
  wsLoading.value = true
  artifactsLoading.value = true
}

async function load() {
  const [wsRes, artRes] = await Promise.allSettled([
    fetch(`/api/workspaces?project_id=${props.projectId}&include_deleted=true`),
    fetch(`/api/fs/projects/${props.projectId}/artifacts`),
  ])
  if (wsRes.status === 'fulfilled' && wsRes.value.ok) workspaces.value = await wsRes.value.json()
  wsLoading.value = false
  if (artRes.status === 'fulfilled' && artRes.value.ok) artifacts.value = await artRes.value.json()
  artifactsLoading.value = false
}

async function toggleWs(id: string) {
  const s = new Set(expandedWs.value)
  if (s.has(id)) { s.delete(id); expandedWs.value = s; return }
  s.add(id)
  expandedWs.value = s
  if (!wsData.value[id]) await loadWsData(id)
}

async function loadWsData(wsId: string) {
  const [tasksRes, diffRes] = await Promise.allSettled([
    fetch(`/api/workspaces/${wsId}/tasks`),
    fetch(`/api/lg/execution-diff?session=${wsId}`),
  ])
  const tasks: ApiTask[] = tasksRes.status === 'fulfilled' && tasksRes.value.ok
    ? (await tasksRes.value.json()).filter((t: ApiTask) => t.status === 'approved')
    : []
  const diffData = diffRes.status === 'fulfilled' && diffRes.value.ok ? await diffRes.value.json() : null
  const diff: FileDiff[] = diffData?.files ?? []
  wsData.value = { ...wsData.value, [wsId]: { tasks, diff } }
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
  done: 'rgba(74,222,128,0.12)', deleted: 'rgba(255,255,255,0.05)',
}
const statusText: Record<string, string> = {
  idle: 'var(--color-gray-300)', grooming: 'var(--color-info)',
  awaiting_execution: 'var(--color-amber)', executing: 'var(--color-success)',
  done: 'var(--color-success)', deleted: 'var(--color-gray-600)',
}
function statusBadge(status: string) {
  return { display: 'inline-flex', alignItems: 'center', padding: '2px 8px', borderRadius: '999px', background: statusColors[status] ?? 'rgba(255,255,255,0.06)', color: statusText[status] ?? 'var(--color-gray-500)', fontSize: '10px', fontWeight: 700, letterSpacing: '0.06em', textTransform: 'uppercase' as const, fontFamily: 'var(--font-body)' }
}

const wrap          = { maxWidth: '800px', margin: '40px auto 0', padding: '0 32px 60px', display: 'flex', flexDirection: 'column', gap: '28px' } as Record<string, any>
const sectionLabel  = { fontSize: '10px', fontWeight: 700, letterSpacing: '0.12em', color: 'var(--color-gray-600)', fontFamily: 'var(--font-body)', textTransform: 'uppercase' as const, marginBottom: '10px' }
const subLabel      = { fontSize: '10px', fontWeight: 700, letterSpacing: '0.10em', color: 'var(--color-gray-700)', fontFamily: 'var(--font-body)', textTransform: 'uppercase' as const, margin: '14px 0 6px' }
const emptyState    = { fontSize: '13px', color: 'var(--color-gray-600)', fontFamily: 'var(--font-body)', padding: '12px 0' }
const emptyInline   = { fontSize: '12px', color: 'var(--color-gray-700)', fontFamily: 'var(--font-body)', paddingTop: '8px' }
const divider       = { height: '1px', background: 'rgba(255,255,255,0.06)' }
const metaText      = { fontSize: '11px', color: 'var(--color-gray-600)', fontFamily: 'var(--font-mono)', flexShrink: 0 }

const wsCard   = { background: 'var(--color-surface-1)', border: '1px solid rgba(255,255,255,0.07)', borderRadius: '10px', overflow: 'hidden' }
const wsHeader = { width: '100%', display: 'flex', alignItems: 'center', gap: '10px', padding: '14px 18px', background: 'transparent', border: 'none', cursor: 'pointer', textAlign: 'left' as const }
const wsName   = { fontFamily: 'var(--font-display)', fontSize: '14px', fontWeight: 600, color: 'var(--color-fg-primary)', letterSpacing: '-0.01em' }
const wsBody   = { padding: '0 18px 16px', display: 'flex', flexDirection: 'column' as const }

const reqCard       = { background: 'var(--color-surface-0)', border: '1px solid rgba(255,255,255,0.05)', borderRadius: '6px', padding: '8px 12px', display: 'flex', alignItems: 'flex-start', gap: '8px' }
const reqToggleBtn  = { width: '100%', background: 'transparent', border: 'none', padding: 0, cursor: 'pointer', textAlign: 'left' as const, display: 'flex', alignItems: 'center', justifyContent: 'space-between', gap: '8px' }
const reqPreview    = { flex: 1, minWidth: 0, fontSize: '12px', color: 'var(--color-gray-400)', fontFamily: 'var(--font-body)', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' as const }
const reqAction     = { fontSize: '11px', color: 'var(--color-amber)', fontFamily: 'var(--font-body)', fontWeight: 600, flexShrink: 0, textDecoration: 'none', cursor: 'pointer', background: 'transparent', border: 'none' }
const reqFullText   = { fontSize: '12px', color: 'var(--color-gray-300)', fontFamily: 'var(--font-body)', lineHeight: 1.6, whiteSpace: 'pre-wrap' as const, marginTop: '6px' }
const reqThumb      = { maxWidth: '200px', maxHeight: '120px', borderRadius: '4px', objectFit: 'cover' as const, border: '1px solid rgba(255,255,255,0.06)' }

const taskRow  = { display: 'flex', alignItems: 'flex-start', gap: '10px', padding: '4px 0' }
const taskIdx  = { width: '16px', height: '16px', borderRadius: '4px', background: 'rgba(255,255,255,0.06)', display: 'inline-flex', alignItems: 'center', justifyContent: 'center', fontFamily: 'var(--font-mono)', fontSize: '10px', color: 'var(--color-gray-500)', flexShrink: 0, marginTop: '1px' }
const taskTitle = { fontSize: '12.5px', color: 'var(--color-gray-300)', fontFamily: 'var(--font-body)' }

const diffCard   = { border: '1px solid rgba(255,255,255,0.06)', borderRadius: '7px', overflow: 'hidden' }
const diffHeader = { width: '100%', display: 'flex', alignItems: 'center', gap: '8px', padding: '8px 12px', background: 'var(--color-surface-0)', border: 'none', cursor: 'pointer', textAlign: 'left' as const }
const diffPath   = { fontFamily: 'var(--font-mono)', fontSize: '11px', color: 'var(--color-gray-200)', flex: 1, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' as const }
const diffStats  = { fontFamily: 'var(--font-mono)', fontSize: '11px', flexShrink: 0 }
const newBadge   = { fontFamily: 'var(--font-mono)', fontSize: '10px', fontWeight: 700, color: '#4ade80', flexShrink: 0 }
const diffBody   = { background: 'rgba(0,0,0,0.3)', maxHeight: '320px', overflowY: 'auto' as const, overflowX: 'auto' as const }

const artifactCard    = { background: 'var(--color-surface-1)', border: '1px solid rgba(255,255,255,0.07)', borderRadius: '10px', padding: '16px 18px' }
const artifactName    = { fontFamily: 'var(--font-body)', fontSize: '13.5px', fontWeight: 600, color: 'var(--color-fg-primary)' }
const artifactType    = { fontSize: '10px', fontWeight: 700, letterSpacing: '0.08em', color: 'var(--color-gray-500)', fontFamily: 'var(--font-body)', textTransform: 'uppercase' as const, background: 'rgba(255,255,255,0.05)', padding: '2px 7px', borderRadius: '4px' }
const versionBadge    = { fontSize: '10px', color: 'var(--color-amber)', fontFamily: 'var(--font-mono)', background: 'rgba(245,197,24,0.10)', padding: '2px 7px', borderRadius: '4px' }
const codeWrap        = { position: 'relative' } as Record<string, any>
const codeBlock       = { margin: 0, padding: '12px', background: 'rgba(0,0,0,0.3)', border: '1px solid rgba(255,255,255,0.06)', borderRadius: '6px', fontFamily: 'var(--font-mono)', fontSize: '12px', color: 'var(--color-gray-200)', overflowX: 'auto' as const, whiteSpace: 'pre' as const, lineHeight: 1.6 }
const copyBtn         = { position: 'absolute', top: '8px', right: '8px', padding: '4px 10px', background: 'rgba(255,255,255,0.08)', border: '1px solid rgba(255,255,255,0.12)', borderRadius: '4px', color: 'var(--color-gray-300)', fontFamily: 'var(--font-body)', fontSize: '11px', cursor: 'pointer' } as Record<string, any>
const genericContent  = { fontSize: '12.5px', color: 'var(--color-gray-300)', fontFamily: 'var(--font-body)', lineHeight: 1.6, whiteSpace: 'pre-wrap' as const }
</script>
