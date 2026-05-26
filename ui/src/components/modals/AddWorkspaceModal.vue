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

      <label :style="{ ...labelStyle, marginTop: '16px' }">Requirement</label>

      <!-- Tab switcher -->
      <div :style="tabBar">
        <button :style="tab(activeTab === 'text')" @click="activeTab = 'text'">Text</button>
        <button :style="tab(activeTab === 'file')" @click="activeTab = 'file'">File</button>
      </div>

      <!-- Text tab -->
      <textarea
        v-if="activeTab === 'text'"
        v-model="requirementText"
        :style="textareaStyle"
        placeholder="Describe what needs to be built or changed…"
        rows="6"
      />

      <!-- File tab -->
      <div
        v-else
        :style="dropZone(dragging)"
        @dragover.prevent="dragging = true"
        @dragleave.prevent="dragging = false"
        @drop.prevent="onDrop"
      >
        <template v-if="selectedFile">
          <AppIcon name="file" :size="20" color="#F5C518" />
          <div style="font-family:'DM Sans',sans-serif;font-size:13px;color:#E0E0E0;margin-top:6px">{{ selectedFile.name }}</div>
          <div style="font-family:'JetBrains Mono',monospace;font-size:11px;color:#717171;margin-top:2px">{{ formatSize(selectedFile.size) }}</div>
          <button :style="clearFileBtn" @click.stop="selectedFile = null">Remove</button>
        </template>
        <template v-else>
          <AppIcon name="upload" :size="20" color="#555" />
          <div style="font-family:'DM Sans',sans-serif;font-size:13px;color:#717171;margin-top:8px">Drop a file here or</div>
          <button :style="browseBtn" @click="fileInput?.click()">Browse</button>
          <div style="font-family:'DM Sans',sans-serif;font-size:11px;color:#3D3D3D;margin-top:6px">PDF, PNG, JPG, WEBP, GIF</div>
        </template>
        <input ref="fileInput" type="file" :accept="acceptTypes" style="display:none" @change="onFileChange" />
      </div>

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
import AppIcon from '../AppIcon.vue'
import Spinner from '../grooming/Spinner.vue'

const props = defineProps<{ projectId: string }>()

const emit = defineEmits<{
  'close': []
  'created': [workspace: Workspace]
}>()

const name           = ref('')
const activeTab      = ref<'text' | 'file'>('text')
const requirementText = ref('')
const selectedFile   = ref<File | null>(null)
const dragging       = ref(false)
const fileInput      = ref<HTMLInputElement | null>(null)
const error          = ref('')
const submitting     = ref(false)

const acceptTypes = 'application/pdf,image/png,image/jpeg,image/webp,image/gif'

const allowedTypes = new Set([
  'application/pdf',
  'image/png',
  'image/jpeg',
  'image/webp',
  'image/gif',
])

const canSubmit = computed(() => {
  const n = name.value.trim()
  if (!n) return false
  if (activeTab.value === 'text') return requirementText.value.trim().length > 0
  return selectedFile.value !== null
})

function onDrop(e: DragEvent) {
  dragging.value = false
  const file = e.dataTransfer?.files[0]
  if (file) setFile(file)
}

function onFileChange(e: Event) {
  const file = (e.target as HTMLInputElement).files?.[0]
  if (file) setFile(file)
}

function setFile(file: File) {
  error.value = ''
  if (!allowedTypes.has(file.type)) {
    error.value = `Unsupported file type. Use PDF, PNG, JPG, WEBP, or GIF.`
    return
  }
  selectedFile.value = file
}

function formatSize(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`
  return `${(bytes / (1024 * 1024)).toFixed(1)} MB`
}

async function submit() {
  error.value = ''
  submitting.value = true
  try {
    let res: Response
    if (activeTab.value === 'text') {
      res = await fetch('/api/workspaces', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          project_id: parseInt(props.projectId),
          name: name.value.trim(),
          requirement: requirementText.value,
        }),
      })
    } else {
      const fd = new FormData()
      fd.append('project_id', props.projectId)
      fd.append('name', name.value.trim())
      fd.append('requirement_file', selectedFile.value!)
      res = await fetch('/api/workspaces', { method: 'POST', body: fd })
    }
    if (!res.ok) {
      const body = await res.json().catch(() => ({}))
      error.value = body.error ?? `Error ${res.status}`
      return
    }
    const ws = await res.json() as Workspace
    emit('created', ws)
  } catch (e) {
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
  borderRadius: '12px', width: '100%', maxWidth: '520px',
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
const tabBar = {
  display: 'flex', gap: '2px', marginBottom: '10px',
  background: '#0A0A0A', borderRadius: '6px', padding: '3px',
  border: '1px solid rgba(255,255,255,0.08)',
  width: 'fit-content',
}
const tab = (active: boolean) => ({
  padding: '5px 14px', borderRadius: '4px', border: 'none', cursor: 'pointer',
  fontFamily: "'DM Sans',sans-serif", fontSize: '12px', fontWeight: 600,
  background: active ? 'rgba(255,255,255,0.08)' : 'transparent',
  color: active ? '#E0E0E0' : '#717171',
  transition: 'all 120ms ease',
})
const textareaStyle = {
  width: '100%', background: '#0A0A0A', border: '1px solid rgba(255,255,255,0.12)',
  borderRadius: '6px', padding: '10px 12px', color: '#E0E0E0',
  fontFamily: "'DM Sans',sans-serif", fontSize: '13.5px', outline: 'none',
  resize: 'vertical' as const, lineHeight: 1.6, boxSizing: 'border-box' as const,
  minHeight: '140px',
}
const dropZone = (active: boolean) => ({
  border: `1.5px dashed ${active ? '#F5C518' : 'rgba(255,255,255,0.12)'}`,
  borderRadius: '8px', padding: '28px 20px',
  display: 'flex', flexDirection: 'column' as const, alignItems: 'center',
  background: active ? 'rgba(245,197,24,0.04)' : '#0A0A0A',
  transition: 'all 150ms ease', cursor: 'default', minHeight: '140px',
  justifyContent: 'center',
})
const browseBtn = {
  marginTop: '10px', padding: '7px 16px', borderRadius: '6px',
  background: 'rgba(255,255,255,0.06)', border: '1px solid rgba(255,255,255,0.12)',
  color: '#D4D4D4', cursor: 'pointer',
  fontFamily: "'DM Sans',sans-serif", fontSize: '12px', fontWeight: 600,
}
const clearFileBtn = {
  marginTop: '10px', padding: '5px 12px', borderRadius: '5px',
  background: 'transparent', border: '1px solid rgba(255,255,255,0.10)',
  color: '#717171', cursor: 'pointer',
  fontFamily: "'DM Sans',sans-serif", fontSize: '11px',
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
