<template>
  <div class="fade-in" :style="wrap">
    <div>
      <div :style="heading">Artifacts</div>
      <div :style="sub">
        All workspaces for this project, including deleted ones. Requirements are preserved as historical context.
      </div>
    </div>

    <div v-if="loading" :style="emptyState">Loading...</div>

    <div v-else-if="workspaces.length === 0" :style="emptyState">
      No workspaces yet for this project.
    </div>

    <div v-else style="display:flex;flex-direction:column;gap:16px">
      <div v-for="ws in workspaces" :key="ws.id" :style="wsCard">
        <div style="display:flex;align-items:center;gap:10px;margin-bottom:12px">
          <div :style="wsName">{{ ws.name }}</div>
          <span :style="statusBadge(ws.status)">{{ ws.status }}</span>
        </div>

        <div v-if="ws.requirements.length === 0" :style="noReqs">
          No requirements
        </div>
        <div v-else style="display:flex;flex-direction:column;gap:6px">
          <div v-for="r in ws.requirements" :key="r.id" :style="reqCard">
            <AppIcon
              :name="r.type === 'text' ? 'file-text' : r.type === 'image' ? 'image' : 'file'"
              :size="13"
              :extra-style="{ color: 'var(--color-gray-500)', flexShrink: 0 }"
            />
            <span :style="reqText">
              {{ r.type === 'text' ? (r.text_content ?? '').slice(0, 120) : r.file_name }}
            </span>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import type { WorkspaceWithRequirements, WorkspaceStatus } from '../types'
import AppIcon from './AppIcon.vue'

const props = defineProps<{ projectId: string }>()

const workspaces = ref<WorkspaceWithRequirements[]>([])
const loading = ref(true)

onMounted(async () => {
  try {
    const r = await fetch(`/api/workspaces?project_id=${props.projectId}&include_deleted=true`)
    if (r.ok) workspaces.value = await r.json()
  } catch { /* ignore */ } finally {
    loading.value = false
  }
})

const statusColors: Record<string, string> = {
  idle:               'rgba(255,255,255,0.12)',
  grooming:           'rgba(96,165,250,0.20)',
  awaiting_execution: 'rgba(245,197,24,0.20)',
  executing:          'rgba(74,222,128,0.20)',
  done:               'rgba(74,222,128,0.12)',
  deleted:            'rgba(255,255,255,0.06)',
}
const statusText: Record<string, string> = {
  idle:               'var(--color-gray-300)',
  grooming:           'var(--color-info)',
  awaiting_execution: 'var(--color-amber)',
  executing:          'var(--color-success)',
  done:               'var(--color-success)',
  deleted:            'var(--color-gray-600)',
}

function statusBadge(status: WorkspaceStatus) {
  return {
    display: 'inline-flex', alignItems: 'center',
    padding: '2px 8px', borderRadius: '999px',
    background: statusColors[status] ?? 'rgba(255,255,255,0.08)',
    color: statusText[status] ?? 'var(--color-gray-400)',
    fontSize: '10px', fontWeight: 700, letterSpacing: '0.06em', textTransform: 'uppercase',
    fontFamily: 'var(--font-body)',
  }
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
const emptyState = { fontSize: '13px', color: 'var(--color-gray-400)', fontFamily: 'var(--font-body)', padding: '20px 0', textAlign: 'center' as const }
const wsCard = {
  background: 'var(--color-surface-1)', border: '1px solid rgba(255,255,255,0.07)',
  borderRadius: '10px', padding: '18px 20px',
}
const wsName = {
  fontFamily: 'var(--font-display)', fontSize: '16px', fontWeight: 600,
  color: 'var(--color-fg-primary)', letterSpacing: '-0.01em',
}
const noReqs = { fontSize: '12px', color: 'var(--color-gray-600)', fontFamily: 'var(--font-body)' }
const reqCard = {
  background: 'var(--color-surface-0)', border: '1px solid rgba(255,255,255,0.05)',
  borderRadius: '6px', padding: '8px 12px', display: 'flex', alignItems: 'center', gap: '8px',
}
const reqText = {
  flex: 1, minWidth: 0, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' as const,
  fontSize: '12px', color: 'var(--color-gray-400)', fontFamily: 'var(--font-body)',
}
</script>
