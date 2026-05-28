<template>
  <div :style="s.root">
    <!-- Header -->
    <div :style="s.header">
      <span :style="s.title">Hook Debug</span>
      <button :style="s.closeBtn" @click="$emit('close')">✕</button>
    </div>

    <div :style="s.body">
      <!-- LEFT: Protocol Tester -->
      <div :style="s.left">
        <div :style="s.sectionLabel">Protocol Tester</div>

        <!-- Workspace picker -->
        <div style="margin-bottom:12px">
          <div :style="s.fieldLabel">Workspace</div>
          <select v-model="selectedWorkspaceId" :style="s.select">
            <option value="">-- pick a workspace --</option>
            <option v-for="w in testableWorkspaces" :key="w.id" :value="w.id">
              {{ w.name }} ({{ w.status ?? 'idle' }})
            </option>
          </select>
        </div>

        <!-- Session type -->
        <div style="display:flex;gap:6px;margin-bottom:14px">
          <button
            v-for="t in ['grooming','execution']"
            :key="t"
            :style="s.typeBtn(sessionType === t)"
            @click="sessionType = t as 'grooming' | 'execution'"
          >{{ t }}</button>
        </div>

        <!-- Command input -->
        <div :style="s.fieldLabel">Command</div>
        <div style="display:flex;gap:8px;margin-bottom:10px">
          <input
            v-model="command"
            :style="s.input"
            placeholder="#lg.recon.tree"
            @keydown.enter="sendCommand"
          />
          <button
            :disabled="!command.trim() || !selectedWorkspaceId || running"
            :style="s.sendBtn(!command.trim() || !selectedWorkspaceId || running)"
            @click="sendCommand"
          >{{ running ? '…' : 'Send' }}</button>
        </div>

        <!-- Quick fire -->
        <div style="display:flex;flex-wrap:wrap;gap:6px;margin-bottom:16px">
          <button
            v-for="q in quickCommands"
            :key="q"
            :style="s.quickBtn"
            @click="fireQuick(q)"
          >#lg.{{ q }}</button>
        </div>

        <!-- Response -->
        <div v-if="lastResult" :style="s.resultBox">
          <div style="display:flex;align-items:center;justify-content:space-between;margin-bottom:8px">
            <span :style="s.sectionLabel" style="margin:0">Response</span>
            <span style="display:flex;align-items:center;gap:8px">
              <span :style="s.elapsed">{{ lastResult.ms }}ms</span>
              <span :style="s.exitCode">exit {{ lastResult.exitCode }}</span>
              <span :style="s.badge(lastResult.ok)">{{ lastResult.ok ? 'ok' : 'error' }}</span>
            </span>
          </div>
          <pre :style="s.pre">{{ lastResult.text }}</pre>
        </div>
        <div v-else :style="s.emptyResult">
          Send a command to see the response.
        </div>

        <!-- History -->
        <div v-if="history.length > 1" style="margin-top:16px">
          <div :style="s.sectionLabel">History</div>
          <div style="display:flex;flex-direction:column;gap:4px;margin-top:6px">
            <div
              v-for="(h, i) in history.slice(1)"
              :key="i"
              :style="s.historyItem"
              @click="restoreHistory(h)"
            >
              <span :style="s.historyCmd">#lg.{{ h.cmd }}</span>
              <span :style="s.historyArgs">{{ h.args }}</span>
              <span :style="{ ...s.badge(h.ok), marginLeft: 'auto' }">{{ h.ok ? 'ok' : 'err' }}</span>
            </div>
          </div>
        </div>
      </div>

      <!-- RIGHT: Echo Log -->
      <div :style="s.right">
        <div style="display:flex;align-items:center;justify-content:space-between;margin-bottom:12px">
          <span :style="s.sectionLabel">Echo Log</span>
          <span :style="s.callCount">{{ calls.length }} calls</span>
        </div>

        <!-- PTY message send -->
        <div style="display:flex;gap:8px;margin-bottom:14px">
          <input
            v-model="ptyMessage"
            :style="s.input"
            placeholder="Send message to debug PTY…"
            :disabled="ptySending"
            @keydown.enter="sendPty"
          />
          <button
            :disabled="ptySending || !ptyMessage.trim()"
            :style="s.sendBtn(ptySending || !ptyMessage.trim())"
            @click="sendPty"
          >{{ ptySending ? '…' : 'Send' }}</button>
        </div>

        <div :style="s.callList">
          <div v-if="calls.length === 0" :style="s.empty">
            No calls yet. Start a grooming session or send a PTY message.
          </div>
          <div v-for="(call, i) in [...calls].reverse()" :key="i" :style="s.callItem">
            <span :style="s.callCmd">#lg.{{ call.cmd }}</span>
            <span :style="s.callArgs">{{ call.args || '—' }}</span>
            <span :style="s.callTime">{{ fmt(call.timestamp) }}</span>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted } from 'vue'
import type { Workspace } from '../types'

const props = defineProps<{ workspaces: Workspace[] }>()
defineEmits<{ close: [] }>()

interface Call { cmd: string; args: string; timestamp: string }
interface Result { cmd: string; args: string; text: string; ms: number; ok: boolean; exitCode: number }

const selectedWorkspaceId = ref('')
const command = ref('')
const running = ref(false)
const lastResult = ref<Result | null>(null)
const history = ref<Result[]>([])

const ptyMessage = ref('')
const ptySending = ref(false)
const calls = ref<Call[]>([])
let pollTimer: ReturnType<typeof setInterval> | null = null

const quickCommands = ['recon.tree', 'recon.search handlers', 'recon.search routes', 'echo ping']

const testableWorkspaces = computed(() =>
  props.workspaces.filter(w => w.id !== 'reconnaissance')
)

const selectedWorkspace = computed(() =>
  testableWorkspaces.value.find(w => w.id === selectedWorkspaceId.value)
)

const sessionType = ref<'grooming' | 'execution'>('grooming')

async function sendCommand() {
  const raw = command.value.trim()
  if (!raw || !selectedWorkspaceId.value || running.value) return

  running.value = true
  const t0 = Date.now()
  try {
    const r = await fetch('/api/debug/exec', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        workspace_id: selectedWorkspaceId.value,
        command: raw,
        session_type: sessionType.value,
      }),
    })
    const body = await r.json().catch(() => ({ output: '', exit_code: -1 }))
    const ms = Date.now() - t0
    const exitCode: number = body.exit_code ?? -1
    const text: string = body.output || body.error || ''
    const ok = exitCode !== -1 && !text.startsWith('error:')
    const displayCmd = raw.replace(/^#lg[!]?\./, '').split(' ')[0]
    const displayArgs = raw.replace(/^#lg[!]?\./, '').split(' ').slice(1).join(' ')
    const result: Result = { cmd: displayCmd, args: displayArgs, text, ms, ok, exitCode }
    lastResult.value = result
    history.value = [result, ...history.value].slice(0, 10)
  } catch {
    const ms = Date.now() - t0
    const result: Result = { cmd: raw, args: '', text: 'error: network failure', ms, ok: false, exitCode: -1 }
    lastResult.value = result
    history.value = [result, ...history.value].slice(0, 10)
  } finally {
    running.value = false
  }
}

function fireQuick(q: string) {
  command.value = '#lg.' + q
  sendCommand()
}

function restoreHistory(h: Result) {
  command.value = '#lg.' + h.cmd + (h.args ? ' ' + h.args : '')
  lastResult.value = h
}

async function sendPty() {
  const msg = ptyMessage.value.trim()
  if (!msg || ptySending.value) return
  ptySending.value = true
  ptyMessage.value = ''
  try {
    await fetch('/api/debug/send', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ message: msg }),
    })
  } finally {
    ptySending.value = false
  }
}

async function poll() {
  try {
    const r = await fetch('/api/lg/calls')
    if (r.ok) calls.value = await r.json()
  } catch { /* ignore */ }
}

function fmt(ts: string) {
  const d = new Date(ts)
  return d.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit', second: '2-digit' })
}

onMounted(() => { poll(); pollTimer = setInterval(poll, 1500) })
onUnmounted(() => { if (pollTimer) clearInterval(pollTimer) })

const s = {
  root: {
    position: 'fixed', inset: 0, zIndex: 200,
    background: '#080808',
    display: 'flex', flexDirection: 'column',
    fontFamily: "'DM Sans', sans-serif",
  } as Record<string, any>,
  header: {
    display: 'flex', alignItems: 'center', justifyContent: 'space-between',
    padding: '16px 24px',
    borderBottom: '1px solid rgba(255,255,255,0.07)',
    flexShrink: 0,
  },
  title: { fontSize: '14px', fontWeight: 600, color: '#E0E0E0', fontFamily: "'DM Sans',sans-serif" },
  closeBtn: {
    background: 'transparent', border: 'none', cursor: 'pointer',
    color: '#555', fontSize: '16px', padding: '4px 8px', borderRadius: '4px',
  },
  body: {
    flex: 1, display: 'flex', overflow: 'hidden',
  },
  left: {
    width: '520px', flexShrink: 0,
    borderRight: '1px solid rgba(255,255,255,0.06)',
    padding: '20px 24px', overflowY: 'auto',
  },
  right: {
    flex: 1, padding: '20px 24px', display: 'flex', flexDirection: 'column', overflow: 'hidden',
  },
  sectionLabel: {
    fontSize: '10px', fontWeight: 700, letterSpacing: '0.10em', textTransform: 'uppercase',
    color: '#555', fontFamily: "'DM Sans',sans-serif", marginBottom: '12px', display: 'block',
  } as Record<string, any>,
  fieldLabel: {
    fontSize: '11px', color: '#717171', fontFamily: "'DM Sans',sans-serif",
    marginBottom: '5px', fontWeight: 500,
  },
  select: {
    width: '100%', padding: '9px 12px',
    background: '#111', border: '1px solid rgba(255,255,255,0.10)',
    borderRadius: '6px', color: '#E0E0E0', fontSize: '13px',
    fontFamily: "'DM Sans',sans-serif", outline: 'none', cursor: 'pointer',
  },
  hint: {
    fontSize: '11px', color: '#F59E0B', marginTop: '5px',
    fontFamily: "'DM Sans',sans-serif",
  },
  input: {
    flex: 1, background: '#111', border: '1px solid rgba(255,255,255,0.10)',
    borderRadius: '6px', padding: '9px 13px',
    color: '#E0E0E0', fontSize: '13px', outline: 'none',
    fontFamily: "'JetBrains Mono','Courier Prime',monospace",
  },
  sendBtn: (disabled: boolean) => ({
    padding: '9px 18px', borderRadius: '6px', border: 'none',
    background: disabled ? '#1A1A1A' : '#F5C518',
    color: disabled ? '#555' : '#0A0A0A',
    fontSize: '13px', fontWeight: 700, cursor: disabled ? 'not-allowed' : 'pointer',
    fontFamily: "'DM Sans',sans-serif", flexShrink: 0,
  }),
  quickBtn: {
    padding: '5px 10px', borderRadius: '4px',
    background: 'transparent', border: '1px solid rgba(255,255,255,0.08)',
    color: '#9A9A9A', fontSize: '11px', cursor: 'pointer',
    fontFamily: "'JetBrains Mono','Courier Prime',monospace",
    transition: 'all 100ms ease',
  },
  resultBox: {
    background: '#0D0D0D', border: '1px solid rgba(255,255,255,0.08)',
    borderRadius: '8px', padding: '14px 16px',
  },
  pre: {
    margin: 0, color: '#C4C4C4', fontSize: '12px', lineHeight: 1.7,
    fontFamily: "'JetBrains Mono','Courier Prime',monospace",
    whiteSpace: 'pre-wrap', wordBreak: 'break-all' as const,
    maxHeight: '260px', overflowY: 'auto',
  },
  emptyResult: {
    fontSize: '12px', color: '#3D3D3D', fontFamily: "'DM Sans',sans-serif",
    padding: '16px 0',
  },
  elapsed: {
    fontSize: '11px', color: '#555',
    fontFamily: "'JetBrains Mono','Courier Prime',monospace",
  },
  exitCode: {
    fontSize: '11px', color: '#3D3D3D',
    fontFamily: "'JetBrains Mono','Courier Prime',monospace",
  },
  typeBtn: (active: boolean) => ({
    padding: '4px 12px', borderRadius: '4px', border: 'none',
    background: active ? 'rgba(245,197,24,0.12)' : 'rgba(255,255,255,0.04)',
    color: active ? '#F5C518' : '#717171',
    fontSize: '11px', fontWeight: active ? 600 : 400,
    fontFamily: "'DM Sans',sans-serif", cursor: 'pointer',
    letterSpacing: '0.02em',
  }),
  badge: (ok: boolean) => ({
    fontSize: '10px', fontWeight: 700, letterSpacing: '0.05em',
    padding: '2px 7px', borderRadius: '999px',
    background: ok ? 'rgba(74,222,128,0.10)' : 'rgba(248,113,113,0.10)',
    color: ok ? '#4ADE80' : '#F87171',
    fontFamily: "'JetBrains Mono','Courier Prime',monospace",
  }),
  historyItem: {
    display: 'flex', alignItems: 'center', gap: '10px',
    padding: '7px 10px', borderRadius: '5px',
    background: '#0D0D0D', border: '1px solid rgba(255,255,255,0.05)',
    cursor: 'pointer',
  },
  historyCmd: {
    fontFamily: "'JetBrains Mono','Courier Prime',monospace",
    fontSize: '11px', color: '#F5C518', flexShrink: 0,
  },
  historyArgs: { fontSize: '12px', color: '#555', flex: 1, minWidth: 0, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' as const },
  callCount: { fontSize: '11px', color: '#3D3D3D', fontFamily: "'JetBrains Mono','Courier Prime',monospace" },
  callList: { flex: 1, overflowY: 'auto', display: 'flex', flexDirection: 'column', gap: '6px' },
  empty: { fontSize: '12px', color: '#3D3D3D', paddingTop: '4px' },
  callItem: {
    display: 'flex', alignItems: 'baseline', gap: '12px',
    padding: '9px 12px',
    background: '#0D0D0D', border: '1px solid rgba(255,255,255,0.05)',
    borderRadius: '5px',
  },
  callCmd: {
    fontFamily: "'JetBrains Mono','Courier Prime',monospace",
    fontSize: '11px', color: '#F5C518', flexShrink: 0,
  },
  callArgs: { fontSize: '12px', color: '#717171', flex: 1, minWidth: 0, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' as const },
  callTime: {
    fontFamily: "'JetBrains Mono','Courier Prime',monospace",
    fontSize: '10px', color: '#3D3D3D', flexShrink: 0,
  },
}
</script>
