<template>
  <div style="flex:1;display:flex;overflow:hidden;background:var(--color-surface-0)">
    <div style="flex:1;overflow:auto">

      <!-- awaiting_execution -->
      <div v-if="phase === 'awaiting_execution'" class="fade-in" style="max-width:760px;margin:24px auto 0;padding:0 32px 40px">
        <div :style="phaseTitle" style="margin-bottom:6px">Execution plan</div>
        <div :style="phaseSub">{{ approvedTasks.length }} task{{ approvedTasks.length !== 1 ? 's' : '' }} approved. Start the session to implement them.</div>

        <div style="display:flex;flex-direction:column;gap:8px;margin-bottom:28px">
          <div
            v-for="(task, i) in approvedTasks"
            :key="task.id"
            :style="taskRow"
          >
            <div :style="taskIndex">{{ i + 1 }}</div>
            <div style="flex:1;min-width:0">
              <div :style="taskTitle">{{ task.title }}</div>
              <div v-if="task.reason" :style="taskReason">{{ task.reason }}</div>
            </div>
          </div>
        </div>

        <div v-if="startError" :style="errorText">{{ startError }}</div>

        <button :disabled="starting" :style="startBtnStyle" @click="handleStart">
          <AppIcon v-if="!starting" name="play" :size="14" />
          {{ starting ? 'Starting…' : 'Start execution' }}
        </button>
      </div>

      <!-- executing -->
      <div v-else-if="phase === 'executing'" class="fade-in" style="max-width:760px;margin:40px auto 0;padding:0 32px 40px">
        <template v-if="sessionIdleSec >= 300 || sessionIdleSec < 0">
          <div style="display:flex;align-items:center;gap:12px;margin-bottom:10px">
            <AppIcon name="triangle-alert" :size="20" color="var(--color-amber)" />
            <div :style="phaseTitle">{{ sessionIdleSec < 0 ? 'Session ended unexpectedly' : 'Session appears stuck' }}</div>
          </div>
          <div :style="phaseSub">
            <template v-if="sessionIdleSec < 0">The execution session is no longer active.</template>
            <template v-else>No activity for {{ Math.floor(sessionIdleSec / 60) }} minutes. The model may have hit an error.</template>
          </div>
          <button :style="forceStopBtnStyle" :disabled="forceStopping" @click="handleForceStop">
            {{ forceStopping ? 'Stopping…' : 'Force stop' }}
          </button>
        </template>
        <template v-else>
          <div style="display:flex;align-items:center;gap:12px;margin-bottom:10px">
            <Spinner :size="18" />
            <div :style="phaseTitle">Executing…</div>
          </div>
          <div :style="phaseSub">The model is implementing the approved tasks.</div>
          <div v-if="echoMessages.length > 0" ref="feedEl" :style="feedWrap">
            <div
              v-for="(msg, i) in echoMessages"
              :key="i"
              style="display:flex;gap:10px;padding:5px 0"
            >
              <span :style="feedTs">{{ formatTs(msg.ts) }}</span>
              <span :style="feedText">{{ msg.text }}</span>
            </div>
          </div>
        </template>
      </div>

      <!-- done -->
      <div v-else-if="phase === 'done'" class="fade-in" style="max-width:760px;margin:40px auto 0;padding:0 32px 60px">
        <div style="display:flex;align-items:center;gap:12px;margin-bottom:8px">
          <div :style="doneIcon"><AppIcon name="check-circle-2" :size="22" color="var(--color-success)" /></div>
          <div :style="phaseTitle">Execution complete</div>
        </div>
        <div :style="phaseSub">All approved tasks have been implemented.</div>

        <template v-if="diffFiles.length > 0">
          <div style="font-size:10px;font-weight:700;letter-spacing:0.10em;color:var(--color-gray-600);font-family:'DM Sans',sans-serif;margin-bottom:10px">
            CHANGES &nbsp;{{ diffFiles.length }} file{{ diffFiles.length !== 1 ? 's' : '' }}
          </div>
          <div style="display:flex;flex-direction:column;gap:6px">
            <div v-for="f in diffFiles" :key="f.file_path" :style="diffCard">
              <button :style="diffHeader" @click="toggleFile(f.file_path)">
                <span :style="diffPath">{{ f.file_path }}</span>
                <span :style="diffNew" v-if="f.is_new">NEW</span>
                <span :style="diffStats">
                  <span style="color:#4ade80">+{{ f.lines_added }}</span>
                  <span style="color:var(--color-gray-600)"> / </span>
                  <span style="color:#f87171">-{{ f.lines_removed }}</span>
                </span>
                <AppIcon :name="expandedFiles.has(f.file_path) ? 'chevron-down' : 'chevrons-up-down'" :size="11" :extra-style="{ color: 'var(--color-gray-600)', flexShrink: 0 }" />
              </button>
              <div v-if="expandedFiles.has(f.file_path)" :style="diffBody">
                <div v-for="(line, i) in f.diff.split('\n')" :key="i" :style="diffLine(line)">{{ line }}</div>
              </div>
            </div>
          </div>
        </template>
        <div v-else-if="diffLoaded" :style="phaseSub">No file changes recorded.</div>
      </div>

    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted, nextTick } from 'vue'
import type { ApiTask } from '../../types'
import AppIcon from '../AppIcon.vue'
import Spinner from '../grooming/Spinner.vue'

const props = defineProps<{
  workspace: { id: string; status?: string; name: string; branch: string }
}>()

type ExecPhase = 'awaiting_execution' | 'executing' | 'done'
const phase = ref<ExecPhase>('awaiting_execution')

const approvedTasks = ref<ApiTask[]>([])
const startError = ref('')
const starting = ref(false)
const forceStopping = ref(false)

const echoMessages = ref<{ ts: string; text: string }[]>([])
const sessionIdleSec = ref(0)
const feedEl = ref<HTMLElement | null>(null)

interface FileDiff {
  file_path: string
  diff: string
  is_new: boolean
  lines_added: number
  lines_removed: number
}
const diffFiles = ref<FileDiff[]>([])
const diffLoaded = ref(false)
const expandedFiles = ref(new Set<string>())

let pollTimer: ReturnType<typeof setInterval> | null = null

function initPhase() {
  const st = props.workspace.status
  if (st === 'executing') {
    phase.value = 'executing'
    startPoll()
  } else if (st === 'done') {
    phase.value = 'done'
    loadDiff()
  } else {
    phase.value = 'awaiting_execution'
    loadApprovedTasks()
  }
}

onMounted(() => { initPhase() })
onUnmounted(() => { stopPoll() })

async function loadApprovedTasks() {
  try {
    const r = await fetch(`/api/workspaces/${props.workspace.id}/tasks`)
    if (!r.ok) return
    const tasks: ApiTask[] = await r.json()
    approvedTasks.value = tasks.filter(t => t.status === 'approved')
  } catch { /* ignore */ }
}

function startPoll() {
  stopPoll()
  pollTimer = setInterval(pollExecuting, 5000)
  pollExecuting()
}

function stopPoll() {
  if (pollTimer) { clearInterval(pollTimer); pollTimer = null }
}

async function pollExecuting() {
  try {
    const [actR, wsR] = await Promise.all([
      fetch(`/api/workspaces/${props.workspace.id}/session/activity`),
      fetch(`/api/workspaces/${props.workspace.id}`),
    ])
    if (actR.ok) {
      const data = await actR.json()
      sessionIdleSec.value = data.idle_seconds ?? 0
      if (Array.isArray(data.messages)) {
        const prev = echoMessages.value.length
        echoMessages.value = data.messages
        if (data.messages.length > prev) {
          await nextTick()
          if (feedEl.value) feedEl.value.scrollTop = feedEl.value.scrollHeight
        }
      }
    }
    if (wsR.ok) {
      const ws = await wsR.json()
      if (ws.status === 'done') {
        stopPoll()
        phase.value = 'done'
        loadDiff()
      }
    }
  } catch { /* ignore */ }
}

async function handleStart() {
  startError.value = ''
  starting.value = true
  try {
    const r = await fetch(`/api/workspaces/${props.workspace.id}/execution/start`, { method: 'POST' })
    if (r.ok || r.status === 204) {
      echoMessages.value = []
      sessionIdleSec.value = 0
      phase.value = 'executing'
      startPoll()
      return
    }
    const body = await r.json().catch(() => ({}))
    if (r.status === 409) {
      startError.value = body.error ?? 'Another workspace is already executing on this project.'
    } else if (r.status === 422) {
      startError.value = body.error ?? 'Git checkout failed. Check the branch name in project settings.'
    } else {
      startError.value = body.error ?? `Error ${r.status}`
    }
  } catch {
    startError.value = 'Network error, please try again.'
  } finally {
    starting.value = false
  }
}

async function handleForceStop() {
  forceStopping.value = true
  try {
    await fetch(`/api/workspaces/${props.workspace.id}/execution/force-stop`, { method: 'POST' })
    stopPoll()
    echoMessages.value = []
    sessionIdleSec.value = 0
    phase.value = 'awaiting_execution'
    loadApprovedTasks()
  } catch { /* ignore */ } finally {
    forceStopping.value = false
  }
}

async function loadDiff() {
  try {
    const r = await fetch(`/api/lg/execution-diff?session=${props.workspace.id}`)
    if (!r.ok) return
    const data = await r.json()
    diffFiles.value = data.files ?? []
  } catch { /* ignore */ } finally {
    diffLoaded.value = true
  }
}

function toggleFile(path: string) {
  const s = new Set(expandedFiles.value)
  if (s.has(path)) { s.delete(path) } else { s.add(path) }
  expandedFiles.value = s
}

function formatTs(iso: string): string {
  const d = new Date(iso)
  return d.toLocaleTimeString('en-US', { hour12: false, hour: '2-digit', minute: '2-digit', second: '2-digit' })
}

const startBtnStyle = computed(() => ({
  display: 'inline-flex',
  alignItems: 'center',
  gap: '8px',
  padding: '10px 22px',
  background: starting.value ? 'rgba(255,255,255,0.06)' : 'var(--color-success)',
  color: starting.value ? 'var(--color-gray-500)' : 'var(--color-surface-0)',
  border: 'none',
  borderRadius: '7px',
  cursor: starting.value ? 'not-allowed' : 'pointer',
  fontFamily: 'var(--font-body)',
  fontSize: '13px',
  fontWeight: 700,
}))

const phaseTitle  = { fontFamily: 'var(--font-display)', fontSize: '22px', fontWeight: 700, color: 'var(--color-fg-primary)', letterSpacing: '-0.02em' }
const phaseSub    = { fontSize: '13.5px', color: 'var(--color-gray-300)', marginBottom: '20px', fontFamily: 'var(--font-body)', lineHeight: 1.6 }
const taskRow     = { display: 'flex', alignItems: 'flex-start', gap: '14px', padding: '14px 16px', background: 'var(--color-surface-1)', border: '1px solid rgba(255,255,255,0.06)', borderRadius: '8px' } as Record<string, any>
const taskIndex   = { width: '20px', height: '20px', borderRadius: '5px', background: 'rgba(255,255,255,0.06)', display: 'flex', alignItems: 'center', justifyContent: 'center', fontFamily: 'var(--font-mono)', fontSize: '11px', color: 'var(--color-gray-500)', flexShrink: 0, marginTop: '1px' }
const taskTitle   = { fontSize: '13.5px', fontWeight: 600, color: 'var(--color-fg-primary)', fontFamily: 'var(--font-body)', marginBottom: '4px' }
const taskReason  = { fontSize: '12.5px', color: 'var(--color-gray-400)', fontFamily: 'var(--font-body)', lineHeight: 1.55 }
const doneIcon    = { width: '56px', height: '56px', borderRadius: '14px', background: 'rgba(255,255,255,0.03)', border: '1px solid rgba(255,255,255,0.06)', display: 'flex', alignItems: 'center', justifyContent: 'center' }
const errorText   = { fontSize: '12px', color: 'var(--color-error)', marginBottom: '12px', fontFamily: 'var(--font-body)' }
const forceStopBtnStyle = { display: 'inline-flex', alignItems: 'center', gap: '7px', padding: '8px 16px', borderRadius: '6px', background: 'rgba(245,197,24,0.10)', border: '1px solid rgba(245,197,24,0.30)', color: 'var(--color-amber)', fontFamily: 'var(--font-body)', fontSize: '13px', fontWeight: 600, cursor: 'pointer' }
const feedWrap    = { marginTop: '16px', maxHeight: '240px', overflowY: 'auto', background: 'var(--color-surface-1)', border: '1px solid rgba(255,255,255,0.06)', borderRadius: '8px', padding: '10px 14px' } as Record<string, any>
const feedTs      = { fontFamily: 'var(--font-mono)', fontSize: '10px', color: 'var(--color-gray-600)', flexShrink: 0, paddingTop: '2px' }
const feedText    = { fontSize: '13px', color: 'var(--color-gray-400)', fontFamily: 'var(--font-body)', lineHeight: 1.5 }
const diffCard    = { border: '1px solid rgba(255,255,255,0.06)', borderRadius: '8px', overflow: 'hidden' }
const diffHeader  = { width: '100%', display: 'flex', alignItems: 'center', gap: '10px', padding: '10px 14px', background: 'var(--color-surface-1)', border: 'none', cursor: 'pointer', textAlign: 'left' as const }
const diffPath    = { fontFamily: 'var(--font-mono)', fontSize: '11px', color: 'var(--color-gray-200)', flex: 1, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' as const }
const diffNew     = { fontFamily: 'var(--font-mono)', fontSize: '10px', fontWeight: 700, color: '#4ade80', flexShrink: 0 }
const diffStats   = { fontFamily: 'var(--font-mono)', fontSize: '11px', flexShrink: 0 }
const diffBody    = { padding: '8px 0', background: 'rgba(0,0,0,0.3)', overflowX: 'auto' as const, maxHeight: '400px', overflowY: 'auto' as const }
function diffLine(line: string) {
  let bg = 'transparent'
  let color = 'var(--color-gray-500)'
  if (line.startsWith('+') && !line.startsWith('+++')) { bg = 'rgba(74,222,128,0.08)'; color = '#4ade80' }
  else if (line.startsWith('-') && !line.startsWith('---')) { bg = 'rgba(248,113,113,0.08)'; color = '#f87171' }
  else if (line.startsWith('@@')) { color = 'var(--color-info)' }
  return { display: 'block', padding: '0 14px', fontFamily: 'var(--font-mono)', fontSize: '11px', lineHeight: 1.6, whiteSpace: 'pre' as const, background: bg, color }
}
</script>
