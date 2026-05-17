<template>
  <div :style="overlay" @mousedown.self="$emit('close')">
    <div :style="panel">
      <div :style="eyebrowStyle">Workspaces are pinned to a commit</div>
      <div :style="titleStyle">New workspace</div>
      <div :style="bodyText">
        Each workspace is a self-contained task — its own grooming, plan, and patches.
        It'll pin to the current commit of
        <code :style="codeStyle">{{ branch }}</code>.
      </div>
      <label :style="labelStyle">Workspace name</label>
      <input
        v-model="name"
        :style="inputStyle"
        placeholder="e.g. Add idempotency keys to /checkout"
        autofocus
        @keydown.enter="submit"
        @keydown.escape="$emit('close')"
      />
      <div style="display:flex;gap:8px;justify-content:flex-end;margin-top:16px">
        <button :style="btnGhost" @click="$emit('close')">Cancel</button>
        <button :style="btnPrimary(!!name.trim())" :disabled="!name.trim()" @click="submit">
          Create workspace
        </button>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref } from 'vue'

defineProps<{ branch: string }>()

const emit = defineEmits<{
  'close': []
  'create': [name: string]
}>()

const name = ref('')

function submit() {
  const n = name.value.trim()
  if (!n) { emit('close'); return }
  emit('create', n)
  name.value = ''
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
  color: '#fff', letterSpacing: '-0.02em', marginBottom: '14px',
}
const bodyText = {
  fontSize: '13px', color: '#9A9A9A', marginBottom: '14px',
  fontFamily: "'DM Sans',sans-serif", lineHeight: 1.6,
}
const codeStyle = {
  fontFamily: "'JetBrains Mono','Courier Prime',monospace",
  color: '#F5C518', fontSize: '12px',
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
}
const btnGhost = {
  padding: '8px 14px', borderRadius: '6px',
  background: 'transparent', color: '#B0B0B0',
  border: '1px solid rgba(255,255,255,0.12)', cursor: 'pointer',
  fontFamily: "'DM Sans',sans-serif", fontWeight: 500, fontSize: '13px',
}
const btnPrimary = (enabled: boolean) => ({
  padding: '8px 14px', borderRadius: '6px',
  background: enabled ? '#F5C518' : '#2A2A2A',
  color: enabled ? '#0A0A0A' : '#555',
  border: 'none', cursor: enabled ? 'pointer' : 'not-allowed',
  fontFamily: "'DM Sans',sans-serif", fontWeight: 700, fontSize: '13px',
})
</script>
