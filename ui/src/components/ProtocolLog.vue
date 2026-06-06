<template>
  <div :style="root">
    <div :style="toolbar">
      <div :style="toolbarLeft">
        <span :style="label">Protocol Log</span>
        <span :style="countBadge">{{ calls.length }} calls</span>
      </div>
      <div style="display:flex;gap:8px;align-items:center">
        <button :style="filterBtn(filterCmd === '')" @click="filterCmd = ''">All</button>
        <button :style="filterBtn(filterCmd === 'annotate')" @click="filterCmd = 'annotate'">Annotate</button>
        <button :style="filterBtn(filterCmd === 'recon')" @click="filterCmd = 'recon'">Recon</button>
        <button :style="refreshBtnStyle" @click="load" title="Refresh">
          <AppIcon name="refresh-cw" :size="12" />
        </button>
      </div>
    </div>

    <div v-if="calls.length === 0" :style="empty">
      No protocol calls recorded yet for this workspace.
    </div>

    <div v-else :style="tableWrap">
      <table :style="table">
        <thead>
          <tr>
            <th :style="th">Time</th>
            <th :style="th">Command</th>
            <th :style="th">Args</th>
            <th :style="th">Response</th>
            <th :style="{ ...th, textAlign: 'right' }">ms</th>
          </tr>
        </thead>
        <tbody>
          <tr
            v-for="(c, i) in filtered"
            :key="i"
            :style="row(c.cmd)"
            @click="selected = selected === i ? null : i"
          >
            <td :style="td">{{ fmtTime(c.timestamp) }}</td>
            <td :style="cmdCell(c.cmd)">{{ c.cmd }}</td>
            <td :style="td">{{ truncate(c.args, 60) }}</td>
            <td :style="respCell(c.response)">{{ truncate(c.response, 80) }}</td>
            <td :style="{ ...td, textAlign: 'right', fontVariantNumeric: 'tabular-nums' }">{{ c.duration_ms }}</td>
          </tr>
        </tbody>
      </table>

      <div v-if="selected !== null && filtered[selected]" :style="detail">
        <div :style="detailRow">
          <span :style="detailKey">Command</span>
          <span :style="cmdCell(filtered[selected].cmd)">{{ filtered[selected].cmd }}</span>
        </div>
        <div v-if="filtered[selected].session_type" :style="detailRow">
          <span :style="detailKey">Session</span>
          <span :style="detailVal">{{ filtered[selected].session_type }}</span>
        </div>
        <div :style="detailRow">
          <span :style="detailKey">Duration</span>
          <span :style="detailVal">{{ filtered[selected].duration_ms }}ms</span>
        </div>
        <div :style="{ ...detailRow, alignItems: 'flex-start' }">
          <span :style="detailKey">Args</span>
          <pre :style="codeBlock">{{ filtered[selected].args || '(none)' }}</pre>
        </div>
        <div :style="{ ...detailRow, alignItems: 'flex-start' }">
          <span :style="detailKey">Response</span>
          <pre :style="respBlock(filtered[selected].response)">{{ filtered[selected].response || '(empty)' }}</pre>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted } from 'vue'
import AppIcon from './AppIcon.vue'

const props = defineProps<{ workspaceId: string }>()

interface CallRecord {
  cmd: string
  args: string
  response: string
  session_id: string
  session_type: string
  duration_ms: number
  timestamp: string
}

const calls = ref<CallRecord[]>([])
const filterCmd = ref('')
const selected = ref<number | null>(null)
let timer: ReturnType<typeof setInterval> | null = null

async function load() {
  try {
    const r = await fetch(`/api/lg/calls?workspace=${props.workspaceId}`)
    if (r.ok) {
      const data: CallRecord[] = await r.json()
      calls.value = [...data].reverse()
    }
  } catch { /* ignore */ }
}

const filtered = computed(() => {
  if (!filterCmd.value) return calls.value
  if (filterCmd.value === 'recon') return calls.value.filter(c => c.cmd.startsWith('recon.'))
  return calls.value.filter(c => c.cmd === filterCmd.value)
})

function fmtTime(ts: string) {
  const d = new Date(ts)
  return d.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit', second: '2-digit' })
}

function truncate(s: string, n: number) {
  if (!s) return ''
  return s.length > n ? s.slice(0, n) + '...' : s
}

onMounted(() => {
  load()
  timer = setInterval(load, 4000)
})
onUnmounted(() => { if (timer) clearInterval(timer) })

const root        = { flex: 1, display: 'flex', flexDirection: 'column', overflow: 'hidden', background: 'var(--color-surface-0)' } as Record<string, any>
const toolbar     = { display: 'flex', alignItems: 'center', justifyContent: 'space-between', padding: '14px 24px 12px', borderBottom: '1px solid rgba(255,255,255,0.06)', flexShrink: 0 }
const toolbarLeft = { display: 'flex', alignItems: 'center', gap: '10px' }
const label       = { fontFamily: 'var(--font-body)', fontSize: '13px', fontWeight: 600, color: 'var(--color-fg-primary)' }
const countBadge  = { fontFamily: 'var(--font-mono)', fontSize: '11px', color: 'var(--color-gray-500)', background: 'rgba(255,255,255,0.05)', padding: '2px 7px', borderRadius: '999px' }
const filterBtn   = (active: boolean) => ({ padding: '4px 10px', borderRadius: '5px', border: 'none', cursor: 'pointer', fontFamily: 'var(--font-body)', fontSize: '11.5px', fontWeight: 500, background: active ? 'rgba(255,255,255,0.10)' : 'transparent', color: active ? 'var(--color-fg-primary)' : 'var(--color-gray-500)' })
const refreshBtnStyle = { display: 'inline-flex', alignItems: 'center', padding: '5px', borderRadius: '5px', border: 'none', cursor: 'pointer', background: 'transparent', color: 'var(--color-gray-500)' }
const empty       = { flex: 1, display: 'flex', alignItems: 'center', justifyContent: 'center', fontFamily: 'var(--font-body)', fontSize: '13px', color: 'var(--color-gray-600)' }
const tableWrap   = { flex: 1, overflowY: 'auto', display: 'flex', flexDirection: 'column' } as Record<string, any>
const table       = { width: '100%', borderCollapse: 'collapse', fontFamily: 'var(--font-mono)', fontSize: '11.5px' } as Record<string, any>
const th          = { padding: '8px 16px', textAlign: 'left' as const, color: 'var(--color-gray-600)', fontFamily: 'var(--font-body)', fontSize: '11px', fontWeight: 600, borderBottom: '1px solid rgba(255,255,255,0.05)', background: 'var(--color-surface-0)', position: 'sticky', top: 0 } as Record<string, any>
const td          = { padding: '7px 16px', color: 'var(--color-gray-300)', borderBottom: '1px solid rgba(255,255,255,0.03)', verticalAlign: 'top' }
const row         = (cmd: string) => ({ cursor: 'pointer', background: cmd === 'annotate' ? 'rgba(251,191,36,0.03)' : 'transparent' })

function cmdCell(cmd: string) {
  const color = cmd === 'annotate'
    ? 'var(--color-amber)'
    : cmd.startsWith('recon.') ? 'var(--color-gray-200)' : 'var(--color-gray-500)'
  return { ...td, color, fontWeight: cmd === 'annotate' ? 600 : 400 }
}

function respCell(resp: string) {
  const isError = resp && (resp.startsWith('error:') || resp.startsWith('not found:'))
  const isOk = resp === 'ok'
  const color = isError ? 'var(--color-error)' : isOk ? 'var(--color-success, #34d399)' : 'var(--color-gray-500)'
  return { ...td, color }
}

const detail      = { flexShrink: 0, borderTop: '1px solid rgba(255,255,255,0.08)', padding: '16px 24px', display: 'flex', flexDirection: 'column', gap: '10px', background: 'var(--color-surface-1)' } as Record<string, any>
const detailRow   = { display: 'flex', gap: '16px', alignItems: 'center' }
const detailKey   = { fontFamily: 'var(--font-body)', fontSize: '11px', fontWeight: 600, color: 'var(--color-gray-600)', width: '70px', flexShrink: 0 }
const detailVal   = { fontFamily: 'var(--font-mono)', fontSize: '12px', color: 'var(--color-gray-300)' }
const codeBlock   = { margin: 0, fontFamily: 'var(--font-mono)', fontSize: '11.5px', color: 'var(--color-gray-300)', whiteSpace: 'pre-wrap', wordBreak: 'break-all', background: 'rgba(0,0,0,0.2)', padding: '8px 12px', borderRadius: '6px', flex: 1 } as Record<string, any>
const respBlock   = (resp: string) => {
  const isError = resp && (resp.startsWith('error:') || resp.startsWith('not found:'))
  const isOk = resp === 'ok'
  const color = isError ? 'var(--color-error)' : isOk ? 'var(--color-success, #34d399)' : 'var(--color-gray-300)'
  return { ...codeBlock, color }
}
</script>
