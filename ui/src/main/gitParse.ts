import type {
  GitDiff,
  GitDiffHunk,
  GitDiffLine,
  GitDiffRow,
  GitFile,
  GitFileKind
} from '../preload/types'

export interface ParsedStatus {
  branch: string | null
  detached: boolean
  upstream: string | null
  ahead: number
  behind: number
  hasCommits: boolean
  files: GitFile[]
}

function kindOf(xy: string): GitFileKind {
  const [index, worktree] = [xy[0], xy[1]]
  if (index === 'D' || worktree === 'D') return 'deleted'
  if (index === 'A') return 'added'
  if (index === 'R' || worktree === 'R') return 'renamed'
  if (index === 'C' || worktree === 'C') return 'copied'
  if (index === 'T' || worktree === 'T') return 'typechange'
  return 'modified'
}

export function parseStatus(raw: string): ParsedStatus {
  const result: ParsedStatus = {
    branch: null,
    detached: false,
    upstream: null,
    ahead: 0,
    behind: 0,
    hasCommits: true,
    files: []
  }
  const records = raw.split('\0')
  for (let i = 0; i < records.length; i++) {
    const record = records[i]
    if (!record) continue
    if (record.startsWith('# ')) {
      const [key, ...rest] = record.slice(2).split(' ')
      const value = rest.join(' ')
      if (key === 'branch.oid') result.hasCommits = value !== '(initial)'
      else if (key === 'branch.head') {
        result.detached = value === '(detached)'
        result.branch = result.detached ? null : value
      } else if (key === 'branch.upstream') result.upstream = value
      else if (key === 'branch.ab') {
        const match = /^\+(\d+) -(\d+)$/.exec(value)
        if (match) {
          result.ahead = Number(match[1])
          result.behind = Number(match[2])
        }
      }
      continue
    }
    const type = record[0]
    if (type === '?') {
      result.files.push({ path: record.slice(2), kind: 'untracked' })
    } else if (type === '1') {
      const fields = record.split(' ')
      result.files.push({ path: fields.slice(8).join(' '), kind: kindOf(fields[1]) })
    } else if (type === '2') {
      const fields = record.split(' ')
      const origPath = records[++i]
      result.files.push({
        path: fields.slice(9).join(' '),
        origPath,
        kind: kindOf(fields[1])
      })
    } else if (type === 'u') {
      const fields = record.split(' ')
      result.files.push({ path: fields.slice(10).join(' '), kind: 'conflicted' })
    }
  }
  result.files.sort((a, b) => a.path.localeCompare(b.path))
  return result
}

const HUNK_HEADER = /^@@ -(\d+)(?:,\d+)? \+(\d+)(?:,\d+)? @@/

function stripCarriageReturn(text: string): string {
  return text.endsWith('\r') ? text.slice(0, -1) : text
}

export function parseDiff(raw: string): GitDiff {
  if (!raw.trim()) return { status: 'empty' }
  const lines = raw.split('\n')
  if (
    lines.some((line) => line.startsWith('Binary files ') || line.startsWith('GIT binary patch'))
  ) {
    return { status: 'binary' }
  }

  const hunks: GitDiffHunk[] = []
  let current: GitDiffHunk | null = null
  let oldNumber = 0
  let newNumber = 0
  let removals: GitDiffLine[] = []
  let additions: GitDiffLine[] = []
  let added = 0
  let removed = 0

  const flush = (): void => {
    if (!current) return
    const count = Math.max(removals.length, additions.length)
    for (let i = 0; i < count; i++) {
      const row: GitDiffRow = { left: removals[i] ?? null, right: additions[i] ?? null }
      current.rows.push(row)
    }
    removals = []
    additions = []
  }

  for (const line of lines) {
    const header = HUNK_HEADER.exec(line)
    if (header) {
      flush()
      current = { header: line, rows: [] }
      hunks.push(current)
      oldNumber = Number(header[1])
      newNumber = Number(header[2])
      continue
    }
    if (!current || line.length === 0 || line.startsWith('\\')) continue
    const marker = line[0]
    const text = stripCarriageReturn(line.slice(1))
    if (marker === '-') {
      removals.push({ number: oldNumber++, text, type: 'del' })
      removed++
    } else if (marker === '+') {
      additions.push({ number: newNumber++, text, type: 'add' })
      added++
    } else if (marker === ' ') {
      flush()
      current.rows.push({
        left: { number: oldNumber++, text, type: 'context' },
        right: { number: newNumber++, text, type: 'context' }
      })
    }
  }
  flush()

  if (hunks.length === 0) return { status: 'empty' }
  return { status: 'ok', hunks, added, removed }
}
