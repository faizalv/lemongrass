<template>
  <div :style="overlay" @mousedown.self="onBackdropClick">
    <div :style="panel">

      <!-- Header -->
      <div :style="header">
        <div :style="iconWrap">
          <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="#0A0A0A" stroke-width="2.2" stroke-linecap="round" stroke-linejoin="round">
            <path d="M11 20A7 7 0 0 1 9.8 6.1C15.5 5 17 4.48 19.2 2.2c1.7 6.6.2 13.1-5 17.7"/>
            <path d="M2 21c0-3 1.85-5.36 5.08-6"/>
          </svg>
        </div>
        <div>
          <div :style="eyebrow">Get started</div>
          <div :style="titleStyle">Add a project</div>
        </div>
      </div>

      <!-- LOADING -->
      <div v-if="phase === 'loading'" :style="centeredBody">
        <div class="spin" :style="spinner"></div>
        <span :style="hint">Scanning filesystem…</span>
      </div>

      <!-- BROWSING -->
      <div v-else-if="phase === 'browsing' || phase === 'adding'" :style="body">
        <div v-if="fetchError" :style="errBox">
          <span>{{ fetchError }}</span>
          <button :style="retryBtn" @click="loadTree()">Retry</button>
        </div>

        <template v-else>
          <div :style="treeHeader">
            <span :style="treeLabel">Filesystem</span>
            <button
              :style="refreshBtn"
              :disabled="refreshing || phase === 'adding'"
              @click="loadTree(true)"
            >
              <svg
                :class="refreshing ? 'spin' : ''"
                width="12" height="12" viewBox="0 0 24 24" fill="none"
                stroke="currentColor" stroke-width="2.2"
                stroke-linecap="round" stroke-linejoin="round"
              >
                <path d="M3 12a9 9 0 0 1 9-9 9.75 9.75 0 0 1 6.74 2.74L21 8"/>
                <path d="M21 3v5h-5"/>
                <path d="M21 12a9 9 0 0 1-9 9 9.75 9.75 0 0 1-6.74-2.74L3 16"/>
                <path d="M8 16H3v5"/>
              </svg>
              {{ refreshing ? 'Scanning…' : 'Refresh' }}
            </button>
          </div>

          <div :style="treeWrap">
            <FolderNode
              v-for="node in tree"
              :key="node.path"
              :node="node"
              :selected-path="selectedPath"
              @select="selectedPath = $event"
            />
          </div>

          <div :style="selectionBar">
            <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="#555" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" style="flex-shrink:0">
              <path d="M22 19a2 2 0 0 1-2 2H4a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h5l2 3h9a2 2 0 0 1 2 2z"/>
            </svg>
            <span :style="selectionText">
              {{ selectedPath || 'Select a folder above' }}
            </span>
          </div>
        </template>
      </div>

      <!-- RESTARTING -->
      <div v-else-if="phase === 'restarting'" :style="centeredBody">
        <div class="spin" :style="{ ...spinner, borderTopColor: '#60A5FA' }"></div>
        <span :style="hint">Server is restarting…</span>
        <span :style="subHint">This takes about 10–15 seconds</span>
      </div>

      <!-- ERROR -->
      <div v-else-if="phase === 'error'" :style="centeredBody">
        <svg width="32" height="32" viewBox="0 0 24 24" fill="none" stroke="#F87171" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round">
          <circle cx="12" cy="12" r="10"/>
          <line x1="12" y1="8" x2="12" y2="12"/>
          <line x1="12" y1="16" x2="12.01" y2="16"/>
        </svg>
        <span :style="{ ...hint, color: '#F87171' }">{{ restartError }}</span>
        <button :style="retryBtn" @click="$emit('close')">Close</button>
      </div>

      <!-- Footer -->
      <div v-if="phase === 'browsing' || phase === 'adding'" :style="footer">
        <button :style="btnGhost" @click="$emit('close')" :disabled="phase === 'adding'">Cancel</button>
        <button
          :style="btnPrimary(!!selectedPath && phase === 'browsing')"
          :disabled="!selectedPath || phase === 'adding'"
          @click="addProject"
        >
          <svg v-if="phase === 'adding'" class="spin" width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round">
            <path d="M21 12a9 9 0 1 1-6.219-8.56"/>
          </svg>
          <svg v-else width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.2" stroke-linecap="round" stroke-linejoin="round">
            <path d="M22 19a2 2 0 0 1-2 2H4a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h5l2 3h9a2 2 0 0 1 2 2z"/>
          </svg>
          {{ phase === 'adding' ? 'Adding…' : 'Add project' }}
        </button>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import type { FsNode, FsProject } from '../../types'
import FolderNode from '../FolderNode.vue'

const emit = defineEmits<{
  close: []
  added: [projects: FsProject[]]
}>()

type Phase = 'loading' | 'browsing' | 'adding' | 'restarting' | 'error'

const phase = ref<Phase>('loading')
const tree = ref<FsNode[]>([])
const selectedPath = ref('')
const fetchError = ref('')
const restartError = ref('')
const refreshing = ref(false)

onMounted(() => loadTree())

async function loadTree(force = false) {
  fetchError.value = ''
  if (force) {
    refreshing.value = true
  } else {
    phase.value = 'loading'
  }
  try {
    const url = force ? '/api/fs/browse?refresh=true' : '/api/fs/browse'
    const r = await fetch(url)
    if (!r.ok) throw new Error(`Server returned ${r.status}`)
    tree.value = await r.json()
    if (!force) phase.value = 'browsing'
  } catch (e: any) {
    fetchError.value = e?.message ?? 'Failed to load filesystem'
    if (!force) phase.value = 'browsing'
  } finally {
    refreshing.value = false
  }
}

async function addProject() {
  if (!selectedPath.value || phase.value !== 'browsing') return
  phase.value = 'adding'

  try {
    const r = await fetch('/api/fs/projects', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ path: selectedPath.value }),
    })
    if (r.status !== 202) throw new Error(`Unexpected status ${r.status}`)
  } catch (e: any) {
    restartError.value = e?.message ?? 'Add project request failed'
    phase.value = 'error'
    return
  }

  phase.value = 'restarting'
  try {
    await pollHealth(60_000)
    const r = await fetch('/api/fs/projects')
    if (!r.ok) throw new Error('Failed to load projects')
    const projects: FsProject[] = await r.json()
    emit('added', projects)
  } catch (e: any) {
    restartError.value = e?.message ?? 'Server did not come back in time'
    phase.value = 'error'
  }
}

async function pollHealth(timeoutMs: number) {
  const deadline = Date.now() + timeoutMs
  while (Date.now() < deadline) {
    await delay(2500)
    try {
      const r = await fetch('/api/health')
      if (r.ok) return
    } catch { /* server is down, keep polling */ }
  }
  throw new Error('Server did not come back within 60 seconds')
}

function delay(ms: number) {
  return new Promise(resolve => setTimeout(resolve, ms))
}

function onBackdropClick() {
  if (phase.value === 'restarting' || phase.value === 'adding') return
  emit('close')
}

// ── Styles ──────────────────────────────────────────────────────────────────

const overlay = {
  position: 'fixed', inset: 0, background: 'rgba(0,0,0,0.75)',
  backdropFilter: 'blur(8px)', zIndex: 300,
  display: 'flex', alignItems: 'center', justifyContent: 'center',
  padding: '24px', animation: 'lgFadeIn 160ms ease',
}
const panel = {
  background: '#111', border: '1px solid rgba(255,255,255,0.10)',
  borderRadius: '14px', width: '100%', maxWidth: '520px',
  boxShadow: '0 32px 80px rgba(0,0,0,0.8)',
  display: 'flex', flexDirection: 'column', overflow: 'hidden',
}
const header = {
  padding: '22px 24px 18px',
  borderBottom: '1px solid rgba(255,255,255,0.06)',
  display: 'flex', alignItems: 'center', gap: '14px',
}
const iconWrap = {
  width: '40px', height: '40px', borderRadius: '10px',
  background: '#F5C518', flexShrink: 0,
  display: 'flex', alignItems: 'center', justifyContent: 'center',
}
const eyebrow = {
  fontSize: '10px', fontWeight: 700, letterSpacing: '0.12em',
  textTransform: 'uppercase', color: '#F5C518',
  fontFamily: "'DM Sans',sans-serif", marginBottom: '3px',
}
const titleStyle = {
  fontFamily: "'Comfortaa',sans-serif", fontSize: '20px', fontWeight: 700,
  color: '#fff', letterSpacing: '-0.02em',
}
const body = { padding: '16px 20px 12px' }
const centeredBody = {
  padding: '40px 24px', display: 'flex', flexDirection: 'column',
  alignItems: 'center', gap: '12px',
}
const spinner = {
  width: '28px', height: '28px', borderRadius: '50%',
  border: '2.5px solid rgba(255,255,255,0.08)',
  borderTopColor: '#F5C518',
}
const hint = {
  fontSize: '14px', fontFamily: "'DM Sans',sans-serif",
  color: '#9A9A9A', fontWeight: 500,
}
const subHint = {
  fontSize: '12px', fontFamily: "'DM Sans',sans-serif",
  color: '#555', marginTop: '-4px',
}
const treeHeader = {
  display: 'flex', alignItems: 'center', justifyContent: 'space-between',
  marginBottom: '6px',
}
const treeLabel = {
  fontSize: '10px', fontWeight: 700, letterSpacing: '0.10em',
  textTransform: 'uppercase', color: '#3D3D3D',
  fontFamily: "'DM Sans',sans-serif",
}
const refreshBtn = {
  display: 'inline-flex', alignItems: 'center', gap: '5px',
  padding: '4px 10px', borderRadius: '5px', border: 'none',
  background: 'rgba(255,255,255,0.06)', color: '#9A9A9A',
  cursor: 'pointer', fontSize: '11px', fontFamily: "'DM Sans',sans-serif",
  fontWeight: 500, transition: 'background 120ms',
}
const treeWrap = {
  background: '#0A0A0A', border: '1px solid rgba(255,255,255,0.08)',
  borderRadius: '8px', padding: '8px', overflowY: 'auto',
  maxHeight: '320px', minHeight: '120px',
}
const selectionBar = {
  marginTop: '10px', display: 'flex', alignItems: 'center', gap: '8px',
  padding: '8px 10px',
  background: '#0A0A0A', border: '1px solid rgba(255,255,255,0.06)',
  borderRadius: '6px',
}
const selectionText = {
  flex: 1, fontSize: '12px', fontFamily: "'JetBrains Mono','Courier Prime',monospace",
  color: '#9A9A9A', whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis',
}
const errBox = {
  display: 'flex', alignItems: 'center', justifyContent: 'space-between',
  gap: '12px', padding: '10px 12px',
  background: 'rgba(248,113,113,0.06)', border: '1px solid rgba(248,113,113,0.20)',
  borderRadius: '7px', fontSize: '13px', color: '#F87171',
  fontFamily: "'DM Sans',sans-serif",
}
const footer = {
  padding: '14px 20px',
  borderTop: '1px solid rgba(255,255,255,0.06)',
  display: 'flex', gap: '8px', justifyContent: 'flex-end',
}
const btnGhost = {
  padding: '9px 16px', borderRadius: '6px',
  background: 'transparent', color: '#B0B0B0',
  border: '1px solid rgba(255,255,255,0.12)', cursor: 'pointer',
  fontFamily: "'DM Sans',sans-serif", fontWeight: 500, fontSize: '13px',
}
const btnPrimary = (enabled: boolean) => ({
  display: 'inline-flex', alignItems: 'center', gap: '7px',
  padding: '9px 18px', borderRadius: '6px',
  background: enabled ? '#F5C518' : '#1E1E1E',
  color: enabled ? '#0A0A0A' : '#444',
  border: 'none', cursor: enabled ? 'pointer' : 'not-allowed',
  fontFamily: "'DM Sans',sans-serif", fontWeight: 700, fontSize: '13px',
  transition: 'background 150ms',
})
const retryBtn = {
  padding: '5px 12px', borderRadius: '5px', border: 'none',
  background: 'rgba(255,255,255,0.08)', color: '#D4D4D4',
  cursor: 'pointer', fontSize: '12px', fontFamily: "'DM Sans',sans-serif",
  flexShrink: 0,
}
</script>
