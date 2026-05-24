<template>
  <div style="flex:1;display:flex;overflow:hidden;background:#0A0A0A">

    <!-- LEFT: Implementation Details panel -->
    <div v-if="tabHasLeftPanel" :style="leftPanel">
      <div :style="leftPanelHeader">
        <div :style="leftPanelTitle">Implementation details</div>
        <div style="font-size:13px;color:#9A9A9A;font-family:'DM Sans',sans-serif">
          {{ committed.length }} ready{{ committing ? ' · 1 distilling' : '' }}
        </div>
      </div>
      <div style="flex:1;overflow:auto;padding:8px 0">
        <ImplementationDetailItem
          v-for="id in committed"
          :key="id"
          :task="taskWithIdx(id)"
          :active="activeDetail === id"
          @select="activeDetail = id"
        />
        <div v-if="committing" :style="distillingItem">
          <Spinner :size="11" />
          Distilling…
        </div>
      </div>
    </div>

    <!-- MAIN area -->
    <div style="flex:1;display:flex;flex-direction:column;overflow:hidden;background:#0A0A0A">
      <!-- Done banner -->
      <DoneBanner v-if="phase === 'done'" @reset="reset" @continue="$emit('jump-tab', 'planning')" />

      <!-- Detail viewer when a detail is selected -->
      <ImplDetailView
        v-if="tabHasLeftPanel && activeDetail"
        :task="taskWithIdx(activeDetail)"
        style="flex:1"
      />

      <!-- Grooming phase content -->
      <div v-else style="flex:1;overflow:auto">

        <!-- Idle -->
        <IdlePanel
          v-if="phase === 'idle'"
          v-model="requirement"
          :attachments="attachments"
          @start="handleStart"
          @attach="handleAttach"
          @use-sample="requirement = SAMPLE_REQUIREMENT"
        />

        <!-- Reading recon -->
        <StepLog
          v-else-if="phase === 'reading_recon'"
          title="Reading recon map…"
          subtitle="Checking which modules I already understand for this requirement."
          :steps="reconSteps"
        />

        <!-- Permission -->
        <div v-else-if="phase === 'permission'" class="fade-in" style="max-width:760px;margin:32px auto 0;padding:0 32px 40px">
          <div style="display:flex;align-items:center;gap:10px;margin-bottom:4px">
            <AppIcon name="check-circle-2" :size="16" color="#4ADE80" />
            <div :style="phaseTitle">Recon mostly hits — one module missing</div>
          </div>
          <div :style="phaseSub">4 of 5 modules already indexed. I need permission for the last one before I can plan safely.</div>
          <PermissionCard
            :path-info="RECON_PATH_INFO"
            @approve="phase = 'recon_running'"
            @skip="phase = 'generating_tasks'"
          />
        </div>

        <!-- Recon running -->
        <StepLog
          v-else-if="phase === 'recon_running'"
          title="Running recon on internal/transport/…"
          subtitle="One-time scan. This will cache and only re-index when files change."
          :steps="reconRunSteps"
        />

        <!-- Generating tasks -->
        <div v-else-if="phase === 'generating_tasks'" class="fade-in" style="max-width:760px;margin:24px auto 0;padding:0 32px 40px">
          <div style="display:flex;align-items:center;gap:10px;margin-bottom:6px">
            <Spinner :size="16" />
            <div :style="phaseTitle">Proposing task breakdown…</div>
          </div>
          <div :style="phaseSub">Reading the PRD against the recon map. Each task lands as it's finalized — review them inline.</div>
          <div style="display:flex;flex-direction:column;gap:14px">
            <div v-for="tid in streamedTasks" :key="tid" class="card-in">
              <TaskCard
                :task="taskWithIdx(tid)"
                :decision="decisions[tid] ?? null"
                :correction="corrections[tid]"
                :commit-status="commitStatusOf(tid)"
                @decide="d => handleDecide(tid, d)"
                @correction-change="t => corrections[tid] = t"
                @amend="handleAmend(tid)"
              />
            </div>
            <div v-if="streamedTasks.length < PROPOSED_TASKS.length" :style="draftingCard">
              <Spinner :size="13" />
              Drafting task {{ streamedTasks.length + 1 }}<TypingDots />
            </div>
          </div>
        </div>

        <!-- Reviewing / Done tasks list -->
        <div v-else-if="phase === 'reviewing' || phase === 'done'" class="fade-in" style="max-width:760px;margin:24px auto 0;padding:0 32px 0">
          <div style="display:flex;align-items:baseline;justify-content:space-between;margin-bottom:6px">
            <div :style="phaseTitle">Proposed tasks</div>
            <div style="font-family:'JetBrains Mono','Courier Prime',monospace;font-size:11px;color:#717171;white-space:nowrap;flex-shrink:0">
              {{ reviewedCount }}/{{ streamedTasks.length }} reviewed
            </div>
          </div>
          <div :style="phaseSub">Accept, reject, or push back with a correction on each task. Implementation details run once you've decided on all of them.</div>

          <div style="display:flex;flex-direction:column;gap:14px;padding-bottom:120px">
            <TaskCard
              v-for="task in PROPOSED_TASKS"
              :key="task.id"
              :task="taskWithIdx(task.id)"
              :decision="decisions[task.id] ?? null"
              :correction="corrections[task.id]"
              :commit-status="commitStatusOf(task.id)"
              @decide="d => handleDecide(task.id, d)"
              @correction-change="t => corrections[task.id] = t"
              @amend="handleAmend(task.id)"
            />
          </div>

          <ReviewActionBar
            v-if="phase === 'reviewing'"
            :total="streamedTasks.length"
            :reviewed-count="reviewedCount"
            :accepted-count="acceptedCount"
            :rejected-count="rejectedCount"
            :blocker="blocker"
            :can-generate="canGenerate"
            :batch-mode="batchMode"
            :committing="committing"
            :committed-count="committed.length"
            @generate="handleGenerateAll"
          />
        </div>

      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import type { Decision, GroomingPhase, ReconStep } from '../../types'
import { PROPOSED_TASKS, RECON_PATH_INFO } from '../../data/sampleData'
import AppIcon from '../AppIcon.vue'
import Spinner from './Spinner.vue'
import TypingDots from './TypingDots.vue'
import IdlePanel from './IdlePanel.vue'
import StepLog from './StepLog.vue'
import PermissionCard from './PermissionCard.vue'
import TaskCard from './TaskCard.vue'
import ReviewActionBar from './ReviewActionBar.vue'
import ImplementationDetailItem from './ImplementationDetailItem.vue'
import ImplDetailView from './ImplDetailView.vue'
import DoneBanner from './DoneBanner.vue'

defineEmits<{ 'jump-tab': [tab: string] }>()

const SAMPLE_REQUIREMENT = `Add per-user rate limiting to the public REST API.

- 60 requests/min for anonymous, 300/min for authenticated.
- Return 429 with a Retry-After header.
- Counters live in Redis, keyed by IP for anon and user_id for authed.
- Need an admin endpoint to inspect a user's current quota.
- Surface limit headers (X-RateLimit-Remaining, etc.) on every response.`

const phase = ref<GroomingPhase>('idle')
const requirement = ref('')
const attachments = ref<{ name: string; icon: string }[]>([])
const reconSteps = ref<ReconStep[]>([])
const reconRunSteps = ref<ReconStep[]>([])
const streamedTasks = ref<string[]>([])
const decisions = ref<Record<string, Decision>>({})
const corrections = ref<Record<string, string>>({})
const committed = ref<string[]>([])
const committing = ref<string | null>(null)
const batchMode = ref(false)
const activeDetail = ref<string | null>(null)

// ── Phase: reading_recon ─────────────────────────────
const RECON_STAGES = [
  { label: 'Resolved scope from requirement: REST API surface', detail: 'auth, transport, middleware' },
  { label: 'Recon hit: internal/middleware/ — 8 files, branch matches' },
  { label: 'Recon hit: internal/handlers/ — 22 files, 1 modified since last index', detail: 're-index queued' },
  { label: 'Recon hit: cmd/server/ — 4 files' },
  { label: 'Recon hit: pkg/redis/ — 3 files' },
  { label: 'Recon miss: internal/transport/ — not indexed', detail: 'need permission' },
]

let activeTimer: ReturnType<typeof setTimeout> | null = null
function clearTimer() { if (activeTimer) { clearTimeout(activeTimer); activeTimer = null } }

watch(phase, (p) => {
  clearTimer()
  if (p === 'reading_recon') startReadingRecon()
  else if (p === 'recon_running') startReconRunning()
  else if (p === 'generating_tasks') startGeneratingTasks()
})

function startReadingRecon() {
  let i = 0
  reconSteps.value = [{ label: RECON_STAGES[0].label, detail: RECON_STAGES[0].detail, status: 'pending' }]
  const tick = () => {
    const cur = RECON_STAGES[i]
    const nxt = RECON_STAGES[i + 1]
    const steps = reconSteps.value
    const last = steps[steps.length - 1]
    if (last && last.label === cur.label && last.status === 'pending') {
      steps[steps.length - 1] = { ...last, status: cur.label.includes('miss') ? 'miss' : 'ok' }
      if (nxt) steps.push({ label: nxt.label, detail: nxt.detail, status: 'pending' })
    }
    i++
    if (i < RECON_STAGES.length) {
      activeTimer = setTimeout(tick, 520)
    } else {
      activeTimer = setTimeout(() => { phase.value = 'permission' }, 600)
    }
  }
  activeTimer = setTimeout(tick, 520)
}

const RECON_RUN_STAGES = [
  'Spawning recon worker for internal/transport/',
  'Hashing 14 files (MD5)',
  'Extracting function signatures, route annotations…',
  'Computing dependency graph',
  'Persisting to ~/.lemongrass/db/recon.sqlite',
  'Recon complete — 14/14 files indexed',
]
function startReconRunning() {
  let i = 0
  reconRunSteps.value = [{ label: RECON_RUN_STAGES[0], status: 'pending' }]
  const tick = () => {
    const steps = reconRunSteps.value
    const last = steps[steps.length - 1]
    if (last && last.status === 'pending') {
      steps[steps.length - 1] = { ...last, status: 'ok' }
      if (RECON_RUN_STAGES[i + 1]) steps.push({ label: RECON_RUN_STAGES[i + 1], status: 'pending' })
    }
    i++
    if (i < RECON_RUN_STAGES.length) {
      activeTimer = setTimeout(tick, 480)
    } else {
      activeTimer = setTimeout(() => { phase.value = 'generating_tasks' }, 500)
    }
  }
  activeTimer = setTimeout(tick, 480)
}

function startGeneratingTasks() {
  streamedTasks.value = []
  let i = 0
  const tick = () => {
    const id = PROPOSED_TASKS[i]?.id
    if (!id) return
    if (!streamedTasks.value.includes(id)) streamedTasks.value.push(id)
    i++
    if (i < PROPOSED_TASKS.length) {
      activeTimer = setTimeout(tick, 850)
    } else {
      activeTimer = setTimeout(() => { phase.value = 'reviewing' }, 700)
    }
  }
  activeTimer = setTimeout(tick, 600)
}

// ── Committing animation ─────────────────────────────
watch(committing, (id) => {
  if (!id) return
  activeTimer = setTimeout(() => {
    committed.value.push(id)
    activeDetail.value = id
    committing.value = null
  }, 2400)
})

// ── Batch driver ─────────────────────────────────────
watch([batchMode, committing, () => committed.value.length], () => {
  if (!batchMode.value || committing.value) return
  const next = PROPOSED_TASKS.find(t => decisions.value[t.id] === 'accept' && !committed.value.includes(t.id))
  if (next) {
    activeTimer = setTimeout(() => { committing.value = next.id }, 320)
  } else {
    batchMode.value = false
    activeTimer = setTimeout(() => { phase.value = 'done' }, 600)
  }
})

// ── Derived ──────────────────────────────────────────
const tabHasLeftPanel = computed(() => committed.value.length > 0 || !!committing.value)

const acceptedIds = computed(() => Object.entries(decisions.value).filter(([, v]) => v === 'accept').map(([k]) => k))
const acceptedCount = computed(() => streamedTasks.value.filter(id => decisions.value[id] === 'accept').length)
const rejectedCount = computed(() => streamedTasks.value.filter(id => decisions.value[id] === 'reject').length)
const reviewedCount = computed(() => streamedTasks.value.filter(id => decisions.value[id] && decisions.value[id] !== 'correction').length)

const correctionPending = computed(() =>
  streamedTasks.value.filter(id => decisions.value[id] === 'correction' && (corrections.value[id] || '').trim().length <= 3).length
)
const anyDecisionOpen = computed(() => Object.values(decisions.value).some(v => v === 'correction'))
const allDecided = computed(() =>
  streamedTasks.value.length > 0 &&
  streamedTasks.value.every(id => decisions.value[id] && decisions.value[id] !== 'correction')
)

const blocker = computed((): string | null => {
  if (correctionPending.value > 0) return `${correctionPending.value} correction${correctionPending.value !== 1 ? 's' : ''} need text — write or amend them first.`
  if (anyDecisionOpen.value) {
    const n = Object.values(decisions.value).filter(v => v === 'correction').length
    return `${n} correction${n !== 1 ? 's' : ''} pending — amend or change decision.`
  }
  if (!allDecided.value) {
    const remaining = streamedTasks.value.length - reviewedCount.value
    return `${remaining} task${remaining !== 1 ? 's' : ''} still need a decision.`
  }
  return null
})
const canGenerate = computed(() => !blocker.value && phase.value === 'reviewing' && !batchMode.value)

// ── Helpers ──────────────────────────────────────────
function taskWithIdx(id: string) {
  const task = PROPOSED_TASKS.find(t => t.id === id)!
  return { ...task, idx: PROPOSED_TASKS.findIndex(t => t.id === id) + 1 }
}
function commitStatusOf(id: string) {
  if (committing.value === id) return 'committing' as const
  if (committed.value.includes(id)) return 'committed' as const
  return 'open' as const
}

// ── Handlers ─────────────────────────────────────────
function reset() {
  clearTimer()
  phase.value = 'idle'
  requirement.value = ''
  attachments.value = []
  reconSteps.value = []
  reconRunSteps.value = []
  streamedTasks.value = []
  decisions.value = {}
  corrections.value = {}
  committed.value = []
  committing.value = null
  activeDetail.value = null
  batchMode.value = false
}

function handleStart() { phase.value = 'reading_recon' }

function handleAttach() {
  if (attachments.value.length === 0) {
    attachments.value = [
      { name: 'PRD-rate-limit.md', icon: 'file-text' },
      { name: 'rfc-bucket-design.pdf', icon: 'file' },
    ]
  }
}

function handleDecide(taskId: string, d: Decision) {
  decisions.value = { ...decisions.value, [taskId]: d }
}
function handleAmend(taskId: string) {
  decisions.value = { ...decisions.value, [taskId]: null }
  corrections.value = { ...corrections.value, [taskId]: '' }
}
function handleGenerateAll() {
  if (acceptedCount.value === 0) { phase.value = 'done'; return }
  batchMode.value = true
}

// Styles
const leftPanel = { width: '280px', flexShrink: 0, borderRight: '1px solid rgba(255,255,255,0.06)', display: 'flex', flexDirection: 'column', background: '#0C0C0C' }
const leftPanelHeader = { padding: '18px 18px 10px', borderBottom: '1px solid rgba(255,255,255,0.05)' }
const leftPanelTitle = { fontSize: '10px', fontWeight: 700, letterSpacing: '0.12em', textTransform: 'uppercase', color: '#717171', fontFamily: "'DM Sans',sans-serif", marginBottom: '4px' }
const distillingItem = { padding: '12px 14px', margin: '4px 12px', background: 'rgba(245,197,24,0.04)', border: '1px dashed rgba(245,197,24,0.25)', borderRadius: '6px', display: 'flex', alignItems: 'center', gap: '8px', fontFamily: "'DM Sans',sans-serif", fontSize: '12px', color: '#F5C518' }
const phaseTitle = { fontFamily: "'Comfortaa', sans-serif", fontSize: '22px', fontWeight: 700, color: '#fff', letterSpacing: '-0.02em' }
const phaseSub = { fontSize: '13.5px', color: '#9A9A9A', marginBottom: '20px', fontFamily: "'DM Sans',sans-serif", lineHeight: 1.6 }
const draftingCard = { padding: '14px 18px', border: '1px dashed rgba(255,255,255,0.10)', borderRadius: '10px', display: 'flex', alignItems: 'center', gap: '10px', color: '#717171', fontFamily: "'DM Sans',sans-serif", fontSize: '13px' }
</script>
