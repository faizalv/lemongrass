<template>
  <div style="flex:1;display:flex;overflow:hidden;background:var(--color-surface-0)">

    <!-- awaiting_execution -->
    <div v-if="phase === 'awaiting_execution'" class="fade-in" style="flex:1;overflow:auto">
      <div style="max-width:760px;margin:24px auto 0;padding:0 32px 40px">
        <div :style="phaseTitle" style="margin-bottom:6px">Execution plan</div>
        <div :style="phaseSub">{{ execTasks.length }} task{{ execTasks.length !== 1 ? 's' : '' }} approved. Start the session to implement them.</div>

        <div style="display:flex;flex-direction:column;gap:8px;margin-bottom:28px">
          <div v-for="(task, i) in execTasks" :key="task.id" :style="taskRowWait">
            <div :style="taskIndex">{{ i + 1 }}</div>
            <div style="flex:1;min-width:0">
              <div style="display:flex;align-items:center;gap:8px;margin-bottom:4px">
                <span :style="taskTitle">{{ task.title }}</span>
                <span v-if="task.execution_status" :style="execChip(task.execution_status)">{{ execLabel(task.execution_status) }}</span>
              </div>
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
    </div>

    <!-- executing: two-column layout -->
    <template v-else-if="phase === 'executing'">
      <!-- liveness panel (left) -->
      <div :style="livenessWrap">
        <div style="padding:20px 16px;display:flex;flex-direction:column;height:100%;box-sizing:border-box">
          <div style="margin-bottom:16px">
            <div :style="panelLabel">WORKSPACE</div>
            <div :style="panelValue">{{ workspace.name }}</div>
          </div>

          <template v-if="sessionIdleSec >= 300 || sessionIdleSec < 0">
            <div style="display:flex;align-items:center;gap:8px;margin-bottom:6px">
              <AppIcon name="triangle-alert" :size="15" color="var(--color-amber)" />
              <span style="font-size:12.5px;font-weight:600;color:var(--color-amber)">
                {{ sessionIdleSec < 0 ? 'Session ended' : 'Session stuck' }}
              </span>
            </div>
            <div style="font-size:12px;color:var(--color-gray-400);margin-bottom:14px;line-height:1.55">
              {{ sessionIdleSec < 0 ? 'The execution session is no longer active.' : `No activity for ${Math.floor(sessionIdleSec / 60)} min.` }}
            </div>
            <button :style="forceStopBtnStyle" :disabled="forceStopping" @click="handleForceStop">
              {{ forceStopping ? 'Stopping…' : 'Force stop' }}
            </button>
          </template>
          <template v-else>
            <div style="display:flex;align-items:center;gap:7px;margin-bottom:4px">
              <Spinner :size="13" />
              <span style="font-size:11.5px;color:var(--color-gray-500)">Executing</span>
            </div>
            <div style="font-size:12.5px;font-weight:600;color:var(--color-fg-primary);min-height:18px;margin-bottom:12px;overflow:hidden;text-overflow:ellipsis;white-space:nowrap">
              {{ currentTaskTitle ?? 'Starting up…' }}
            </div>
            <div v-if="echoMessages.length > 0" ref="feedEl" :style="feedWrap" style="flex:1;min-height:0">
              <div v-for="(msg, i) in echoMessages" :key="i" style="display:flex;gap:8px;padding:4px 0">
                <span :style="feedTs">{{ formatTs(msg.ts) }}</span>
                <span :style="feedText">{{ msg.text }}</span>
              </div>
            </div>
          </template>
        </div>
      </div>

      <!-- tasks panel (right) -->
      <div :style="tasksWrap">
        <div :style="panelSectionLabel">TASKS &nbsp;{{ execTasks.length }}</div>
        <div style="display:flex;flex-direction:column;gap:6px">
          <div v-for="(task, i) in execTasks" :key="task.id">
            <div :style="taskRow">
              <div :style="taskIndex">{{ i + 1 }}</div>
              <div style="flex:1;min-width:0">
                <div style="display:flex;align-items:center;gap:8px">
                  <span :style="taskTitle" style="overflow:hidden;text-overflow:ellipsis;white-space:nowrap">{{ task.title }}</span>
                  <span :style="execChip(task.execution_status || '')">
                    <span v-if="task.execution_status === 'in_progress'" class="pulse-dot" />
                    {{ execLabel(task.execution_status || '') }}
                  </span>
                </div>
              </div>
              <button
                v-if="task.execution_status === 'done'"
                :style="expandBtn"
                @click="toggleTask(task.id)"
              >
                <AppIcon :name="expandedTaskIds.has(task.id) ? 'chevron-up' : 'chevron-down'" :size="13" />
              </button>
            </div>

            <!-- expanded done card -->
            <div v-if="task.execution_status === 'done' && expandedTaskIds.has(task.id)" :style="expandedCard">
              <div v-if="task.execution_notes" style="font-size:12.5px;color:var(--color-gray-300);margin-bottom:10px;line-height:1.55">{{ task.execution_notes }}</div>

              <div v-if="task.execution_diff && task.execution_diff.length > 0" style="display:flex;flex-direction:column;gap:4px;margin-bottom:12px">
                <div v-for="f in task.execution_diff" :key="f.file_path" :style="diffCard">
                  <button :style="diffHeader" @click="toggleFile(task.id + ':' + f.file_path)">
                    <span :style="diffPath">{{ f.file_path }}</span>
                    <span v-if="f.is_new" :style="diffNew">NEW</span>
                    <span :style="diffStats">
                      <span style="color:#4ade80">+{{ f.lines_added }}</span>
                      <span style="color:var(--color-gray-600)"> / </span>
                      <span style="color:#f87171">-{{ f.lines_removed }}</span>
                    </span>
                    <AppIcon :name="expandedFiles.has(task.id + ':' + f.file_path) ? 'chevron-up' : 'chevrons-up-down'" :size="11" :extra-style="{ color: 'var(--color-gray-600)', flexShrink: 0 }" />
                  </button>
                  <div v-if="expandedFiles.has(task.id + ':' + f.file_path)" :style="diffBody">
                    <div v-for="(line, li) in f.diff.split('\n')" :key="li" :style="diffLine(line)">{{ line }}</div>
                  </div>
                </div>
              </div>

              <template v-if="rejectingTaskId === task.id">
                <div style="display:flex;gap:8px;align-items:flex-end">
                  <textarea
                    :value="rejectReason"
                    @input="rejectReason = ($event.target as HTMLTextAreaElement).value"
                    placeholder="Reason for rejection…"
                    :style="rejectTextarea"
                    rows="2"
                  />
                  <div style="display:flex;flex-direction:column;gap:6px">
                    <button :style="rejectSubmitBtn" :disabled="rejecting || !rejectReason.trim()" @click="submitReject(task.id)">
                      {{ rejecting ? '…' : 'Reject' }}
                    </button>
                    <button :style="rejectCancelBtn" @click="cancelReject">Cancel</button>
                  </div>
                </div>
              </template>
              <template v-else>
                <button :style="rejectBtnStyle" @click="startReject(task.id)">Reject</button>
              </template>
            </div>

            <!-- rejected card -->
            <div v-else-if="task.execution_status === 'rejected'" :style="rejectedCard">
              <div style="font-size:12px;color:var(--color-error);line-height:1.55">{{ task.rejection_reason }}</div>
            </div>
          </div>
        </div>
      </div>
    </template>

    <!-- done -->
    <div v-else-if="phase === 'done'" class="fade-in" style="flex:1;overflow:auto">
      <div style="max-width:900px;margin:40px auto 0;padding:0 32px 60px">
        <div style="display:flex;align-items:center;gap:12px;margin-bottom:8px">
          <div :style="doneIcon"><AppIcon name="check-circle-2" :size="22" color="var(--color-success)" /></div>
          <div :style="phaseTitle">Execution complete</div>
        </div>
        <div :style="phaseSub">All approved tasks have been implemented.</div>

        <div :style="panelSectionLabel">TASKS &nbsp;{{ execTasks.length }}</div>
        <div style="display:flex;flex-direction:column;gap:6px">
          <div v-for="(task, i) in execTasks" :key="task.id">
            <div :style="taskRow">
              <div :style="taskIndex">{{ i + 1 }}</div>
              <div style="flex:1;min-width:0">
                <div style="display:flex;align-items:center;gap:8px">
                  <span :style="taskTitle" style="overflow:hidden;text-overflow:ellipsis;white-space:nowrap">{{ task.title }}</span>
                  <span :style="execChip(task.execution_status || '')">{{ execLabel(task.execution_status || '') }}</span>
                </div>
              </div>
              <button
                v-if="task.execution_status === 'done' || task.execution_status === 'rejected'"
                :style="expandBtn"
                @click="toggleTask(task.id)"
              >
                <AppIcon :name="expandedTaskIds.has(task.id) ? 'chevron-up' : 'chevron-down'" :size="13" />
              </button>
            </div>

            <div v-if="expandedTaskIds.has(task.id)" :style="expandedCard">
              <div v-if="task.rejection_reason" style="font-size:12px;color:var(--color-error);margin-bottom:8px;line-height:1.55">{{ task.rejection_reason }}</div>
              <div v-if="task.execution_notes" style="font-size:12.5px;color:var(--color-gray-300);margin-bottom:10px;line-height:1.55">{{ task.execution_notes }}</div>
              <div v-if="task.execution_diff && task.execution_diff.length > 0" style="display:flex;flex-direction:column;gap:4px">
                <div v-for="f in task.execution_diff" :key="f.file_path" :style="diffCard">
                  <button :style="diffHeader" @click="toggleFile(task.id + ':' + f.file_path)">
                    <span :style="diffPath">{{ f.file_path }}</span>
                    <span v-if="f.is_new" :style="diffNew">NEW</span>
                    <span :style="diffStats">
                      <span style="color:#4ade80">+{{ f.lines_added }}</span>
                      <span style="color:var(--color-gray-600)"> / </span>
                      <span style="color:#f87171">-{{ f.lines_removed }}</span>
                    </span>
                    <AppIcon :name="expandedFiles.has(task.id + ':' + f.file_path) ? 'chevron-up' : 'chevrons-up-down'" :size="11" :extra-style="{ color: 'var(--color-gray-600)', flexShrink: 0 }" />
                  </button>
                  <div v-if="expandedFiles.has(task.id + ':' + f.file_path)" :style="diffBody">
                    <div v-for="(line, li) in f.diff.split('\n')" :key="li" :style="diffLine(line)">{{ line }}</div>
                  </div>
                </div>
              </div>
            </div>
          </div>
        </div>
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

const execTasks = ref<ApiTask[]>([])
const startError = ref('')
const starting = ref(false)
const forceStopping = ref(false)

const echoMessages = ref<{ ts: string; text: string }[]>([])
const sessionIdleSec = ref(0)
const currentTaskTitle = ref<string | null>(null)
const feedEl = ref<HTMLElement | null>(null)

const expandedTaskIds = ref(new Set<string>())
const expandedFiles = ref(new Set<string>())
const rejectingTaskId = ref<string | null>(null)
const rejectReason = ref('')
const rejecting = ref(false)

let pollTimer: ReturnType<typeof setInterval> | null = null

function initPhase() {
  const st = props.workspace.status
  if (st === 'executing') {
    phase.value = 'executing'
    loadExecTasks()
    startPoll()
  } else if (st === 'done') {
    phase.value = 'done'
    loadExecTasks()
  } else {
    phase.value = 'awaiting_execution'
    loadExecTasks()
  }
}

onMounted(() => { initPhase() })
onUnmounted(() => { stopPoll() })

async function loadExecTasks() {
  try {
    const r = await fetch(`/api/workspaces/${props.workspace.id}/tasks`)
    if (!r.ok) return
    const tasks: ApiTask[] = await r.json()
    execTasks.value = tasks.filter(t => t.status === 'approved')
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
    const [actR, wsR, tasksR] = await Promise.all([
      fetch(`/api/workspaces/${props.workspace.id}/session/activity`),
      fetch(`/api/workspaces/${props.workspace.id}`),
      fetch(`/api/workspaces/${props.workspace.id}/tasks`),
    ])
    if (actR.ok) {
      const data = await actR.json()
      sessionIdleSec.value = data.idle_seconds ?? 0
      currentTaskTitle.value = data.current_task_title ?? null
      if (Array.isArray(data.messages)) {
        const prev = echoMessages.value.length
        echoMessages.value = data.messages
        if (data.messages.length > prev) {
          await nextTick()
          if (feedEl.value) feedEl.value.scrollTop = feedEl.value.scrollHeight
        }
      }
    }
    if (tasksR.ok) {
      const tasks: ApiTask[] = await tasksR.json()
      execTasks.value = tasks.filter(t => t.status === 'approved')
    }
    if (wsR.ok) {
      const ws = await wsR.json()
      if (ws.status === 'done') {
        stopPoll()
        phase.value = 'done'
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
      currentTaskTitle.value = null
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
    currentTaskTitle.value = null
    phase.value = 'awaiting_execution'
    loadExecTasks()
  } catch { /* ignore */ } finally {
    forceStopping.value = false
  }
}

function toggleTask(id: string) {
  const s = new Set(expandedTaskIds.value)
  if (s.has(id)) { s.delete(id) } else { s.add(id) }
  expandedTaskIds.value = s
}

function toggleFile(key: string) {
  const s = new Set(expandedFiles.value)
  if (s.has(key)) { s.delete(key) } else { s.add(key) }
  expandedFiles.value = s
}

function startReject(id: string) {
  rejectingTaskId.value = id
  rejectReason.value = ''
}

function cancelReject() {
  rejectingTaskId.value = null
  rejectReason.value = ''
}

async function submitReject(taskId: string) {
  if (!rejectReason.value.trim()) return
  rejecting.value = true
  try {
    const r = await fetch(`/api/workspaces/${props.workspace.id}/tasks/${taskId}/reject`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ reason: rejectReason.value.trim() }),
    })
    if (r.ok || r.status === 204) {
      const reason = rejectReason.value.trim()
      execTasks.value = execTasks.value.map(t =>
        t.id === taskId ? { ...t, execution_status: 'rejected', rejection_reason: reason } : t
      )
      expandedTaskIds.value = new Set([...expandedTaskIds.value].filter(id => id !== taskId))
    }
    rejectingTaskId.value = null
    rejectReason.value = ''
  } catch { /* ignore */ } finally {
    rejecting.value = false
  }
}

function formatTs(iso: string): string {
  const d = new Date(iso)
  return d.toLocaleTimeString('en-US', { hour12: false, hour: '2-digit', minute: '2-digit', second: '2-digit' })
}

function execLabel(status: string): string {
  if (status === 'in_progress') return 'in progress'
  if (status === 'done') return 'done'
  if (status === 'rejected') return 'rejected'
  return 'pending'
}

function execChip(status: string) {
  const base = {
    display: 'inline-flex', alignItems: 'center', gap: '4px',
    padding: '2px 7px', borderRadius: '4px',
    fontFamily: 'var(--font-mono)', fontSize: '10px', fontWeight: 700,
    letterSpacing: '0.04em', flexShrink: 0,
  }
  if (status === 'in_progress') return { ...base, background: 'rgba(245,197,24,0.12)', color: 'var(--color-amber)', border: '1px solid rgba(245,197,24,0.25)' }
  if (status === 'done') return { ...base, background: 'rgba(74,222,128,0.10)', color: '#4ade80', border: '1px solid rgba(74,222,128,0.20)' }
  if (status === 'rejected') return { ...base, background: 'rgba(248,113,113,0.10)', color: '#f87171', border: '1px solid rgba(248,113,113,0.20)' }
  return { ...base, background: 'rgba(255,255,255,0.04)', color: 'var(--color-gray-500)', border: '1px solid rgba(255,255,255,0.08)' }
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

const phaseTitle    = { fontFamily: 'var(--font-display)', fontSize: '22px', fontWeight: 700, color: 'var(--color-fg-primary)', letterSpacing: '-0.02em' }
const phaseSub      = { fontSize: '13.5px', color: 'var(--color-gray-300)', marginBottom: '20px', fontFamily: 'var(--font-body)', lineHeight: 1.6 }
const taskRowWait   = { display: 'flex', alignItems: 'flex-start', gap: '14px', padding: '14px 16px', background: 'var(--color-surface-1)', border: '1px solid rgba(255,255,255,0.06)', borderRadius: '8px' } as Record<string, any>
const taskRow       = { display: 'flex', alignItems: 'center', gap: '14px', padding: '12px 16px', background: 'var(--color-surface-1)', border: '1px solid rgba(255,255,255,0.06)', borderRadius: '8px' } as Record<string, any>
const taskIndex     = { width: '20px', height: '20px', borderRadius: '5px', background: 'rgba(255,255,255,0.06)', display: 'flex', alignItems: 'center', justifyContent: 'center', fontFamily: 'var(--font-mono)', fontSize: '11px', color: 'var(--color-gray-500)', flexShrink: 0 }
const taskTitle     = { fontSize: '13.5px', fontWeight: 600, color: 'var(--color-fg-primary)', fontFamily: 'var(--font-body)' }
const taskReason    = { fontSize: '12.5px', color: 'var(--color-gray-400)', fontFamily: 'var(--font-body)', lineHeight: 1.55 }
const doneIcon      = { width: '56px', height: '56px', borderRadius: '14px', background: 'rgba(255,255,255,0.03)', border: '1px solid rgba(255,255,255,0.06)', display: 'flex', alignItems: 'center', justifyContent: 'center' }
const errorText     = { fontSize: '12px', color: 'var(--color-error)', marginBottom: '12px', fontFamily: 'var(--font-body)' }
const forceStopBtnStyle = { display: 'inline-flex', alignItems: 'center', gap: '7px', padding: '8px 16px', borderRadius: '6px', background: 'rgba(245,197,24,0.10)', border: '1px solid rgba(245,197,24,0.30)', color: 'var(--color-amber)', fontFamily: 'var(--font-body)', fontSize: '13px', fontWeight: 600, cursor: 'pointer' }
const feedWrap      = { overflowY: 'auto', background: 'var(--color-surface-1)', border: '1px solid rgba(255,255,255,0.06)', borderRadius: '8px', padding: '8px 12px' } as Record<string, any>
const feedTs        = { fontFamily: 'var(--font-mono)', fontSize: '10px', color: 'var(--color-gray-600)', flexShrink: 0, paddingTop: '2px' }
const feedText      = { fontSize: '12px', color: 'var(--color-gray-400)', fontFamily: 'var(--font-body)', lineHeight: 1.5 }
const livenessWrap  = { width: '280px', borderRight: '1px solid rgba(255,255,255,0.06)', display: 'flex', flexDirection: 'column', overflow: 'hidden', flexShrink: 0 } as Record<string, any>
const tasksWrap     = { flex: 1, overflowY: 'auto', padding: '20px 24px' } as Record<string, any>
const panelLabel    = { fontSize: '10px', fontWeight: 700, letterSpacing: '0.10em', color: 'var(--color-gray-600)', fontFamily: 'var(--font-body)', marginBottom: '2px' }
const panelValue    = { fontSize: '13px', fontWeight: 600, color: 'var(--color-fg-primary)', fontFamily: 'var(--font-body)' }
const panelSectionLabel = { fontSize: '10px', fontWeight: 700, letterSpacing: '0.10em', color: 'var(--color-gray-600)', fontFamily: 'var(--font-body)', marginBottom: '12px' }
const expandBtn     = { display: 'flex', alignItems: 'center', justifyContent: 'center', width: '24px', height: '24px', background: 'transparent', border: 'none', cursor: 'pointer', color: 'var(--color-gray-500)', flexShrink: 0, padding: 0 }
const expandedCard  = { marginTop: '2px', padding: '14px 16px', background: 'rgba(0,0,0,0.2)', borderRadius: '0 0 8px 8px', border: '1px solid rgba(255,255,255,0.06)', borderTop: 'none' }
const rejectedCard  = { marginTop: '2px', padding: '10px 16px', background: 'rgba(248,113,113,0.06)', borderRadius: '0 0 8px 8px', border: '1px solid rgba(248,113,113,0.15)', borderTop: 'none' }
const rejectBtnStyle   = { padding: '4px 12px', borderRadius: '5px', background: 'rgba(248,113,113,0.10)', border: '1px solid rgba(248,113,113,0.20)', color: '#f87171', fontFamily: 'var(--font-body)', fontSize: '12px', fontWeight: 600, cursor: 'pointer' }
const rejectTextarea   = { flex: 1, background: 'var(--color-surface-1)', border: '1px solid rgba(255,255,255,0.10)', borderRadius: '6px', color: 'var(--color-fg-primary)', fontFamily: 'var(--font-body)', fontSize: '12.5px', padding: '8px 10px', resize: 'none' as const, lineHeight: 1.5 }
const rejectSubmitBtn  = { padding: '6px 14px', borderRadius: '5px', background: 'rgba(248,113,113,0.15)', border: '1px solid rgba(248,113,113,0.30)', color: '#f87171', fontFamily: 'var(--font-body)', fontSize: '12px', fontWeight: 600, cursor: 'pointer', whiteSpace: 'nowrap' as const }
const rejectCancelBtn  = { padding: '6px 14px', borderRadius: '5px', background: 'transparent', border: '1px solid rgba(255,255,255,0.10)', color: 'var(--color-gray-500)', fontFamily: 'var(--font-body)', fontSize: '12px', cursor: 'pointer', whiteSpace: 'nowrap' as const }
const diffCard      = { border: '1px solid rgba(255,255,255,0.06)', borderRadius: '6px', overflow: 'hidden' }
const diffHeader    = { width: '100%', display: 'flex', alignItems: 'center', gap: '10px', padding: '8px 12px', background: 'var(--color-surface-1)', border: 'none', cursor: 'pointer', textAlign: 'left' as const }
const diffPath      = { fontFamily: 'var(--font-mono)', fontSize: '11px', color: 'var(--color-gray-200)', flex: 1, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' as const }
const diffNew       = { fontFamily: 'var(--font-mono)', fontSize: '10px', fontWeight: 700, color: '#4ade80', flexShrink: 0 }
const diffStats     = { fontFamily: 'var(--font-mono)', fontSize: '11px', flexShrink: 0 }
const diffBody      = { padding: '8px 0', background: 'rgba(0,0,0,0.3)', overflowX: 'auto' as const, maxHeight: '320px', overflowY: 'auto' as const }

function diffLine(line: string) {
  let bg = 'transparent'
  let color = 'var(--color-gray-500)'
  if (line.startsWith('+') && !line.startsWith('+++')) { bg = 'rgba(74,222,128,0.08)'; color = '#4ade80' }
  else if (line.startsWith('-') && !line.startsWith('---')) { bg = 'rgba(248,113,113,0.08)'; color = '#f87171' }
  else if (line.startsWith('@@')) { color = 'var(--color-info)' }
  return { display: 'block', padding: '0 14px', fontFamily: 'var(--font-mono)', fontSize: '11px', lineHeight: 1.6, whiteSpace: 'pre' as const, background: bg, color }
}
</script>

<style scoped>
@keyframes pulse-opacity {
  0%, 100% { opacity: 1 }
  50% { opacity: 0.3 }
}
.pulse-dot {
  display: inline-block;
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: var(--color-amber);
  animation: pulse-opacity 1.4s ease-in-out infinite;
  flex-shrink: 0;
}
.fade-in {
  animation: fadeIn 0.2s ease;
}
@keyframes fadeIn {
  from { opacity: 0 }
  to { opacity: 1 }
}
</style>
