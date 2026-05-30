<template>
  <div style="display:flex;height:100vh;overflow:hidden;background:var(--color-surface-0)">
    <AppSidebar
      :projects="projects"
      :current-project-id="currentProjectId"
      :workspaces="workspaces"
      :active-workspace-id="activeWorkspaceId"
      @switch-project="handleSwitchProject"
      @select-workspace="handleSelectWorkspace"
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
          v-if="currentProject && !route.params.workspaceId"
          :project="currentProject"
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
      :workspaces="workspaces"
      @close="debugOpen = false"
    />
  </div>
</template>

<script setup lang="ts">
import { ref, computed, watch, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
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

const route = useRoute()
const router = useRouter()

const projects = ref<Project[]>([])
const workspacesByProj = ref<Record<string, Workspace[]>>({})
const settingsOpen = ref(false)
const debugOpen = ref(false)
const addingWorkspace = ref(false)
const addProjectOpen = ref(false)
const deletingProjectId = ref('')

const currentProjectId = computed(() => route.params.projectId as string || '')
const activeWorkspaceId = computed(() => (route.params.workspaceId as string) || 'reconnaissance')

const workspaces = computed(() => workspacesByProj.value[currentProjectId.value] ?? [])
const currentProject = computed(() => projects.value.find(p => p.id === currentProjectId.value))
const deletingProject = computed(() => projects.value.find(p => p.id === deletingProjectId.value))
const activeWorkspace = computed(() => {
  const wsId = route.params.workspaceId as string
  if (!wsId) return null
  return workspaces.value.find(w => w.id === wsId) ?? null
})
const workspaceWithMeta = computed(() => ({
  ...activeWorkspace.value!,
  branch: currentProject.value?.branch ?? 'main',
  commit: '7c3d1a8',
}))

watch(currentProjectId, pid => { if (pid) loadWorkspaces(pid) })

onMounted(async () => {
  try {
    const r = await fetch('/api/fs/projects')
    if (!r.ok) return
    const fsProjects: FsProject[] = await r.json()
    applyFsProjects(fsProjects)

    const pid = new URLSearchParams(location.search).get('project')
    if (pid && projects.value.find(p => p.id === pid)) {
      router.push('/project/' + pid + '/reconnaissance')
    } else if (!route.params.projectId && projects.value.length > 0) {
      router.push('/project/' + projects.value[0].id + '/reconnaissance')
    } else if (route.params.projectId) {
      await loadWorkspaces(currentProjectId.value)
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

function handleSwitchProject(pid: string) {
  router.push('/project/' + pid + '/reconnaissance')
}

function handleAdded(fsProjects: FsProject[]) {
  const last = fsProjects[fsProjects.length - 1]
  applyFsProjects(fsProjects)
  addProjectOpen.value = false
  if (last) {
    router.push('/project/' + String(last.id) + '/reconnaissance')
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
    router.push(next ? '/project/' + next.id + '/reconnaissance' : '/')
  }
  deletingProjectId.value = ''
}

function handleWorkspaceCreated(ws: Workspace) {
  const pid = currentProjectId.value
  workspacesByProj.value = {
    ...workspacesByProj.value,
    [pid]: [...(workspacesByProj.value[pid] ?? []), ws],
  }
  addingWorkspace.value = false
  router.push('/project/' + pid + '/workspace/' + ws.id)
}

function handleSelectWorkspace(wsId: string) {
  if (wsId === 'reconnaissance') {
    router.push('/project/' + currentProjectId.value + '/reconnaissance')
  } else {
    router.push('/project/' + currentProjectId.value + '/workspace/' + wsId)
  }
}
</script>
