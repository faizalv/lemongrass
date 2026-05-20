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

export type WorkspaceStatus = 'idle' | 'grooming' | 'planning' | 'testing' | 'error'

export interface Workspace {
  id: string
  name: string
  icon: string
  status?: WorkspaceStatus
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
}
