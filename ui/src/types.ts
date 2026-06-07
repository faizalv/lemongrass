export interface Project {
  id: string
  name: string
  branch: string
  shortPath: string
}

export interface FsNode {
  name: string
  path: string
  children: FsNode[]
}

export interface FsProject {
  id: number
  path: string
  status: 'pending' | 'active' | 'missing' | 'removed'
  created_at: string
}

export type WorkspaceStatus = 'idle' | 'grooming' | 'awaiting_execution' | 'executing' | 'done' | 'deleted' | 'planning' | 'testing' | 'error'

export interface Workspace {
  id: string
  project_id?: string
  name: string
  icon?: string
  status?: WorkspaceStatus
}

export interface WorkspaceRequirement {
  id: string
  workspace_id: string
  type: 'text' | 'pdf' | 'image'
  text_content?: string
  file_name?: string
  created_at: string
}

export type StepStatus = 'ok' | 'miss' | 'pending'

export interface ReconStep {
  label: string
  detail?: string
  status: StepStatus
}

export interface TaskFile {
  path: string
  range: string
  note: string
}

export interface Task {
  id: string
  title: string
  prdRef: string
  howTo: string
  files: TaskFile[]
  estTokens: string
  idx?: number
}

export type Decision = 'accept' | 'reject' | 'correction' | null

export type GroomingPhase =
  | 'idle'
  | 'reading_recon'
  | 'permission'
  | 'recon_running'
  | 'generating_tasks'
  | 'reviewing'
  | 'done'

export type TaskCommitStatus = 'open' | 'committing' | 'committed'

export interface SemanticNode {
  id: string
  file_path: string
  line_start: number
  line_end: number
  package: string
  symbol: string
  kind: string
  language: string
  receiver?: string
  signature?: string
  status: string
  description?: string
}

export interface LangCoverage {
  language: string
  total: number
  explored: number
  stale: number
}

export interface ReconTreeNode {
  name:     string
  path:     string
  isDir:    boolean
  children: ReconTreeNode[]
  explored: number
  stale:    number
  total:    number
}

export interface ApiTask {
  id: string
  workspace_id: string
  title: string
  reason: string
  impl: string[]
  status: 'pending' | 'approved' | 'rejected'
  created_at: string
  approved_at?: string
}

export interface ProjectArtifact {
  id: string
  type: string
  name: string
  content: string
  version: number
  created_at: string
}

export interface KnowledgeEntry {
  key: string
  content: string
  updated_at: string
}

export interface WorkspaceWithRequirements {
  id: string
  project_id: number
  name: string
  status: WorkspaceStatus
  created_at: string
  updated_at: string
  requirements: WorkspaceRequirement[]
}
