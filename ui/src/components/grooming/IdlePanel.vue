<template>
  <div class="fade-in" :style="wrap">
    <div>
      <div :style="heading">Requirements</div>
      <div :style="sub">
        Add text or upload files. Items are saved immediately. You can add or remove requirements until grooming starts.
      </div>
    </div>

    <!-- Requirements section -->
    <div>
      <!-- Header row -->
      <div style="display:flex;align-items:center;gap:8px">
        <span :style="sectionLabel">Requirements</span>
        <span style="flex:1" />
        <button v-if="!locked" :style="toolBtn" @click="addingText = true; uploadError = ''">
          <AppIcon name="plus" :size="12" />
          Add text
        </button>
        <label v-if="!locked" :style="{ ...toolBtn, cursor: uploading ? 'not-allowed' : 'pointer' }">
          <AppIcon name="paperclip" :size="12" />
          Upload file
          <input
            type="file"
            accept=".pdf,.png,.jpg,.jpeg,.webp,.gif"
            style="display:none"
            :disabled="uploading"
            @change="onFileSelect"
          />
        </label>
      </div>

      <!-- Add text inline expand -->
      <div v-if="addingText" :style="{ ...inputBox, marginTop: '10px' }">
        <textarea
          v-model="addTextValue"
          :style="textarea"
          placeholder="Paste your requirement here…"
          @keydown.escape="cancelAddText"
        />
        <div :style="toolbar">
          <span :style="charCount">{{ addTextValue.length }} chars</span>
          <span style="flex:1" />
          <button :style="toolBtn" @click="cancelAddText">Cancel</button>
          <button
            :disabled="addTextValue.trim().length < 10"
            :style="startBtn(addTextValue.trim().length >= 10)"
            @click="submitText"
          >Add</button>
        </div>
      </div>

      <!-- Upload / add error -->
      <div v-if="uploadError" style="font-size:11px;color:var(--color-error);margin-top:6px;font-family:'DM Sans',sans-serif">
        {{ uploadError }}
      </div>

      <!-- Requirements list -->
      <div style="display:flex;flex-direction:column;gap:8px;margin-top:8px">
        <div v-if="requirements.length === 0" :style="emptyState">
          No requirements yet. Add one to get started.
        </div>
        <div v-for="r in requirements" :key="r.id" :style="reqCard">
          <AppIcon
            :name="r.type === 'text' ? 'file-text' : r.type === 'image' ? 'image' : 'file'"
            :size="14"
            :extra-style="{ color: 'var(--color-gray-400)', flexShrink: 0 }"
          />
          <span :style="reqCardText">
            {{ r.type === 'text' ? (r.text_content ?? '').slice(0, 120) : r.file_name }}
          </span>
          <button v-if="!locked" :style="deleteBtn" @click="deleteReq(r.id)">&times;</button>
        </div>
      </div>
    </div>

    <!-- Start grooming -->
    <div style="display:flex;flex-direction:column;align-items:flex-start;gap:6px;margin-top:4px">
      <button
        :disabled="requirements.length === 0 || starting"
        :style="startBtn(requirements.length > 0 && !starting)"
        @click="$emit('start')"
      >
        {{ starting ? 'Starting…' : 'Start grooming' }}
        <AppIcon v-if="!starting" name="arrow-right" :size="13" />
      </button>
      <span v-if="requirements.length === 0" :style="disabledHint">
        Add at least one requirement to start grooming
      </span>
    </div>

    <!-- Info box -->
    <div :style="infoBox">
      <AppIcon name="info" :size="14" color="var(--color-info)" :extra-style="{ flexShrink: 0, marginTop: '2px' }" />
      <div :style="infoText">
        Grooming reads your recon map and proposes a task breakdown based on your requirements. No code is touched in this phase.
      </div>
    </div>

    <!-- Error slot (grooming start errors) -->
    <slot name="error" />
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import type { WorkspaceRequirement } from '../../types'
import AppIcon from '../AppIcon.vue'

const props = defineProps<{ workspaceId: string; locked: boolean; starting?: boolean }>()
defineEmits<{ 'start': [] }>()

const requirements = ref<WorkspaceRequirement[]>([])
const addingText   = ref(false)
const addTextValue = ref('')
const uploading    = ref(false)
const uploadError  = ref('')

onMounted(async () => {
  try {
    const r = await fetch(`/api/workspaces/${props.workspaceId}/requirements`)
    if (r.ok) requirements.value = await r.json()
  } catch { /* ignore */ }
})

function cancelAddText() {
  addingText.value = false
  addTextValue.value = ''
}

async function submitText() {
  if (addTextValue.value.trim().length < 10) return
  uploadError.value = ''
  try {
    const r = await fetch(`/api/workspaces/${props.workspaceId}/requirements`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ text_content: addTextValue.value.trim() }),
    })
    if (!r.ok) {
      const body = await r.json().catch(() => ({}))
      uploadError.value = body.error ?? `Error ${r.status}`
      return
    }
    const req: WorkspaceRequirement = await r.json()
    requirements.value.push(req)
    cancelAddText()
  } catch {
    uploadError.value = 'Network error, please try again.'
  }
}

async function onFileSelect(e: Event) {
  const file = (e.target as HTMLInputElement).files?.[0]
  if (!file) return
  uploadError.value = ''

  const ext = file.name.split('.').pop()?.toLowerCase() ?? ''
  const maxSize: Record<string, number> = {
    pdf: 50 * 1024 * 1024,
    png: 20 * 1024 * 1024, jpg: 20 * 1024 * 1024, jpeg: 20 * 1024 * 1024,
    webp: 20 * 1024 * 1024, gif: 20 * 1024 * 1024,
  }
  if (!(ext in maxSize)) {
    uploadError.value = `Unsupported file type: .${ext}`
    ;(e.target as HTMLInputElement).value = ''
    return
  }
  if (file.size > maxSize[ext]) {
    uploadError.value = 'File exceeds size limit'
    ;(e.target as HTMLInputElement).value = ''
    return
  }

  uploading.value = true
  try {
    const fd = new FormData()
    fd.append('file', file)
    const r = await fetch(`/api/workspaces/${props.workspaceId}/requirements`, { method: 'POST', body: fd })
    if (!r.ok) {
      const body = await r.json().catch(() => ({}))
      uploadError.value = body.error ?? `Error ${r.status}`
      return
    }
    const req: WorkspaceRequirement = await r.json()
    requirements.value.push(req)
  } catch {
    uploadError.value = 'Network error, please try again.'
  } finally {
    uploading.value = false
    ;(e.target as HTMLInputElement).value = ''
  }
}

async function deleteReq(id: string) {
  try {
    await fetch(`/api/workspaces/${props.workspaceId}/requirements/${id}`, { method: 'DELETE' })
    requirements.value = requirements.value.filter(r => r.id !== id)
  } catch { /* ignore */ }
}

const wrap = {
  maxWidth: '760px', margin: '40px auto 0', padding: '0 32px 40px',
  display: 'flex', flexDirection: 'column', gap: '18px',
} as Record<string, any>
const heading = {
  fontFamily: 'var(--font-display)', fontSize: '26px', fontWeight: 700,
  color: 'var(--color-fg-primary)', letterSpacing: '-0.02em', marginBottom: '8px',
}
const sub = { fontSize: '14px', color: 'var(--color-gray-300)', fontFamily: 'var(--font-body)', lineHeight: 1.6 }
const sectionLabel = {
  fontSize: '10px', fontWeight: 700, letterSpacing: '0.10em', textTransform: 'uppercase',
  color: 'var(--color-gray-400)', fontFamily: 'var(--font-body)',
}
const toolBtn = {
  display: 'inline-flex', alignItems: 'center', gap: '6px',
  padding: '6px 10px', background: 'transparent',
  border: '1px solid rgba(255,255,255,0.10)', borderRadius: '5px',
  color: 'var(--color-gray-200)', fontFamily: 'var(--font-body)', fontSize: '12px', fontWeight: 500,
  cursor: 'pointer',
}
const inputBox = {
  background: 'var(--color-gray-900)', border: '1px solid rgba(255,255,255,0.08)',
  borderRadius: '10px', overflow: 'hidden',
}
const textarea = {
  width: '100%', minHeight: '120px', padding: '14px 16px',
  background: 'transparent', border: 'none', outline: 'none', resize: 'vertical' as const,
  color: 'var(--color-gray-100)', fontFamily: 'var(--font-body)', fontSize: '14px', lineHeight: 1.7,
}
const toolbar = {
  display: 'flex', alignItems: 'center', gap: '8px',
  padding: '8px 12px', borderTop: '1px solid rgba(255,255,255,0.06)', background: 'var(--color-surface-0)',
}
const charCount = {
  fontSize: '11px', color: 'var(--color-gray-500)', fontFamily: 'var(--font-mono)', flex: 1,
}
const emptyState = {
  fontSize: '13px', color: 'var(--color-gray-400)', fontFamily: 'var(--font-body)',
  padding: '20px 0', textAlign: 'center' as const,
}
const reqCard = {
  background: 'var(--color-gray-900)', border: '1px solid rgba(255,255,255,0.07)', borderRadius: '10px',
  padding: '14px 16px', display: 'flex', alignItems: 'center', gap: '10px',
}
const reqCardText = {
  flex: 1, minWidth: 0, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' as const,
  fontSize: '13px', color: 'var(--color-gray-100)', fontFamily: 'var(--font-body)',
}
const deleteBtn = {
  padding: '3px 8px', borderRadius: '4px', background: 'transparent',
  border: '1px solid rgba(255,255,255,0.10)', color: 'var(--color-gray-500)',
  fontFamily: 'var(--font-body)', fontSize: '12px', cursor: 'pointer',
  flexShrink: 0,
}
const startBtn = (enabled: boolean) => ({
  display: 'inline-flex', alignItems: 'center', gap: '7px',
  padding: '8px 16px', borderRadius: '6px',
  background: enabled ? 'var(--color-amber)' : 'var(--color-gray-700)',
  color: enabled ? 'var(--color-surface-0)' : 'var(--color-gray-500)',
  border: 'none', cursor: enabled ? 'pointer' : 'not-allowed',
  fontFamily: 'var(--font-body)', fontWeight: 700, fontSize: '13px',
})
const disabledHint = {
  fontSize: '12px', color: 'var(--color-gray-400)', fontFamily: 'var(--font-body)',
}
const infoBox = {
  marginTop: '4px', padding: '12px 16px',
  background: 'rgba(96,165,250,0.05)', border: '1px solid rgba(96,165,250,0.18)',
  borderRadius: '8px', display: 'flex', gap: '10px',
}
const infoText = { fontSize: '12.5px', color: 'var(--color-gray-300)', lineHeight: 1.6, fontFamily: 'var(--font-body)' }
</script>
