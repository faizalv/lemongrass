<template>
  <div :style="overlay" @mousedown.self="$emit('close')">
    <div :style="panel">
      <div :style="eyebrowStyle">New workspace</div>
      <div :style="titleStyle">Create workspace</div>

      <label :style="labelStyle">Workspace name</label>
      <input
        v-model="name"
        :style="inputStyle"
        placeholder="e.g. Add idempotency keys to /checkout"
        autofocus
        @keydown.escape="$emit('close')"
      />

      <div v-if="error" :style="errorStyle">{{ error }}</div>

      <div style="display:flex;gap:8px;justify-content:flex-end;margin-top:20px">
        <button :style="btnGhost" @click="$emit('close')">Cancel</button>
        <button :style="btnPrimary(canSubmit)" :disabled="!canSubmit || submitting" @click="submit">
          <Spinner v-if="submitting" :size="13" />
          {{ submitting ? 'Creating…' : 'Create workspace' }}
        </button>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed } from 'vue'
import type { Workspace } from '../../types'
import Spinner from '../grooming/Spinner.vue'

const props = defineProps<{ projectId: string }>()

const emit = defineEmits<{
  'close': []
  'created': [workspace: Workspace]
}>()

const name       = ref('')
const error      = ref('')
const submitting = ref(false)

const canSubmit = computed(() => name.value.trim().length > 0)

async function submit() {
  error.value = ''
  submitting.value = true
  try {
    const res = await fetch('/api/workspaces', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        project_id: parseInt(props.projectId),
        name: name.value.trim(),
      }),
    })
    if (!res.ok) {
      const body = await res.json().catch(() => ({}))
      error.value = body.error ?? `Error ${res.status}`
      return
    }
    const ws = await res.json() as Workspace
    emit('created', ws)
  } catch {
    error.value = 'Network error, please try again.'
  } finally {
    submitting.value = false
  }
}

const overlay = {
  position: 'fixed', inset: 0, background: 'rgba(0,0,0,0.7)',
  backdropFilter: 'blur(6px)', zIndex: 300,
  display: 'flex', alignItems: 'center', justifyContent: 'center',
  padding: '24px', animation: 'lgFadeIn 160ms ease',
}
const panel = {
  background: '#111', border: '1px solid rgba(255,255,255,0.10)',
  borderRadius: '12px', width: '100%', maxWidth: '480px',
  padding: '22px 24px', boxShadow: '0 24px 64px rgba(0,0,0,0.7)',
}
const eyebrowStyle = {
  fontSize: '10px', fontWeight: 700, letterSpacing: '0.12em',
  textTransform: 'uppercase', color: '#F5C518',
  fontFamily: "'DM Sans',sans-serif", marginBottom: '8px',
}
const titleStyle = {
  fontFamily: "'Comfortaa',sans-serif", fontSize: '20px', fontWeight: 700,
  color: '#fff', letterSpacing: '-0.02em', marginBottom: '18px',
}
const labelStyle = {
  display: 'block', fontSize: '11px', fontWeight: 700, letterSpacing: '0.08em',
  textTransform: 'uppercase', color: '#9A9A9A', marginBottom: '7px',
  fontFamily: "'DM Sans',sans-serif",
}
const inputStyle = {
  width: '100%', background: '#0A0A0A', border: '1px solid rgba(255,255,255,0.12)',
  borderRadius: '6px', padding: '10px 12px', color: '#fff',
  fontFamily: "'DM Sans',sans-serif", fontSize: '14px', outline: 'none',
  boxSizing: 'border-box' as const,
}
const errorStyle = {
  marginTop: '10px', fontSize: '12px', color: '#F87171',
  fontFamily: "'DM Sans',sans-serif",
}
const btnGhost = {
  padding: '9px 16px', borderRadius: '6px',
  background: 'transparent', color: '#B0B0B0',
  border: '1px solid rgba(255,255,255,0.12)', cursor: 'pointer',
  fontFamily: "'DM Sans',sans-serif", fontWeight: 500, fontSize: '13px',
}
const btnPrimary = (enabled: boolean) => ({
  display: 'inline-flex', alignItems: 'center', gap: '8px',
  padding: '9px 16px', borderRadius: '6px',
  background: enabled ? '#F5C518' : '#2A2A2A',
  color: enabled ? '#0A0A0A' : '#555',
  border: 'none', cursor: enabled ? 'pointer' : 'not-allowed',
  fontFamily: "'DM Sans',sans-serif", fontWeight: 700, fontSize: '13px',
})
</script>
