<template>
  <div :style="s.sidebar">
    <!-- Wordmark -->
    <div :style="s.logoArea">
      <div :style="s.leaf">
        <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="#0A0A0A" stroke-width="2.4" stroke-linecap="round" stroke-linejoin="round">
          <path d="M11 20A7 7 0 0 1 9.8 6.1C15.5 5 17 4.48 19.2 2.2c1.7 6.6.2 13.1-5 17.7"/>
          <path d="M2 21c0-3 1.85-5.36 5.08-6"/>
        </svg>
      </div>
      <span :style="s.wordmark">lemongrass</span>
    </div>

    <!-- Project picker -->
    <div :style="s.projWrap">
      <!-- Empty: no projects -->
      <button v-if="projects.length === 0" :style="s.addProjectBtn" @click="$emit('add-project')">
        <AppIcon name="folder-git-2" :size="13" :extra-style="{ color: '#555', flexShrink: 0 }" />
        <span :style="{ flex: 1, minWidth: 0, fontSize: '12.5px', color: '#555', fontWeight: 500, textAlign: 'left' }">Add a project…</span>
        <AppIcon name="plus" :size="12" :extra-style="{ color: '#555', flexShrink: 0 }" />
      </button>

      <!-- Populated: project picker -->
      <template v-else>
        <button ref="triggerRef" :style="s.projBtn(projMenuOpen)" @click="projMenuOpen = !projMenuOpen">
          <AppIcon name="folder-git-2" :extra-style="{ color: '#F5C518', flexShrink: 0 }" :size="13" />
          <div style="flex:1;min-width:0">
            <div :style="s.projLabel">{{ currentProject?.name }}</div>
            <div :style="s.projMeta">{{ currentProject?.branch }} · {{ currentProject?.shortPath }}</div>
          </div>
          <AppIcon :name="projMenuOpen ? 'chevron-down' : 'chevrons-up-down'" :size="13" :extra-style="{ color: '#555', flexShrink: 0 }" />
        </button>

        <div v-if="projMenuOpen" ref="menuRef" :style="s.projMenu">
          <div
            v-for="p in projects"
            :key="p.id"
            :style="s.projMenuItemWrap"
            @mouseenter="hoveredMenuProject = p.id"
            @mouseleave="hoveredMenuProject = ''"
          >
            <button
              :style="s.projMenuItem(p.id === currentProjectId)"
              @click="selectProject(p.id)"
            >
              <AppIcon :name="p.id === currentProjectId ? 'check' : 'folder'" :size="12" :extra-style="{ color: p.id === currentProjectId ? '#F5C518' : '#555' }" />
              <div style="flex:1;min-width:0">
                <div style="white-space:nowrap;overflow:hidden;text-overflow:ellipsis">{{ p.name }}</div>
                <div :style="{ ...s.projMeta, color: '#3D3D3D' }">{{ p.branch }}</div>
              </div>
            </button>
            <button
              v-if="hoveredMenuProject === p.id"
              :style="s.trashBtn"
              title="Remove project"
              @click.stop="requestDelete(p.id)"
            >
              <svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                <polyline points="3 6 5 6 21 6"/>
                <path d="M19 6l-1 14a2 2 0 0 1-2 2H8a2 2 0 0 1-2-2L5 6"/>
                <path d="M10 11v6"/><path d="M14 11v6"/>
                <path d="M9 6V4a1 1 0 0 1 1-1h4a1 1 0 0 1 1 1v2"/>
              </svg>
            </button>
          </div>
          <div :style="s.projMenuDivider"></div>
          <button :style="s.projMenuFoot" @click="$emit('add-project'); projMenuOpen = false">
            <AppIcon name="plus" :size="12" :extra-style="{ color: '#9A9A9A' }" />
            Add new project…
          </button>
        </div>
      </template>
    </div>

    <!-- Workspaces label -->
    <div v-if="projects.length > 0" :style="s.wsLabel">
      <span>Workspaces</span>
      <span :style="s.wsCount">{{ workspaces.length }}</span>
    </div>

    <!-- Workspace list -->
    <div :style="s.wsList">
      <template v-if="projects.length === 0">
        <!-- No projects yet -->
        <div :style="s.wsEmptyHint">
          <AppIcon name="layers" :size="14" :extra-style="{ color: '#2A2A2A', flexShrink: 0 }" />
          <span style="font-size:12px;color:#3D3D3D;line-height:1.5">Add a project to see workspaces here.</span>
        </div>
      </template>

      <template v-else>
        <template v-for="w in workspaces" :key="w.id">
          <button
            :style="s.wsItem(w.id === activeWorkspaceId)"
            @click="$emit('select-workspace', w.id)"
          >
            <AppIcon :name="w.id === 'reconnaissance' ? 'radar' : (w.icon || 'layers')" :size="14" :extra-style="{ color: w.id === activeWorkspaceId ? '#F5C518' : '#717171' }" />
            <span :style="s.wsItemLabel(w.id === activeWorkspaceId)">{{ w.name }}</span>
            <span v-if="w.id !== 'reconnaissance'" :style="s.wsStatusPip(w.status || 'idle')"></span>
          </button>
          <div v-if="w.id === 'reconnaissance'" :style="s.wsDivider"></div>
        </template>

        <button
          :style="s.addBtn(hoveredAdd)"
          @mouseenter="hoveredAdd = true"
          @mouseleave="hoveredAdd = false"
          @click="$emit('add-workspace')"
        >
          <AppIcon name="plus" :size="13" />
          New workspace
        </button>
      </template>
    </div>

    <!-- Settings + Debug -->
    <div :style="s.bottomSection">
      <button
        :style="{ ...s.settingsBtn, background: hoveredDebug ? 'rgba(255,255,255,0.04)' : 'transparent' }"
        @mouseenter="hoveredDebug = true"
        @mouseleave="hoveredDebug = false"
        @click="$emit('open-debug')"
      >
        <AppIcon name="terminal" :size="14" :extra-style="{ color: hoveredDebug ? '#F5C518' : '#9A9A9A' }" />
        <span :style="{ fontSize: '13px', color: hoveredDebug ? '#fff' : '#C4C4C4', fontWeight: 500 }">Hook Debug</span>
      </button>
      <button
        :style="{ ...s.settingsBtn, background: hoveredSettings ? 'rgba(255,255,255,0.04)' : 'transparent' }"
        @mouseenter="hoveredSettings = true"
        @mouseleave="hoveredSettings = false"
        @click="$emit('open-settings')"
      >
        <AppIcon name="settings" :size="14" :extra-style="{ color: hoveredSettings ? '#F5C518' : '#9A9A9A' }" />
        <span :style="{ fontSize: '13px', color: hoveredSettings ? '#fff' : '#C4C4C4', fontWeight: 500 }">Settings</span>
        <span style="margin-left:auto;font-size:10px;color:#3D3D3D;font-family:'JetBrains Mono','Courier Prime',monospace">v0.3</span>
      </button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted } from 'vue'
import type { Project, Workspace } from '../types'
import AppIcon from './AppIcon.vue'

const props = defineProps<{
  projects: Project[]
  currentProjectId: string
  workspaces: Workspace[]
  activeWorkspaceId: string
}>()

const emit = defineEmits<{
  'switch-project': [id: string]
  'select-workspace': [id: string]
  'add-workspace': []
  'add-project': []
  'open-settings': []
  'open-debug': []
  'delete-project': [id: string]
}>()

const projMenuOpen = ref(false)
const hoveredAdd = ref(false)
const hoveredDebug = ref(false)
const hoveredSettings = ref(false)
const hoveredMenuProject = ref('')
const triggerRef = ref<HTMLElement | null>(null)
const menuRef = ref<HTMLElement | null>(null)

const currentProject = computed(() => props.projects.find(p => p.id === props.currentProjectId))

function selectProject(id: string) {
  emit('switch-project', id)
  projMenuOpen.value = false
}

function requestDelete(id: string) {
  projMenuOpen.value = false
  emit('delete-project', id)
}

function onClickOutside(e: MouseEvent) {
  if (!projMenuOpen.value) return
  if (
    menuRef.value && !menuRef.value.contains(e.target as Node) &&
    triggerRef.value && !triggerRef.value.contains(e.target as Node)
  ) {
    projMenuOpen.value = false
  }
}

onMounted(() => document.addEventListener('mousedown', onClickOutside))
onUnmounted(() => document.removeEventListener('mousedown', onClickOutside))

const STATUS_COLORS: Record<string, string> = {
  idle: '#4A4A4A',
  grooming: '#F5C518',
  planning: '#60A5FA',
  testing: '#4ADE80',
  error: '#F87171',
}

const s = {
  sidebar: {
    width: '248px', height: '100vh', background: '#0A0A0A',
    borderRight: '1px solid rgba(255,255,255,0.07)',
    display: 'flex', flexDirection: 'column', flexShrink: 0,
  } as Record<string, any>,
  logoArea: {
    padding: '20px 20px 14px',
    display: 'flex', alignItems: 'center', gap: '8px',
  },
  leaf: {
    width: '18px', height: '18px', borderRadius: '4px',
    background: '#F5C518',
    display: 'flex', alignItems: 'center', justifyContent: 'center',
    flexShrink: 0,
  },
  wordmark: {
    fontFamily: "'Comfortaa', sans-serif",
    fontSize: '18px', fontWeight: 700, color: '#F5F5F5',
    letterSpacing: '-0.01em',
  },
  projWrap: { padding: '0 12px 14px', position: 'relative' },
  projBtn: (open: boolean) => ({
    width: '100%', display: 'flex', alignItems: 'center', gap: '8px',
    padding: '9px 11px',
    background: open ? '#1A1A1A' : '#141414',
    border: `1px solid ${open ? 'rgba(245,197,24,0.30)' : 'rgba(255,255,255,0.07)'}`,
    borderRadius: '6px', cursor: 'pointer', textAlign: 'left',
    transition: 'all 150ms ease', fontFamily: "'DM Sans', sans-serif",
  }),
  projLabel: {
    flex: 1, minWidth: 0, fontSize: '12.5px', color: '#E0E0E0', fontWeight: 500,
    whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis',
  },
  projMeta: {
    fontFamily: "'JetBrains Mono', 'Courier Prime', monospace",
    fontSize: '10px', color: '#555', marginTop: '1px', lineHeight: 1.3,
    whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis',
  },
  projMenu: {
    position: 'absolute', left: '12px', right: '12px', top: 'calc(100% - 4px)',
    background: '#1C1C1C', border: '1px solid rgba(255,255,255,0.13)',
    borderRadius: '8px', boxShadow: '0 12px 32px rgba(0,0,0,0.7)',
    overflow: 'hidden', zIndex: 80, animation: 'lgFadeIn 120ms ease',
  },
  projMenuItem: (active: boolean) => ({
    flex: 1, display: 'flex', alignItems: 'center', gap: '8px',
    padding: '10px 12px', paddingRight: '36px',
    background: 'transparent', border: 'none', cursor: 'pointer',
    color: active ? '#F5C518' : '#D4D4D4',
    fontSize: '12.5px', fontFamily: "'DM Sans',sans-serif", fontWeight: 500,
    textAlign: 'left',
  }),
  projMenuItemWrap: {
    position: 'relative', display: 'flex', alignItems: 'center',
  },
  trashBtn: {
    position: 'absolute', right: '8px',
    display: 'flex', alignItems: 'center', justifyContent: 'center',
    width: '24px', height: '24px', borderRadius: '4px',
    background: 'rgba(248,113,113,0.12)', border: 'none',
    color: '#F87171', cursor: 'pointer', flexShrink: 0,
  },
  projMenuDivider: { height: '1px', background: 'rgba(255,255,255,0.06)' },
  projMenuFoot: {
    width: '100%', display: 'flex', alignItems: 'center', gap: '8px',
    padding: '10px 12px', background: 'transparent', border: 'none', cursor: 'pointer',
    color: '#9A9A9A', fontSize: '12px', fontFamily: "'DM Sans',sans-serif", fontWeight: 500,
    textAlign: 'left',
  },
  wsLabel: {
    fontSize: '10px', fontWeight: 700, color: '#3D3D3D',
    letterSpacing: '0.12em', textTransform: 'uppercase',
    padding: '14px 16px 8px', fontFamily: "'DM Sans', sans-serif",
    display: 'flex', alignItems: 'center', justifyContent: 'space-between',
  },
  wsCount: { color: '#3D3D3D', fontWeight: 600, fontSize: '10px' },
  wsList: {
    flex: 1, overflowY: 'auto', overflowX: 'hidden',
    padding: '0 10px', display: 'flex', flexDirection: 'column', gap: '1px',
  },
  wsItem: (active: boolean) => ({
    width: '100%', display: 'flex', alignItems: 'center', gap: '9px',
    padding: '8px 10px',
    background: active ? 'rgba(245,197,24,0.10)' : 'transparent',
    border: 'none', borderRadius: '6px', cursor: 'pointer',
    textAlign: 'left', transition: 'background 120ms ease',
    fontFamily: "'DM Sans', sans-serif",
  }),
  wsItemLabel: (active: boolean) => ({
    flex: 1, minWidth: 0, fontSize: '13px',
    fontWeight: active ? 600 : 500,
    color: active ? '#F5C518' : '#C4C4C4',
    whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis',
    lineHeight: 1.3,
  }),
  wsStatusPip: (status: string) => ({
    width: '6px', height: '6px', borderRadius: '50%',
    background: STATUS_COLORS[status] || '#4A4A4A',
    flexShrink: 0,
    boxShadow: status === 'grooming' ? '0 0 0 3px rgba(245,197,24,0.12)' : 'none',
  }),
  wsDivider: { height: '1px', background: 'rgba(255,255,255,0.05)', margin: '8px 14px' },
  wsEmptyHint: {
    display: 'flex', alignItems: 'flex-start', gap: '10px',
    padding: '12px 10px', margin: '4px 0',
  },
  addProjectBtn: {
    width: '100%', display: 'flex', alignItems: 'center', gap: '8px',
    padding: '9px 11px',
    background: '#0E0E0E',
    border: '1px dashed rgba(255,255,255,0.10)',
    borderRadius: '6px', cursor: 'pointer', textAlign: 'left',
    fontFamily: "'DM Sans', sans-serif",
    transition: 'border-color 150ms ease',
  },
  addBtn: (hovered: boolean) => ({
    width: '100%', display: 'flex', alignItems: 'center', gap: '9px',
    padding: '8px 10px', marginTop: '4px',
    background: hovered ? 'rgba(255,255,255,0.04)' : 'transparent',
    border: '1px dashed rgba(255,255,255,0.10)',
    borderRadius: '6px', cursor: 'pointer', textAlign: 'left',
    transition: 'all 120ms ease',
    color: hovered ? '#F5C518' : '#717171',
    fontFamily: "'DM Sans', sans-serif", fontSize: '12.5px', fontWeight: 500,
  }),
  bottomSection: { padding: '12px 12px 14px', borderTop: '1px solid rgba(255,255,255,0.06)' },
  settingsBtn: {
    width: '100%', display: 'flex', alignItems: 'center', gap: '10px',
    padding: '9px 11px', border: 'none', borderRadius: '6px',
    cursor: 'pointer', textAlign: 'left',
    fontFamily: "'DM Sans', sans-serif", transition: 'background 120ms ease',
  },
}
</script>
