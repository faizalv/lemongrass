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
            placeholder="#lg.recon.peek <dir>  |  #lg.recon.read <path:symbol:kind>"
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
          <!-- parsed peek view -->
          <div v-if="peekFiles.length > 0" :style="s.peekScroll">
            <div v-for="f in peekFiles" :key="f.path" style="margin-bottom:12px">
              <div :style="s.peekFile">{{ f.path }}</div>
              <div
                v-for="row in f.rows"
                :key="row.kind + row.symbol"
                :style="s.peekRow"
              >
                <span :style="s.peekKind(row.kind)">{{ row.kind }}</span>
                <span :style="s.peekSym">{{ row.symbol }}</span>
                <span :style="s.peekLines">{{ row.lines }}</span>
                <span v-if="row.status" :style="s.peekStatus(row.status)">{{ row.status }}</span>
                <span style="margin-left:auto;display:flex;gap:4px;flex-shrink:0">
                  <button :style="s.drillBtn" @click="peekDrillRead(row)" title="read">read</button>
                  <button :style="s.drillBtn" @click="peekDrillRelated(row)" title="related">related</button>
                </span>
              </div>
            </div>
          </div>
          <pre v-else :style="s.pre">{{ lastResult.text }}</pre>
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

const quickCommands = ['recon.tree', 'recon.peek modules/', 'recon.search handlers', 'echo ping']

const testableWorkspaces = computed(() =>
  props.workspaces.filter(w => w.id !== 'reconnaissance')
)

const selectedWorkspace = computed(() =>
  testableWorkspaces.value.find(w => w.id === selectedWorkspaceId.value)
)

const sessionType = ref<'grooming' | 'execution'>('grooming')

interface PeekRow {
  filePath: string
  kind: string
  symbol: string
  rawSymbol: string  // symbol name without receiver prefix
  lines: string
  status: string
}

interface PeekFile {
  path: string
  rows: PeekRow[]
}

const peekFiles = computed<PeekFile[]>(() => {
  if (!lastResult.value || lastResult.value.cmd !== 'recon.peek') return []
  const lines = lastResult.value.text.split('\n')
  const files: PeekFile[] = []
  let current: PeekFile | null = null
  // peek output lines: file headers have no leading spaces; symbol rows start with 2 spaces
  for (const line of lines) {
    if (!line.startsWith(' ') && line.trim() !== '') {
      current = { path: line.trim(), rows: [] }
      files.push(current)
    } else if (current && line.startsWith('  ')) {
      // "  kind     symbol                                       start-end   ?marker"
      const parts = line.trim().split(/\s+/)
      if (parts.length < 3) continue
      const kind = parts[0]
      const displaySymbol = parts[1]
      const lines = parts[2]
      const status = parts[3] ?? ''
      // rawSymbol: strip Receiver. prefix for method nodes
      const rawSymbol = displaySymbol.includes('.') ? displaySymbol.split('.').pop()! : displaySymbol
      current.rows.push({ filePath: current.path, kind, symbol: displaySymbol, rawSymbol, lines, status })
    }
  }
  return files
})

interface HookParsed { text: string; decision: 'allow' | 'deny' | 'raw' }

function parseHookOutput(raw: string): HookParsed {
  try {
    const parsed = JSON.parse(raw)
    const hso = parsed?.hookSpecificOutput
    if (!hso) return { text: raw, decision: 'raw' }
    if (hso.permissionDecision === 'allow') {
      const cmd: string = hso.updatedInput?.command ?? ''
      const prefix = "printf '%s' "
      if (!cmd.startsWith(prefix)) return { text: raw, decision: 'allow' }
      const escaped = cmd.slice(prefix.length)
      const text = escaped.replace(/'\\''/g, "'").replace(/^'|'$/g, '')
      return { text, decision: 'allow' }
    }
    if (hso.permissionDecision === 'deny') {
      return { text: hso.permissionDecisionReason ?? '', decision: 'deny' }
    }
  } catch { /* not JSON, return as-is */ }
  return { text: raw, decision: 'raw' }
}

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
        project_id: selectedWorkspace.value?.project_id ? Number(selectedWorkspace.value.project_id) : 0,
        command: raw,
        session_type: sessionType.value,
      }),
    })
    const body = await r.json().catch(() => ({ output: '', exit_code: -1 }))
    const ms = Date.now() - t0
    const exitCode: number = body.exit_code ?? -1
    const { text, decision } = parseHookOutput(body.output || body.error || '')
    const ok = decision !== 'deny' && exitCode !== -1 && !text.startsWith('error:')
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

function peekDrillRead(row: PeekRow) {
  command.value = `#lg.recon.read ${row.filePath}:${row.rawSymbol}:${row.kind}`
}

function peekDrillRelated(row: PeekRow) {
  command.value = `#lg.recon.related ${row.filePath}:${row.rawSymbol}:${row.kind}`
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
    background: 'var(--color-surface-0)',
    display: 'flex', flexDirection: 'column',
    fontFamily: 'var(--font-body)',
  } as Record<string, any>,
  header: {
    display: 'flex', alignItems: 'center', justifyContent: 'space-between',
    padding: '16px 24px',
    borderBottom: '1px solid rgba(255,255,255,0.07)',
    flexShrink: 0,
  },
  title: { fontSize: '14px', fontWeight: 600, color: 'var(--color-gray-100)', fontFamily: 'var(--font-body)' },
  closeBtn: {
    background: 'transparent', border: 'none', cursor: 'pointer',
    color: 'var(--color-gray-500)', fontSize: '16px', padding: '4px 8px', borderRadius: '4px',
  },
  body: {
    flex: 1, display: 'flex', overflow: 'hidden',
  },
  left: {
    width: '520px', flexShrink: 0,
    borderRight: '1px solid rgba(255,255,255,0.06)',
    padding: '20px 24px', overflowY: 'auto',
  } as Record<string, any>,
  right: {
    flex: 1, padding: '20px 24px', display: 'flex', flexDirection: 'column', overflow: 'hidden',
  } as Record<string, any>,
  sectionLabel: {
    fontSize: '10px', fontWeight: 700, letterSpacing: '0.10em', textTransform: 'uppercase',
    color: 'var(--color-gray-500)', fontFamily: 'var(--font-body)', marginBottom: '12px', display: 'block',
  } as Record<string, any>,
  fieldLabel: {
    fontSize: '11px', color: 'var(--color-gray-400)', fontFamily: 'var(--font-body)',
    marginBottom: '5px', fontWeight: 500,
  },
  select: {
    width: '100%', padding: '9px 12px',
    background: 'var(--color-gray-900)', border: '1px solid rgba(255,255,255,0.10)',
    borderRadius: '6px', color: 'var(--color-gray-100)', fontSize: '13px',
    fontFamily: 'var(--font-body)', outline: 'none', cursor: 'pointer',
  },
  hint: {
    fontSize: '11px', color: '#F59E0B', marginTop: '5px',
    fontFamily: 'var(--font-body)',
  },
  input: {
    flex: 1, background: 'var(--color-gray-900)', border: '1px solid rgba(255,255,255,0.10)',
    borderRadius: '6px', padding: '9px 13px',
    color: 'var(--color-gray-100)', fontSize: '13px', outline: 'none',
    fontFamily: 'var(--font-mono)',
  },
  sendBtn: (disabled: boolean) => ({
    padding: '9px 18px', borderRadius: '6px', border: 'none',
    background: disabled ? 'var(--color-gray-800)' : 'var(--color-amber)',
    color: disabled ? 'var(--color-gray-500)' : 'var(--color-surface-0)',
    fontSize: '13px', fontWeight: 700, cursor: disabled ? 'not-allowed' : 'pointer',
    fontFamily: 'var(--font-body)', flexShrink: 0,
  }),
  quickBtn: {
    padding: '5px 10px', borderRadius: '4px',
    background: 'transparent', border: '1px solid rgba(255,255,255,0.08)',
    color: 'var(--color-gray-300)', fontSize: '11px', cursor: 'pointer',
    fontFamily: 'var(--font-mono)',
    transition: 'all 100ms ease',
  },
  resultBox: {
    background: 'var(--color-surface-0)', border: '1px solid rgba(255,255,255,0.08)',
    borderRadius: '8px', padding: '14px 16px',
  },
  pre: {
    margin: 0, color: 'var(--color-gray-100)', fontSize: '12px', lineHeight: 1.7,
    fontFamily: 'var(--font-mono)',
    whiteSpace: 'pre-wrap', wordBreak: 'break-all' as const,
    maxHeight: '260px', overflowY: 'auto',
  } as Record<string, any>,
  emptyResult: {
    fontSize: '12px', color: 'var(--color-gray-600)', fontFamily: 'var(--font-body)',
    padding: '16px 0',
  },
  elapsed: {
    fontSize: '11px', color: 'var(--color-gray-500)',
    fontFamily: 'var(--font-mono)',
  },
  exitCode: {
    fontSize: '11px', color: 'var(--color-gray-600)',
    fontFamily: 'var(--font-mono)',
  },
  typeBtn: (active: boolean) => ({
    padding: '4px 12px', borderRadius: '4px', border: 'none',
    background: active ? 'rgba(245,197,24,0.12)' : 'rgba(255,255,255,0.04)',
    color: active ? 'var(--color-amber)' : 'var(--color-gray-400)',
    fontSize: '11px', fontWeight: active ? 600 : 400,
    fontFamily: 'var(--font-body)', cursor: 'pointer',
    letterSpacing: '0.02em',
  }),
  badge: (ok: boolean) => ({
    fontSize: '10px', fontWeight: 700, letterSpacing: '0.05em',
    padding: '2px 7px', borderRadius: '999px',
    background: ok ? 'rgba(74,222,128,0.10)' : 'rgba(248,113,113,0.10)',
    color: ok ? 'var(--color-success)' : 'var(--color-error)',
    fontFamily: 'var(--font-mono)',
  }),
  historyItem: {
    display: 'flex', alignItems: 'center', gap: '10px',
    padding: '7px 10px', borderRadius: '5px',
    background: 'var(--color-surface-0)', border: '1px solid rgba(255,255,255,0.05)',
    cursor: 'pointer',
  },
  historyCmd: {
    fontFamily: 'var(--font-mono)',
    fontSize: '11px', color: 'var(--color-amber)', flexShrink: 0,
  },
  historyArgs: { fontSize: '12px', color: 'var(--color-gray-500)', flex: 1, minWidth: 0, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' as const },
  callCount: { fontSize: '11px', color: 'var(--color-gray-600)', fontFamily: 'var(--font-mono)' },
  callList: { flex: 1, overflowY: 'auto', display: 'flex', flexDirection: 'column', gap: '6px' } as Record<string, any>,
  empty: { fontSize: '12px', color: 'var(--color-gray-600)', paddingTop: '4px' },
  callItem: {
    display: 'flex', alignItems: 'baseline', gap: '12px',
    padding: '9px 12px',
    background: 'var(--color-surface-0)', border: '1px solid rgba(255,255,255,0.05)',
    borderRadius: '5px',
  },
  callCmd: {
    fontFamily: 'var(--font-mono)',
    fontSize: '11px', color: 'var(--color-amber)', flexShrink: 0,
  },
  callArgs: { fontSize: '12px', color: 'var(--color-gray-400)', flex: 1, minWidth: 0, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' as const },
  callTime: {
    fontFamily: 'var(--font-mono)',
    fontSize: '10px', color: 'var(--color-gray-600)', flexShrink: 0,
  },
  peekScroll: {
    maxHeight: '260px', overflowY: 'auto',
  } as Record<string, any>,
  peekFile: {
    fontFamily: 'var(--font-mono)',
    fontSize: '11px', color: 'var(--color-gray-400)', marginBottom: '4px', paddingBottom: '2px',
    borderBottom: '1px solid rgba(255,255,255,0.04)',
  },
  peekRow: {
    display: 'flex', alignItems: 'center', gap: '10px',
    padding: '3px 4px', borderRadius: '3px',
    cursor: 'default',
  } as Record<string, any>,
  peekKind: (kind: string) => ({
    fontFamily: 'var(--font-mono)',
    fontSize: '10px', width: '52px', flexShrink: 0,
    color: kind === 'imports' ? 'var(--color-gray-500)' : kind === 'method' || kind === 'func' ? 'var(--color-info)' : 'var(--color-violet)',
  }),
  peekSym: {
    fontFamily: 'var(--font-mono)',
    fontSize: '11px', color: 'var(--color-gray-100)', flex: 1, minWidth: 0,
    overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' as const,
  },
  peekLines: {
    fontFamily: 'var(--font-mono)',
    fontSize: '10px', color: 'var(--color-gray-600)', flexShrink: 0,
  },
  peekStatus: (s: string) => ({
    fontFamily: 'var(--font-mono)',
    fontSize: '10px', flexShrink: 0,
    color: s.startsWith('*') ? '#F59E0B' : 'var(--color-gray-500)',
  }),
  drillBtn: {
    padding: '1px 6px', borderRadius: '3px', border: '1px solid rgba(255,255,255,0.07)',
    background: 'transparent', color: 'var(--color-gray-500)', fontSize: '10px',
    fontFamily: 'var(--font-body)', cursor: 'pointer',
  },
}
</script>
