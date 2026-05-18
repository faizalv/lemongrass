<template>
  <div :style="overlay" @mousedown.self="$emit('close')">
    <div :style="panel">

      <div :style="header">
        <div :style="iconWrap">
          <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="#0A0A0A" stroke-width="2.2" stroke-linecap="round" stroke-linejoin="round">
            <polyline points="3 6 5 6 21 6"/>
            <path d="M19 6l-1 14a2 2 0 0 1-2 2H8a2 2 0 0 1-2-2L5 6"/>
            <path d="M10 11v6"/>
            <path d="M14 11v6"/>
            <path d="M9 6V4a1 1 0 0 1 1-1h4a1 1 0 0 1 1 1v2"/>
          </svg>
        </div>
        <div>
          <div :style="eyebrow">Destructive action</div>
          <div :style="titleStyle">Remove project</div>
        </div>
      </div>

      <div :style="body">
        <p :style="desc">
          Remove <strong style="color:#E0E0E0">{{ projectName }}</strong> from Lemongrass?
          All associated workspaces will be closed.
        </p>
        <p :style="note">Files on disk are not affected.</p>

        <div v-if="error" :style="errBox">{{ error }}</div>
      </div>

      <div :style="footer">
        <button :style="btnGhost" :disabled="deleting" @click="$emit('close')">Cancel</button>
        <button :style="btnDanger(deleting)" :disabled="deleting" @click="confirm">
          <svg v-if="deleting" class="spin" width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round">
            <path d="M21 12a9 9 0 1 1-6.219-8.56"/>
          </svg>
          {{ deleting ? 'Removing…' : 'Remove project' }}
        </button>
      </div>

    </div>
  </div>
</template>

<script setup lang="ts">
import { ref } from 'vue'

const props = defineProps<{ projectId: string; projectName: string }>()

const emit = defineEmits<{
  close: []
  deleted: [id: string]
}>()

const deleting = ref(false)
const error = ref('')

async function confirm() {
  deleting.value = true
  error.value = ''
  try {
    const r = await fetch(`/api/fs/projects/${props.projectId}`, { method: 'DELETE' })
    if (!r.ok) throw new Error(`Server returned ${r.status}`)
    emit('deleted', props.projectId)
  } catch (e: any) {
    error.value = e?.message ?? 'Failed to remove project'
    deleting.value = false
  }
}

const overlay = {
  position: 'fixed', inset: 0, background: 'rgba(0,0,0,0.75)',
  backdropFilter: 'blur(8px)', zIndex: 300,
  display: 'flex', alignItems: 'center', justifyContent: 'center',
  padding: '24px', animation: 'lgFadeIn 160ms ease',
}
const panel = {
  background: '#111', border: '1px solid rgba(255,255,255,0.10)',
  borderRadius: '14px', width: '100%', maxWidth: '420px',
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
  background: '#F87171', flexShrink: 0,
  display: 'flex', alignItems: 'center', justifyContent: 'center',
}
const eyebrow = {
  fontSize: '10px', fontWeight: 700, letterSpacing: '0.12em',
  textTransform: 'uppercase', color: '#F87171',
  fontFamily: "'DM Sans',sans-serif", marginBottom: '3px',
}
const titleStyle = {
  fontFamily: "'Comfortaa',sans-serif", fontSize: '20px', fontWeight: 700,
  color: '#fff', letterSpacing: '-0.02em',
}
const body = { padding: '20px 24px 8px' }
const desc = {
  fontFamily: "'DM Sans',sans-serif", fontSize: '14px', color: '#9A9A9A',
  margin: '0 0 8px', lineHeight: 1.6,
}
const note = {
  fontFamily: "'DM Sans',sans-serif", fontSize: '12px', color: '#4A4A4A',
  margin: 0, lineHeight: 1.5,
}
const errBox = {
  marginTop: '14px', padding: '10px 12px',
  background: 'rgba(248,113,113,0.06)', border: '1px solid rgba(248,113,113,0.20)',
  borderRadius: '7px', fontSize: '13px', color: '#F87171',
  fontFamily: "'DM Sans',sans-serif",
}
const footer = {
  padding: '18px 24px',
  borderTop: '1px solid rgba(255,255,255,0.06)',
  display: 'flex', gap: '8px', justifyContent: 'flex-end',
}
const btnGhost = {
  padding: '9px 16px', borderRadius: '6px',
  background: 'transparent', color: '#B0B0B0',
  border: '1px solid rgba(255,255,255,0.12)', cursor: 'pointer',
  fontFamily: "'DM Sans',sans-serif", fontWeight: 500, fontSize: '13px',
}
const btnDanger = (loading: boolean) => ({
  display: 'inline-flex', alignItems: 'center', gap: '7px',
  padding: '9px 18px', borderRadius: '6px',
  background: loading ? '#7F1D1D' : '#EF4444',
  color: '#fff', border: 'none',
  cursor: loading ? 'not-allowed' : 'pointer',
  fontFamily: "'DM Sans',sans-serif", fontWeight: 700, fontSize: '13px',
  transition: 'background 150ms',
})
</script>
