import { ipcMain } from 'electron'
import { execFile, type ExecFileException } from 'child_process'
import { resolve, sep } from 'path'
import { parseDiff, parseStatus } from './gitParse'
import type { GitActionResult, GitDiff, GitStatus } from '../preload/types'

const EMPTY_TREE = '4b825dc642cb6eb9a060e54bf8d69288fbee4904'
const READ_TIMEOUT_MS = 15_000
const WRITE_TIMEOUT_MS = 120_000
const PUSH_TIMEOUT_MS = 60_000
const MAX_DIFF_BYTES = 4 * 1024 * 1024
const MAX_OUTPUT_BYTES = 16 * 1024 * 1024

interface RunResult {
  code: number | null
  stdout: string
  stderr: string
  overflow: boolean
}

interface RunOptions {
  input?: string
  timeout?: number
  maxBuffer?: number
  env?: Record<string, string>
}

function run(cwd: string, args: string[], options: RunOptions = {}): Promise<RunResult> {
  return new Promise((resolvePromise) => {
    const child = execFile(
      'git',
      args,
      {
        cwd,
        encoding: 'utf8',
        timeout: options.timeout ?? READ_TIMEOUT_MS,
        maxBuffer: options.maxBuffer ?? MAX_OUTPUT_BYTES,
        env: { ...process.env, ...options.env }
      },
      (failure: ExecFileException | null, stdout, stderr) => {
        if (!failure) return resolvePromise({ code: 0, stdout, stderr, overflow: false })
        if (failure.code === 'ENOENT') {
          return resolvePromise({
            code: null,
            stdout: '',
            stderr: 'git was not found on this machine.',
            overflow: false
          })
        }
        resolvePromise({
          code: typeof failure.code === 'number' ? failure.code : null,
          stdout,
          stderr: stderr || failure.message,
          overflow: failure.code === 'ERR_CHILD_PROCESS_STDIO_MAXBUFFER'
        })
      }
    )
    if (options.input !== undefined) child.stdin?.end(options.input)
  })
}

const readEnv = { GIT_OPTIONAL_LOCKS: '0', LC_ALL: 'C', GIT_LITERAL_PATHSPECS: '1' }
const pushEnv = { GIT_TERMINAL_PROMPT: '0' }

const topLevels = new Map<string, string>()

async function repoRoot(projectPath: string): Promise<string | null> {
  const cached = topLevels.get(projectPath)
  if (cached) return cached
  const result = await run(projectPath, ['rev-parse', '--show-toplevel'], { env: readEnv })
  const root = result.code === 0 ? result.stdout.trim() : ''
  if (!root) return null
  topLevels.set(projectPath, root)
  return root
}

function isInside(root: string, relativePath: string): boolean {
  if (!relativePath || relativePath.includes('\0')) return false
  const target = resolve(root, relativePath)
  return target.startsWith(root + sep)
}

function errorText(result: RunResult): string {
  return result.stderr.trim() || result.stdout.trim() || 'git failed without a message.'
}

const notARepo: GitStatus = {
  isRepo: false,
  branch: null,
  detached: false,
  upstream: null,
  ahead: 0,
  behind: 0,
  hasCommits: false,
  pushTarget: null,
  files: []
}

async function pushRemote(root: string): Promise<string | null> {
  const result = await run(root, ['remote'], { env: readEnv })
  const remotes = result.stdout.split('\n').filter(Boolean)
  if (remotes.length === 0) return null
  return remotes.includes('origin') ? 'origin' : remotes[0]
}

async function status(projectPath: string): Promise<GitStatus> {
  const root = await repoRoot(projectPath)
  if (!root) return notARepo
  const result = await run(
    root,
    ['status', '--porcelain=v2', '--branch', '-z', '--untracked-files=all'],
    { env: readEnv }
  )
  if (result.code !== 0) {
    topLevels.delete(projectPath)
    return notARepo
  }
  const parsed = parseStatus(result.stdout)
  let pushTarget = parsed.upstream
  if (!pushTarget && parsed.branch) {
    const remote = await pushRemote(root)
    pushTarget = remote ? `${remote}/${parsed.branch}` : null
  }
  return { isRepo: true, ...parsed, pushTarget }
}

async function isUntracked(root: string, path: string): Promise<boolean> {
  const result = await run(root, ['ls-files', '--others', '--exclude-standard', '-z', '--', path], {
    env: readEnv
  })
  return result.code === 0 && result.stdout.split('\0').includes(path)
}

async function diff(projectPath: string, path: string, origPath?: string): Promise<GitDiff> {
  const root = await repoRoot(projectPath)
  if (!root) return { status: 'error', message: 'This project is not a git repository.' }
  if (!isInside(root, path) || (origPath !== undefined && !isInside(root, origPath))) {
    return { status: 'error', message: 'That path is outside the repository.' }
  }

  const untracked = await isUntracked(root, path)
  let result: RunResult
  if (untracked) {
    result = await run(
      root,
      ['diff', '--no-index', '--no-color', '--no-ext-diff', '-U3', '--', '/dev/null', path],
      { env: readEnv, maxBuffer: MAX_DIFF_BYTES }
    )
  } else {
    const head = await run(root, ['rev-parse', '--verify', '--quiet', 'HEAD'], { env: readEnv })
    const base = head.code === 0 ? 'HEAD' : EMPTY_TREE
    const paths = origPath ? [path, origPath] : [path]
    result = await run(
      root,
      ['diff', base, '--no-color', '--no-ext-diff', '--no-textconv', '-M', '-U3', '--', ...paths],
      { env: readEnv, maxBuffer: MAX_DIFF_BYTES }
    )
  }
  if (result.overflow) return { status: 'too-large' }
  // A no-index diff exits with 1 when the files differ.
  if (result.code !== 0 && !(untracked && result.code === 1)) {
    return { status: 'error', message: errorText(result) }
  }
  return parseDiff(result.stdout)
}

async function commit(
  projectPath: string,
  message: string,
  paths?: string[]
): Promise<GitActionResult> {
  const root = await repoRoot(projectPath)
  if (!root) return { ok: false, error: 'This project is not a git repository.' }
  const text = message.trim()
  if (!text) return { ok: false, error: 'The commit message is empty.' }
  if (paths && (paths.length === 0 || !paths.every((path) => isInside(root, path)))) {
    return { ok: false, error: 'The selected files are not valid paths in this repository.' }
  }

  // With paths, only untracked files need adding. The path-limited commit reads every tracked
  // path from the worktree and leaves all other files, staged or not, untouched. Adding tracked
  // paths would fail for a staged rename or deletion, whose old path is in neither index nor worktree.
  const env = { GIT_LITERAL_PATHSPECS: '1' }
  let toAdd = ['-A']
  if (paths) {
    const listed = await run(
      root,
      ['ls-files', '--others', '--exclude-standard', '-z', '--', ...paths],
      { env }
    )
    if (listed.code !== 0) return { ok: false, error: errorText(listed) }
    const untracked = listed.stdout.split('\0').filter(Boolean)
    toAdd = untracked.length > 0 ? ['-A', '--', ...untracked] : []
  }
  if (toAdd.length > 0) {
    const add = await run(root, ['add', ...toAdd], { timeout: WRITE_TIMEOUT_MS, env })
    if (add.code !== 0) return { ok: false, error: errorText(add) }
  }
  const result = await run(root, ['commit', '-F', '-', ...(paths ? ['--', ...paths] : [])], {
    input: `${text}\n`,
    timeout: WRITE_TIMEOUT_MS,
    env
  })
  if (result.code !== 0) return { ok: false, error: errorText(result) }
  return { ok: true }
}

async function push(projectPath: string): Promise<GitActionResult> {
  const root = await repoRoot(projectPath)
  if (!root) return { ok: false, error: 'This project is not a git repository.' }

  const branchResult = await run(root, ['symbolic-ref', '--short', '-q', 'HEAD'], {
    env: readEnv
  })
  const branch = branchResult.stdout.trim()
  if (branchResult.code !== 0 || !branch) {
    return { ok: false, error: 'HEAD is detached, so there is no branch to push.' }
  }

  const upstream = await run(root, ['rev-parse', '--abbrev-ref', '--symbolic-full-name', '@{u}'], {
    env: readEnv
  })
  let args: string[]
  if (upstream.code === 0) {
    args = ['push']
  } else {
    const remote = await pushRemote(root)
    if (!remote) return { ok: false, error: 'This repository has no remote to push to.' }
    args = ['push', '-u', remote, 'HEAD']
  }
  const result = await run(root, args, { timeout: PUSH_TIMEOUT_MS, env: pushEnv })
  if (result.code !== 0) return { ok: false, error: errorText(result) }
  return { ok: true }
}

export function registerGitHandlers(): void {
  ipcMain.handle('git:status', (_event, projectPath: string) => status(projectPath))
  ipcMain.handle('git:diff', (_event, projectPath: string, path: string, origPath?: string) =>
    diff(projectPath, path, origPath)
  )
  ipcMain.handle('git:commit', (_event, projectPath: string, message: string, paths?: string[]) =>
    commit(projectPath, message, paths)
  )
  ipcMain.handle('git:push', (_event, projectPath: string) => push(projectPath))
}
