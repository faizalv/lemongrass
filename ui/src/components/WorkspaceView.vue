<template>
  <div style="flex:1;display:flex;flex-direction:column;overflow:hidden">
    <!-- Top bar -->
    <div :style="topBar">
      <div style="padding:22px 32px 16px;display:flex;align-items:flex-end;gap:24px">
        <div style="flex:1;min-width:0">
          <div :style="breadcrumb">
            <AppIcon name="layers" :size="11" :extra-style="{ flexShrink: 0 }" />
            <span>Workspace</span>
            <span style="color:#2A2A2A">·</span>
            <span>{{ workspace.branch }}</span>
            <span style="color:#2A2A2A">·</span>
            <span>pinned @ {{ workspace.commit }}</span>
          </div>
          <div :style="wsTitle">{{ workspace.name }}</div>
        </div>
        <div style="display:flex;gap:8px;flex-shrink:0">
          <button :style="btnGhost">
            <AppIcon name="git-branch" :size="13" />
            Re-pin commit
          </button>
          <button :style="btnGhost">
            <AppIcon name="more-horizontal" :size="14" />
          </button>
        </div>
      </div>

      <!-- Tabs -->
      <div style="display:flex;gap:4px;padding:0 28px">
        <button
          v-for="tab in tabs"
          :key="tab.id"
          :style="tabBtn(tab.id === activeTab)"
          @click="activeTab = tab.id"
        >
          <AppIcon :name="tab.icon" :size="13" />
          {{ tab.label }}
          <span v-if="tab.id === activeTab" :style="tabUnderline" />
        </button>
      </div>
    </div>

    <!-- Tab content -->
    <div style="flex:1;display:flex;overflow:hidden">
      <GroomingView v-if="activeTab === 'grooming'" :workspace="workspace" @jump-tab="activeTab = $event" />

      <div v-else-if="activeTab === 'planning'" class="fade-in" :style="emptyWrap">
        <div :style="emptyIcon"><AppIcon name="route" :size="22" color="#555" /></div>
        <div :style="emptyTitle">Planning hasn't started</div>
        <div :style="emptyBody">Once grooming wraps, the planner will turn each implementation detail into surgical line-range patches and dispatch workers.</div>
        <div :style="lockedBadge">
          <AppIcon name="lock" :size="11" />
          Grooming must finish first
        </div>
      </div>

      <div v-else-if="activeTab === 'testing'" class="fade-in" :style="emptyWrap">
        <div :style="emptyIcon"><AppIcon name="flask-conical" :size="22" color="#555" /></div>
        <div :style="emptyTitle">Nothing to test yet</div>
        <div :style="emptyBody">REST endpoints pre-populated from your Swagger map will land here after the build phase.</div>
        <div :style="lockedBadge">
          <AppIcon name="lock" :size="11" />
          Grooming must finish first
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import type { Workspace } from '../types'
import AppIcon from './AppIcon.vue'
import GroomingView from './grooming/GroomingView.vue'

defineProps<{ workspace: Workspace & { branch: string; commit: string } }>()

const activeTab = ref('grooming')

const tabs = [
  { id: 'grooming', label: 'Grooming', icon: 'message-square-text' },
  { id: 'planning', label: 'Planning', icon: 'route' },
  { id: 'testing', label: 'Testing', icon: 'flask-conical' },
]

const topBar = { borderBottom: '1px solid rgba(255,255,255,0.06)', background: '#0A0A0A', flexShrink: 0 }
const breadcrumb = { display: 'flex', alignItems: 'center', gap: '8px', marginBottom: '6px', fontFamily: "'JetBrains Mono','Courier Prime',monospace", fontSize: '11px', color: '#555', whiteSpace: 'nowrap', overflow: 'hidden' }
const wsTitle = { fontFamily: "'Comfortaa', sans-serif", fontSize: '28px', fontWeight: 700, color: '#fff', letterSpacing: '-0.02em', whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis' }
const btnGhost = { display: 'inline-flex', alignItems: 'center', gap: '6px', padding: '7px 11px', borderRadius: '6px', background: 'transparent', border: '1px solid rgba(255,255,255,0.10)', color: '#B0B0B0', cursor: 'pointer', fontFamily: "'DM Sans',sans-serif", fontSize: '12px', fontWeight: 500 }
const tabBtn = (active: boolean) => ({ position: 'relative', display: 'inline-flex', alignItems: 'center', gap: '7px', padding: '10px 14px 12px', background: 'transparent', border: 'none', cursor: 'pointer', color: active ? '#F5C518' : '#9A9A9A', fontFamily: "'DM Sans',sans-serif", fontSize: '13.5px', fontWeight: active ? 600 : 500 })
const tabUnderline = { position: 'absolute', left: '8px', right: '8px', bottom: '-1px', height: '2px', background: '#F5C518', borderRadius: '2px' }
const emptyWrap = { flex: 1, display: 'flex', flexDirection: 'column', alignItems: 'center', justifyContent: 'center', padding: '40px', textAlign: 'center' }
const emptyIcon = { width: '56px', height: '56px', borderRadius: '14px', background: 'rgba(255,255,255,0.03)', border: '1px solid rgba(255,255,255,0.06)', display: 'flex', alignItems: 'center', justifyContent: 'center', marginBottom: '18px' }
const emptyTitle = { fontFamily: "'Comfortaa',sans-serif", fontSize: '22px', fontWeight: 700, color: '#E0E0E0', letterSpacing: '-0.01em', marginBottom: '8px' }
const emptyBody = { fontSize: '13.5px', color: '#717171', lineHeight: 1.7, maxWidth: '420px', fontFamily: "'DM Sans',sans-serif" }
const lockedBadge = { marginTop: '18px', display: 'inline-flex', alignItems: 'center', gap: '6px', padding: '6px 12px', borderRadius: '999px', background: 'rgba(255,255,255,0.03)', border: '1px solid rgba(255,255,255,0.07)', color: '#717171', fontSize: '11.5px', fontFamily: "'DM Sans',sans-serif", fontWeight: 600 }
</script>
