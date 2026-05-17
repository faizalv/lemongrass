<template>
  <div :style="s.root">
    <div :style="s.header">
      <span :style="s.title">Hook Debug</span>
      <button :style="s.closeBtn" @click="$emit('close')">✕</button>
    </div>

    <div :style="s.inputRow">
      <input
        v-model="message"
        :style="s.input"
        placeholder="Type a message for Claude…"
        :disabled="sending"
        @keydown.enter="send"
      />
      <button :style="s.sendBtn" :disabled="sending || !message.trim()" @click="send">
        {{ sending ? '…' : 'Send' }}
      </button>
    </div>

    <div :style="s.callList">
      <div v-if="calls.length === 0" :style="s.empty">
        No calls yet. Send a message and watch Claude echo back via #lg.echo.
      </div>
      <div v-for="(call, i) in [...calls].reverse()" :key="i" :style="s.callItem">
        <span :style="s.callCmd">#lg.{{ call.cmd }}</span>
        <span :style="s.callArgs">{{ call.args }}</span>
        <span :style="s.callTime">{{ fmt(call.timestamp) }}</span>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, onUnmounted } from 'vue'

defineEmits<{ close: [] }>()

interface Call {
  cmd: string
  args: string
  timestamp: string
}

const message = ref('')
const sending = ref(false)
const calls = ref<Call[]>([])
let pollTimer: ReturnType<typeof setInterval> | null = null

async function send() {
  const msg = message.value.trim()
  if (!msg || sending.value) return
  sending.value = true
  message.value = ''
  try {
    await fetch('/api/lg/debug/send', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ message: msg }),
    })
  } finally {
    sending.value = false
  }
}

async function poll() {
  try {
    const r = await fetch('/api/lg/debug/calls')
    if (r.ok) calls.value = await r.json()
  } catch { /* ignore */ }
}

function fmt(ts: string) {
  const d = new Date(ts)
  return d.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit', second: '2-digit' })
}

onMounted(() => {
  poll()
  pollTimer = setInterval(poll, 1500)
})
onUnmounted(() => { if (pollTimer) clearInterval(pollTimer) })

const s = {
  root: {
    position: 'fixed', inset: 0, zIndex: 200,
    background: '#0A0A0A',
    display: 'flex', flexDirection: 'column',
    fontFamily: "'DM Sans', sans-serif",
  } as Record<string, any>,
  header: {
    display: 'flex', alignItems: 'center', justifyContent: 'space-between',
    padding: '18px 24px 14px',
    borderBottom: '1px solid rgba(255,255,255,0.07)',
  },
  title: { fontSize: '15px', fontWeight: 600, color: '#E0E0E0' },
  closeBtn: {
    background: 'transparent', border: 'none', cursor: 'pointer',
    color: '#555', fontSize: '16px', padding: '4px 8px',
    borderRadius: '4px',
  },
  inputRow: {
    display: 'flex', gap: '10px',
    padding: '16px 24px',
    borderBottom: '1px solid rgba(255,255,255,0.06)',
  },
  input: {
    flex: 1,
    background: '#141414', border: '1px solid rgba(255,255,255,0.10)',
    borderRadius: '6px', padding: '9px 13px',
    color: '#E0E0E0', fontSize: '13.5px', outline: 'none',
    fontFamily: "'DM Sans', sans-serif",
  },
  sendBtn: {
    padding: '9px 20px',
    background: '#F5C518', border: 'none', borderRadius: '6px',
    color: '#0A0A0A', fontSize: '13px', fontWeight: 700,
    cursor: 'pointer',
  },
  callList: {
    flex: 1, overflowY: 'auto',
    padding: '16px 24px', display: 'flex', flexDirection: 'column', gap: '8px',
  },
  empty: { fontSize: '13px', color: '#3D3D3D', paddingTop: '8px' },
  callItem: {
    display: 'flex', alignItems: 'baseline', gap: '12px',
    padding: '10px 14px',
    background: '#111', border: '1px solid rgba(255,255,255,0.06)',
    borderRadius: '6px',
  },
  callCmd: {
    fontFamily: "'JetBrains Mono','Courier Prime',monospace",
    fontSize: '11px', color: '#F5C518', flexShrink: 0,
  },
  callArgs: { fontSize: '13px', color: '#C4C4C4', flex: 1, minWidth: 0 },
  callTime: {
    fontFamily: "'JetBrains Mono','Courier Prime',monospace",
    fontSize: '10px', color: '#3D3D3D', flexShrink: 0,
  },
}
</script>
