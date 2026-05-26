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
      @add-project="addProjectOpen = true"
      @open-settings="settingsOpen = true"
      @open-debug="debugOpen = true"
      @delete-project="handleDeleteProject"
    />

    <div style="flex:1;display:flex;flex-direction:column;overflow:hidden">
      <EmptyState v-if="projects.length === 0" @add="addProjectOpen = true" />

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

    <AddProjectModal
      v-if="addProjectOpen"
      @close="addProjectOpen = false"
      @added="handleAdded"
    />

    <AddWorkspaceModal
      v-if="addingWorkspace"
      :project-id="currentProjectId"
      @close="addingWorkspace = false"
      @created="handleWorkspaceCreated"
    />

    <DeleteProjectModal
      v-if="deletingProjectId"
      :project-id="deletingProjectId"
      :project-name="deletingProject?.name ?? ''"
      @close="deletingProjectId = ''"
      @deleted="handleDeleted"
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
import AddProjectModal from './components/modals/AddProjectModal.vue'
import AddWorkspaceModal from './components/modals/AddWorkspaceModal.vue'
import DeleteProjectModal from './components/modals/DeleteProjectModal.vue'
import SettingsModal from './components/modals/SettingsModal.vue'
import DebugPanel from './components/DebugPanel.vue'

const projects = ref<Project[]>([])
const currentProjectId = ref('')
const workspacesByProj = ref<Record<string, Workspace[]>>({})
const activeWorkspaceId = ref('')
const settingsOpen = ref(false)
const debugOpen = ref(false)
const addingWorkspace = ref(false)
const addProjectOpen = ref(false)
const deletingProjectId = ref('')

const workspaces = computed(() => workspacesByProj.value[currentProjectId.value] ?? [])
const currentProject = computed(() => projects.value.find(p => p.id === currentProjectId.value))
const deletingProject = computed(() => projects.value.find(p => p.id === deletingProjectId.value))
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

    const pid = new URLSearchParams(location.search).get('project')
    const target = pid && projects.value.find(p => p.id === pid) ? pid : currentProjectId.value
    if (target) {
      currentProjectId.value = target
      await loadWorkspaces(target)
      activeWorkspaceId.value = workspacesByProj.value[target]?.[0]?.id ?? 'reconnaissance'
    }
  } catch { /* server not reachable yet */ }
})

async function loadWorkspaces(pid: string) {
  try {
    const r = await fetch(`/api/workspaces?project_id=${pid}`)
    if (!r.ok) return
    const list: Workspace[] = await r.json()
    const recon: Workspace = { id: 'reconnaissance', name: 'Reconnaissance', icon: 'radar' }
    workspacesByProj.value = {
      ...workspacesByProj.value,
      [pid]: [recon, ...list],
    }
  } catch { /* ignore */ }
}

async function handleSwitchProject(pid: string) {
  currentProjectId.value = pid
  await loadWorkspaces(pid)
  const list = workspacesByProj.value[pid] ?? []
  activeWorkspaceId.value = list[1]?.id ?? list[0]?.id ?? 'reconnaissance'
  setProjectParam(pid)
}

function handleAdded(fsProjects: FsProject[]) {
  const last = fsProjects[fsProjects.length - 1]
  if (last) {
    const url = new URL(location.href)
    url.searchParams.set('project', String(last.id))
    location.href = url.toString()
  } else {
    addProjectOpen.value = false
  }
}

function applyFsProjects(fsProjects: FsProject[]) {
  for (const fp of fsProjects) {
    if (fp.status === 'removed') continue
    const id = String(fp.id)
    const name = fp.path.split('/').pop() ?? fp.path
    const project: Project = {
      id,
      name,
      branch: (fp as any).branch ?? 'main',
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
  }
}

function handleDeleteProject(id: string) {
  deletingProjectId.value = id
}

function handleDeleted(id: string) {
  projects.value = projects.value.filter(p => p.id !== id)
  const { [id]: _removed, ...rest } = workspacesByProj.value
  workspacesByProj.value = rest
  if (currentProjectId.value === id) {
    const next = projects.value[0]
    currentProjectId.value = next?.id ?? ''
    activeWorkspaceId.value = next ? 'reconnaissance' : ''
    if (next) setProjectParam(next.id)
    else removeProjectParam()
  }
  deletingProjectId.value = ''
}

function handleWorkspaceCreated(ws: Workspace) {
  const pid = currentProjectId.value
  workspacesByProj.value = {
    ...workspacesByProj.value,
    [pid]: [...(workspacesByProj.value[pid] ?? []), ws],
  }
  activeWorkspaceId.value = ws.id
  addingWorkspace.value = false
}

function setProjectParam(id: string) {
  const url = new URL(location.href)
  url.searchParams.set('project', id)
  history.replaceState(null, '', url)
}

function removeProjectParam() {
  const url = new URL(location.href)
  url.searchParams.delete('project')
  history.replaceState(null, '', url)
}
</script>
