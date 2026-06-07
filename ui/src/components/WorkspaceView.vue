<template>
  <div style="flex:1;display:flex;flex-direction:column;overflow:hidden">
    <!-- Top bar -->
    <div :style="topBar">
      <div style="padding:22px 32px 0;display:flex;align-items:flex-start;gap:24px">
        <div style="flex:1;min-width:0">
          <div :style="breadcrumb">
            <AppIcon name="layers" :size="11" color="var(--color-amber)" :extra-style="{ flexShrink: 0 }" />
            <span>Workspace</span>
            <span style="color:var(--color-gray-700)">·</span>
            <span>{{ workspace.branch }}</span>
          </div>
          <div :style="wsTitle">{{ workspace.name }}</div>
        </div>

        <div style="display:flex;gap:8px;flex-shrink:0;padding-top:6px;position:relative">
          <button :style="btnGhost" @click="menuOpen = !menuOpen">
            <AppIcon name="more-horizontal" :size="14" />
          </button>
          <div v-if="menuOpen" :style="dropdownMenu" @mouseleave="menuOpen = false">
            <button
              v-if="workspace.status === 'idle'"
              :style="dropdownItem('var(--color-error)')"
              @click="deleteWorkspace"
            >
              <AppIcon name="trash-2" :size="13" />
              Delete workspace
            </button>
            <div v-else :style="dropdownItem('var(--color-gray-600)')">
              Delete (must be idle)
            </div>
          </div>
        </div>
      </div>

      <!-- Tabs -->
      <div style="display:flex;gap:4px;padding:0 28px;margin-top:12px">
        <button
          v-for="tab in tabs"
          :key="tab.id"
          :style="tabBtn(tab.id === activeTab)"
          @click="switchTab(tab.id)"
        >
          <AppIcon :name="tab.icon" :size="13" />
          {{ tab.label }}
          <span v-if="tab.id === activeTab" :style="tabUnderline" />
        </button>
      </div>
    </div>

    <!-- Tab content + protocol panel -->
    <div style="flex:1;display:flex;flex-direction:column;overflow:hidden">
      <div style="flex:1;display:flex;overflow:hidden">
        <GroomingView
          v-if="activeTab === 'grooming'"
          :workspace="liveWorkspace"
          @jump-tab="switchTab($event)"
          @status-change="liveStatus = $event"
        />

        <template v-else-if="activeTab === 'execution'">
          <ExecutionView v-if="isExecutionPhase" :workspace="liveWorkspace" />
          <div v-else class="fade-in" :style="emptyWrap">
            <div :style="emptyIcon"><AppIcon name="route" :size="22" color="var(--color-gray-500)" /></div>
            <div :style="emptyTitle">Execution hasn't started yet</div>
            <div :style="emptyBody">Finish grooming first to unlock this phase.</div>
            <div :style="lockedBadge">
              <AppIcon name="lock" :size="11" />
              Grooming must finish first
            </div>
          </div>
        </template>

        <div v-else-if="activeTab === 'testing'" class="fade-in" :style="emptyWrap">
          <div :style="emptyIcon"><AppIcon name="flask-conical" :size="22" color="var(--color-gray-500)" /></div>
          <div :style="emptyTitle">Nothing to test yet</div>
          <div :style="emptyBody">REST endpoints pre-populated from your Swagger map will land here after the build phase.</div>
          <div :style="lockedBadge">
            <AppIcon name="lock" :size="11" />
            Grooming must finish first
          </div>
        </div>
      </div>

      <!-- Protocol log panel -->
      <div v-if="protocolOpen" :style="protocolPanel">
        <ProtocolLog :workspace-id="workspace.id" />
      </div>

      <!-- Protocol toggle strip -->
      <div :style="protocolStrip" @click="protocolOpen = !protocolOpen">
        <AppIcon name="terminal" :size="11" />
        <span>Protocol Log</span>
        <AppIcon
          name="chevron-down"
          :size="12"
          :extra-style="{ transform: protocolOpen ? 'none' : 'rotate(180deg)', transition: 'transform 150ms ease' }"
        />
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, ref, onMounted, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import type { Workspace } from '../types'
import AppIcon from './AppIcon.vue'
import GroomingView from './grooming/GroomingView.vue'
import ExecutionView from './execution/ExecutionView.vue'
import ProtocolLog from './ProtocolLog.vue'

const props = defineProps<{ workspace: Workspace & { branch: string } }>()

const route = useRoute()
const router = useRouter()

const menuOpen = ref(false)
const liveStatus = ref<string>(props.workspace.status ?? 'idle')

const executionStatuses = new Set(['awaiting_execution', 'executing', 'done'])
const isExecutionPhase = computed(() => executionStatuses.has(liveStatus.value))
const liveWorkspace = computed(() => ({ ...props.workspace, status: liveStatus.value as Workspace['status'] }))

async function refreshStatus() {
  try {
    const r = await fetch(`/api/workspaces/${props.workspace.id}`)
    if (r.ok) { const ws = await r.json(); liveStatus.value = ws.status }
  } catch { /* ignore */ }
}

onMounted(() => { refreshStatus() })
watch(() => props.workspace.id, () => {
  liveStatus.value = props.workspace.status ?? 'idle'
  refreshStatus()
})

async function deleteWorkspace() {
  menuOpen.value = false
  if (!confirm(`Delete workspace "${props.workspace.name}"? This cannot be undone.`)) return
  const r = await fetch(`/api/workspaces/${props.workspace.id}`, { method: 'DELETE' })
  if (r.ok || r.status === 204) {
    router.push('/project/' + route.params.projectId + '/reconnaissance')
  }
}

const activeTab = computed(() => route.path.endsWith('/execution') ? 'execution' : 'grooming')

function switchTab(tabId: string) {
  const base = '/project/' + route.params.projectId + '/workspace/' + route.params.workspaceId
  router.push(tabId === 'execution' ? base + '/execution' : base)
}

const tabs = [
  { id: 'grooming',   label: 'Grooming',  icon: 'message-square-text' },
  { id: 'execution',  label: 'Execution', icon: 'route' },
  { id: 'testing',    label: 'Testing',   icon: 'flask-conical' },
]

const protocolOpen = ref(false)

const topBar      = { borderBottom: '1px solid rgba(255,255,255,0.06)', background: 'var(--color-surface-0)', flexShrink: 0 }
const breadcrumb  = { display: 'flex', alignItems: 'center', gap: '8px', marginBottom: '6px', fontFamily: 'var(--font-mono)', fontSize: '11px', color: 'var(--color-gray-500)', whiteSpace: 'nowrap', overflow: 'hidden' }
const wsTitle     = { fontFamily: 'var(--font-display)', fontSize: '28px', fontWeight: 700, color: 'var(--color-fg-primary)', letterSpacing: '-0.02em', whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis' }
const btnGhost    = { display: 'inline-flex', alignItems: 'center', gap: '6px', padding: '7px 11px', borderRadius: '6px', background: 'transparent', border: '1px solid rgba(255,255,255,0.10)', color: 'var(--color-gray-200)', cursor: 'pointer', fontFamily: 'var(--font-body)', fontSize: '12px', fontWeight: 500 }
const tabBtn      = (active: boolean) => ({ position: 'relative', display: 'inline-flex', alignItems: 'center', gap: '7px', padding: '10px 14px 12px', background: 'transparent', border: 'none', cursor: 'pointer', color: active ? 'var(--color-amber)' : 'var(--color-gray-300)', fontFamily: 'var(--font-body)', fontSize: '13.5px', fontWeight: active ? 600 : 500 } as Record<string, any>)
const tabUnderline = { position: 'absolute', left: '8px', right: '8px', bottom: '-1px', height: '2px', background: 'var(--color-amber)', borderRadius: '2px' } as Record<string, any>
const emptyWrap   = { flex: 1, display: 'flex', flexDirection: 'column', alignItems: 'center', justifyContent: 'center', padding: '40px', textAlign: 'center' } as Record<string, any>
const emptyIcon   = { width: '56px', height: '56px', borderRadius: '14px', background: 'rgba(255,255,255,0.03)', border: '1px solid rgba(255,255,255,0.06)', display: 'flex', alignItems: 'center', justifyContent: 'center', marginBottom: '18px' }
const emptyTitle  = { fontFamily: 'var(--font-display)', fontSize: '22px', fontWeight: 700, color: 'var(--color-gray-100)', letterSpacing: '-0.01em', marginBottom: '8px' }
const emptyBody   = { fontSize: '13.5px', color: 'var(--color-gray-400)', lineHeight: 1.7, maxWidth: '420px', fontFamily: 'var(--font-body)' }
const lockedBadge   = { marginTop: '18px', display: 'inline-flex', alignItems: 'center', gap: '6px', padding: '6px 12px', borderRadius: '999px', background: 'rgba(255,255,255,0.03)', border: '1px solid rgba(255,255,255,0.07)', color: 'var(--color-gray-400)', fontSize: '11.5px', fontFamily: 'var(--font-body)', fontWeight: 600 }
const dropdownMenu  = { position: 'absolute', top: 'calc(100% + 6px)', right: 0, background: 'var(--color-surface-1)', border: '1px solid rgba(255,255,255,0.10)', borderRadius: '8px', padding: '4px', minWidth: '180px', zIndex: 100, boxShadow: '0 8px 24px rgba(0,0,0,0.4)' } as Record<string, any>
const dropdownItem  = (color: string) => ({ display: 'flex', alignItems: 'center', gap: '8px', width: '100%', padding: '8px 10px', background: 'transparent', border: 'none', borderRadius: '5px', color, fontFamily: 'var(--font-body)', fontSize: '13px', cursor: 'pointer', textAlign: 'left' as const })
const protocolPanel = { height: '260px', flexShrink: 0, borderTop: '1px solid rgba(255,255,255,0.06)', overflow: 'hidden', display: 'flex', flexDirection: 'column' } as Record<string, any>
const protocolStrip = { display: 'flex', alignItems: 'center', gap: '7px', padding: '6px 20px', borderTop: '1px solid rgba(255,255,255,0.06)', background: 'var(--color-surface-0)', cursor: 'pointer', flexShrink: 0, fontFamily: 'var(--font-body)', fontSize: '11.5px', fontWeight: 500, color: 'var(--color-gray-500)', userSelect: 'none' } as Record<string, any>
</script>
