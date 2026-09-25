export interface ShellAgent {
  id: string
  command: string
  label: string
  hint: string
}

export const SHELL_AGENTS: readonly ShellAgent[] = [
  {
    id: 'claude',
    command: 'claude',
    label: 'Claude Code',
    hint: 'claude'
  },
  {
    id: 'codex',
    command: 'codex',
    label: 'Codex',
    hint: 'codex'
  },
  {
    id: 'cursor',
    command: 'agent',
    label: 'Cursor Agent',
    hint: 'agent'
  },
  {
    id: 'shell',
    command: 'zsh',
    label: 'Shell',
    hint: 'zsh'
  }
]

const RESUME_ARGV: Record<string, (sessionId: string) => string[]> = {
  claude: (sessionId) => ['--resume', sessionId],
  codex: (sessionId) => ['resume', sessionId]
}

export function spawnArgs(command: string, tabId: string, args?: string[]): string[] | undefined {
  const marker =
    command === 'codex'
      ? ['--no-daemon', '-c', `shell_environment_policy.set.LGRASS_TAB_ID=${JSON.stringify(tabId)}`]
      : []
  const result = [...marker, ...(args ?? [])]
  return result.length > 0 ? result : undefined
}

export function isResumableCommand(command: string): boolean {
  return command in RESUME_ARGV
}

export function resumeArgs(command: string, sessionId: string): string[] | null {
  return RESUME_ARGV[command]?.(sessionId) ?? null
}

const PREF_KEY = 'lemongrass.shellAgentId'

export function preferredShellAgentId(): string {
  try {
    const saved = localStorage.getItem(PREF_KEY)
    if (saved && SHELL_AGENTS.some((agent) => agent.id === saved)) return saved
  } catch {
    // ignore
  }
  return 'claude'
}

export function setPreferredShellAgentId(id: string): void {
  try {
    localStorage.setItem(PREF_KEY, id)
  } catch {
    // ignore
  }
}

export function shellAgentById(id: string): ShellAgent {
  return SHELL_AGENTS.find((agent) => agent.id === id) ?? SHELL_AGENTS[0]
}
