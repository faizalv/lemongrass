<template>
  <div v-if="status" :style="root">
    <button :style="bar" @click="expanded = !expanded">
      <AppIcon name="git-branch" :size="11" :extra-style="{ flexShrink: 0 }" />
      <span v-if="status.is_git_repo" style="flex:1;text-align:left">
        {{ status.branch }} · {{ status.head_commit }} · {{ status.changed_files?.length ?? 0 }} changed · {{ status.stale_count }} stale
      </span>
      <span v-else style="flex:1;text-align:left">{{ status.stale_count }} stale</span>
      <AppIcon :name="expanded ? 'chevron-up' : 'chevron-down'" :size="11" :extra-style="{ flexShrink: 0 }" />
    </button>

    <div v-if="expanded" :style="panel">
      <template v-if="status.is_git_repo">
        <div :style="section">
          <span :style="hash">{{ status.head_commit }}</span>
          <span :style="commitMsg">{{ status.head_message }}</span>
        </div>

        <div v-if="status.changed_files?.length" :style="section">
          <div :style="sectionLabel">CHANGED FILES</div>
          <div v-for="f in status.changed_files" :key="f.path" :style="fileRow">
            <span :style="filePath">{{ f.path }}</span>
            <span :style="statusBadge(f.status)">{{ f.status }}</span>
          </div>
        </div>

        <div v-if="status.recent_commits?.length" :style="section">
          <div :style="sectionLabel">RECENT COMMITS</div>
          <div v-for="c in status.recent_commits" :key="c.hash" :style="commitRow">
            <span :style="hash">{{ c.hash }}</span>
            <span :style="commitMsgSmall">{{ c.message }}</span>
            <span :style="commitMeta">{{ c.author }} · {{ formatRelative(c.timestamp) }}</span>
          </div>
        </div>
      </template>

      <div :style="footer">
        <button :style="syncBtn" @click="syncNow">Sync now</button>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, onUnmounted } from 'vue'
import AppIcon from './AppIcon.vue'

const props = defineProps<{ projectId: string }>()

interface GitStatusData {
  is_git_repo: boolean
  branch?: string
  head_commit?: string
  head_message?: string
  changed_files?: { path: string; status: string }[]
  stale_count: number
  recent_commits?: { hash: string; message: string; author: string; timestamp: string }[]
}

const status = ref<GitStatusData | null>(null)
const expanded = ref(false)
let timer: ReturnType<typeof setInterval> | null = null

async function fetchStatus() {
  if (!props.projectId) return
  try {
    const r = await fetch(`/api/recon/projects/${props.projectId}/git-status`)
    if (r.ok) status.value = await r.json()
  } catch { /* ignore */ }
}

async function syncNow() {
  if (!props.projectId) return
  try {
    await fetch(`/api/recon/projects/${props.projectId}/activate`, { method: 'POST' })
  } catch { /* ignore */ }
}

function formatRelative(ts: string): string {
  if (!ts) return ''
  const diff = Math.floor((Date.now() - new Date(ts).getTime()) / 1000)
  if (diff < 60) return `${diff}s ago`
  if (diff < 3600) return `${Math.floor(diff / 60)}m ago`
  if (diff < 86400) return `${Math.floor(diff / 3600)}h ago`
  return `${Math.floor(diff / 86400)}d ago`
}

onMounted(() => {
  fetchStatus()
  timer = setInterval(fetchStatus, 5000)
})
onUnmounted(() => { if (timer) clearInterval(timer) })

const root = { borderTop: '1px solid rgba(255,255,255,0.04)', background: 'var(--color-surface-0)' }
const bar = { width: '100%', display: 'flex', alignItems: 'center', gap: '7px', padding: '6px 32px', background: 'transparent', border: 'none', cursor: 'pointer', fontFamily: "'JetBrains Mono','Courier Prime',monospace", fontSize: '11px', color: 'var(--color-gray-400)', textAlign: 'left' as const }
const panel = { borderTop: '1px solid rgba(255,255,255,0.05)', padding: '12px 32px 16px', display: 'flex', flexDirection: 'column' as const, gap: '16px', maxHeight: '320px', overflowY: 'auto' as const }
const section = { display: 'flex', flexDirection: 'column' as const, gap: '4px' }
const sectionLabel = { fontSize: '10px', fontWeight: 700, letterSpacing: '0.10em', color: 'var(--color-gray-600)', fontFamily: "'DM Sans',sans-serif", marginBottom: '2px' }
const hash = { fontFamily: "'JetBrains Mono','Courier Prime',monospace", fontSize: '11px', color: 'var(--color-gray-500)', flexShrink: 0 }
const commitMsg = { fontFamily: "'DM Sans',sans-serif", fontSize: '12px', color: 'var(--color-gray-300)' }
const fileRow = { display: 'flex', alignItems: 'center', justifyContent: 'space-between', gap: '8px', padding: '2px 0' }
const filePath = { fontFamily: "'JetBrains Mono','Courier Prime',monospace", fontSize: '11px', color: 'var(--color-gray-300)', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' as const }
const commitRow = { display: 'flex', alignItems: 'baseline', gap: '8px' }
const commitMsgSmall = { fontFamily: "'DM Sans',sans-serif", fontSize: '12px', color: 'var(--color-gray-300)', flex: 1, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' as const }
const commitMeta = { fontFamily: "'DM Sans',sans-serif", fontSize: '11px', color: 'var(--color-gray-600)', flexShrink: 0 }
const footer = { display: 'flex', justifyContent: 'flex-end' }
const syncBtn = { padding: '5px 12px', background: 'transparent', border: '1px solid rgba(255,255,255,0.10)', borderRadius: '5px', color: 'var(--color-gray-300)', fontFamily: "'DM Sans',sans-serif", fontSize: '12px', fontWeight: 500, cursor: 'pointer' }

const statusColors: Record<string, string> = { added: '#4ade80', deleted: '#f87171', modified: '#fbbf24' }
function statusBadge(s: string) {
  return { fontFamily: "'DM Sans',sans-serif", fontSize: '10px', fontWeight: 600, color: statusColors[s] ?? 'var(--color-gray-400)', flexShrink: 0 }
}
</script>
