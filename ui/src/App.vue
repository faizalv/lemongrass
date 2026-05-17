<template>
  <div style="display:flex;height:100vh;overflow:hidden;background:#0A0A0A">
    <AppSidebar
      :projects="projects"
      :current-project-id="currentProjectId"
      :workspaces="workspaces"
      :active-workspace-id="activeWorkspaceId"
      @switch-project="handleSwitchProject"
      @select-workspace="activeWorkspaceId = $event"
      @add-workspace="addingWorkspace = true"
      @attach-project="attachProjectOpen = true"
      @open-settings="settingsOpen = true"
      @open-debug="debugOpen = true"
    />

    <div style="flex:1;display:flex;flex-direction:column;overflow:hidden">
      <EmptyState v-if="projects.length === 0" @attach="attachProjectOpen = true" />

      <template v-else>
        <ReconnaissanceView
          v-if="activeWorkspace?.id === 'reconnaissance'"
          :project="currentProject!"
        />
        <WorkspaceView
          v-else-if="activeWorkspace"
          :key="activeWorkspace.id"
          :workspace="workspaceWithMeta"
        />
      </template>
    </div>

    <AttachProjectModal
      v-if="attachProjectOpen"
      @close="attachProjectOpen = false"
      @attached="handleAttached"
    />

    <AddWorkspaceModal
      v-if="addingWorkspace"
      :branch="currentProject?.branch ?? 'main'"
      @close="addingWorkspace = false"
      @create="handleCreateWorkspace"
    />

    <SettingsModal
      v-if="settingsOpen"
      @close="settingsOpen = false"
    />

    <DebugPanel
      v-if="debugOpen"
      @close="debugOpen = false"
    />
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import type { Project, Workspace, FsProject } from './types'
import AppSidebar from './components/AppSidebar.vue'
import EmptyState from './components/EmptyState.vue'
import ReconnaissanceView from './components/ReconnaissanceView.vue'
import WorkspaceView from './components/WorkspaceView.vue'
import AttachProjectModal from './components/modals/AttachProjectModal.vue'
import AddWorkspaceModal from './components/modals/AddWorkspaceModal.vue'
import SettingsModal from './components/modals/SettingsModal.vue'
import DebugPanel from './components/DebugPanel.vue'

const projects = ref<Project[]>([])
const currentProjectId = ref('')
const workspacesByProj = ref<Record<string, Workspace[]>>({})
const activeWorkspaceId = ref('')
const settingsOpen = ref(false)
const debugOpen = ref(false)
const addingWorkspace = ref(false)
const attachProjectOpen = ref(false)

const workspaces = computed(() => workspacesByProj.value[currentProjectId.value] ?? [])
const currentProject = computed(() => projects.value.find(p => p.id === currentProjectId.value))
const activeWorkspace = computed(() => workspaces.value.find(w => w.id === activeWorkspaceId.value) ?? workspaces.value[0])
const workspaceWithMeta = computed(() => ({
  ...activeWorkspace.value!,
  branch: currentProject.value?.branch ?? 'main',
  commit: '7c3d1a8',
}))

onMounted(async () => {
  try {
    const r = await fetch('/api/fs/projects')
    if (!r.ok) return
    const fsProjects: FsProject[] = await r.json()
    applyFsProjects(fsProjects)
  } catch { /* server not reachable yet */ }
})

function handleSwitchProject(pid: string) {
  currentProjectId.value = pid
  const list = workspacesByProj.value[pid] ?? []
  activeWorkspaceId.value = list[1]?.id ?? list[0]?.id ?? ''
}

function handleAttached(fsProjects: FsProject[]) {
  applyFsProjects(fsProjects)
  const last = fsProjects[fsProjects.length - 1]
  if (last) {
    currentProjectId.value = String(last.id)
    activeWorkspaceId.value = 'reconnaissance'
  }
  attachProjectOpen.value = false
}

function applyFsProjects(fsProjects: FsProject[]) {
  for (const fp of fsProjects) {
    if (fp.status === 'removed') continue
    const id = String(fp.id)
    const name = fp.path.split('/').pop() ?? fp.path
    const project: Project = {
      id,
      name,
      branch: 'main',
      shortPath: fp.path.replace(/^\/home\/[^/]+/, '~'),
    }
    if (!projects.value.find(p => p.id === id)) {
      projects.value = [...projects.value, project]
    }
    if (!workspacesByProj.value[id]) {
      workspacesByProj.value = {
        ...workspacesByProj.value,
        [id]: [{ id: 'reconnaissance', name: 'Reconnaissance', icon: 'radar' }],
      }
    }
  }
  if (!currentProjectId.value && projects.value.length > 0) {
    currentProjectId.value = projects.value[0].id
    activeWorkspaceId.value = 'reconnaissance'
  }
}

function handleCreateWorkspace(name: string) {
  const id = 'ws-' + Math.random().toString(36).slice(2, 7)
  const pid = currentProjectId.value
  workspacesByProj.value = {
    ...workspacesByProj.value,
    [pid]: [...(workspacesByProj.value[pid] ?? []), { id, name, icon: 'sparkles', status: 'idle' }],
  }
  activeWorkspaceId.value = id
  addingWorkspace.value = false
}
</script>
