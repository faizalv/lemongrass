// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

import { execFile, spawn } from 'node:child_process'
import type { ProjectTextHit, ProjectTextResult } from '../preload/types'

export const GREP_MAX_HITS = 200
export const GREP_MIN_QUERY = 2
export const GREP_LINE_CHARS = 300
// A line whose first match sits further in than this is cut so the match stays in view.
const GREP_CONTEXT_BEFORE = 40

// Which pass a search runs. `repo` honours .gitignore through the index and leaves biblio/ to the
// `biblio` pass, since biblio/ is normally gitignored; `plain` covers a project outside git.
export type GrepPass = 'repo' | 'plain' | 'biblio'

const COMMON = ['-n', '-I', '-i', '-F', '-z', '--no-color']

// Pathspec magic in `repo` and `plain` needs GIT_LITERAL_PATHSPECS off.
export function grepArgs(query: string, pass: GrepPass): string[] {
  const pattern = ['-e', query, '--']
  if (pass === 'repo')
    return ['grep', '--untracked', ...COMMON, ...pattern, '.', ':(exclude)biblio']
  if (pass === 'plain')
    return [
      'grep',
      '--no-index',
      '--exclude-standard',
      ...COMMON,
      ...pattern,
      '.',
      ':(exclude)biblio'
    ]
  return ['grep', '--no-index', ...COMMON, ...pattern, 'biblio']
}

// Every case-insensitive occurrence of `needle` in `text`, as [start, end) ranges.
export function matchRanges(text: string, needle: string): [number, number][] {
  const out: [number, number][] = []
  if (!needle) return out
  const hay = text.toLowerCase()
  const lower = needle.toLowerCase()
  for (let at = hay.indexOf(lower); at !== -1; at = hay.indexOf(lower, at + lower.length)) {
    out.push([at, at + lower.length])
  }
  return out
}

// Trims the line, cuts it before a far-in first match and after GREP_LINE_CHARS, and marks the
// matches left in view. A cut is shown as an ellipsis.
export function clipLine(raw: string, query: string): { text: string; ranges: [number, number][] } {
  let text = raw.replace(/\s+$/, '')
  text = text.slice(text.length - text.trimStart().length)
  const first = matchRanges(text, query)[0]
  let prefix = ''
  if (first && first[0] > GREP_CONTEXT_BEFORE) {
    text = text.slice(first[0] - GREP_CONTEXT_BEFORE)
    prefix = '…'
  }
  let suffix = ''
  if (text.length > GREP_LINE_CHARS) {
    text = text.slice(0, GREP_LINE_CHARS)
    suffix = '…'
  }
  const shown = prefix + text + suffix
  const ranges = matchRanges(text, query)
    .map(([start, end]): [number, number] => [start + prefix.length, end + prefix.length])
    .filter(([, end]) => end <= prefix.length + text.length)
  return { text: shown, ranges }
}

// Parses one `git grep -n -z` record, `path\0line\0text`. Every pass runs at the project root, so
// the path is already project-relative.
export function parseGrepRecord(record: string, query: string): ProjectTextHit | null {
  const pathEnd = record.indexOf('\0')
  if (pathEnd <= 0) return null
  const lineEnd = record.indexOf('\0', pathEnd + 1)
  if (lineEnd === -1) return null
  const line = Number(record.slice(pathEnd + 1, lineEnd))
  if (!Number.isInteger(line) || line < 1) return null
  const path = record.slice(0, pathEnd).replace(/\\/g, '/').replace(/^\.\//, '')
  return { path, line, ...clipLine(record.slice(lineEnd + 1), query) }
}

// How to run git: the binary and the environment, which must turn literal pathspecs off for the
// `:(exclude)biblio` pathspec.
export interface GrepGit {
  binary: string
  env: Record<string, string>
}

const GREP_TIMEOUT_MS = 10_000

const inRepo = new Map<string, boolean>()

function isRepo(root: string, git: GrepGit): Promise<boolean> {
  const known = inRepo.get(root)
  if (known !== undefined) return Promise.resolve(known)
  return new Promise((resolve) => {
    execFile(
      git.binary,
      ['rev-parse', '--is-inside-work-tree'],
      { cwd: root, env: { ...process.env, ...git.env }, windowsHide: true },
      (failure, stdout) => {
        const repo = !failure && String(stdout).trim() === 'true'
        inRepo.set(root, repo)
        resolve(repo)
      }
    )
  })
}

interface Search {
  cancelled: boolean
  kill?: () => void
}

// One search at a time: the search box sends a query per pause in typing, and a new one makes
// the previous answer useless, so its git process is killed rather than left to finish.
let current: Search | null = null

// Streams one pass into `hits` and stops the process once the cap is reached. Resolves true when
// the pass was cut at the cap.
function runPass(
  root: string,
  query: string,
  pass: GrepPass,
  hits: ProjectTextHit[],
  search: Search,
  git: GrepGit
): Promise<boolean> {
  return new Promise((resolve) => {
    const child = spawn(git.binary, grepArgs(query, pass), {
      cwd: root,
      env: { ...process.env, ...git.env },
      windowsHide: true
    })
    let cut = false
    let rest = ''
    const stop = (): void => {
      child.stdout.removeAllListeners('data')
      child.kill()
    }
    search.kill = stop
    const timer = setTimeout(stop, GREP_TIMEOUT_MS)
    child.stdout.setEncoding('utf8')
    child.stdout.on('data', (chunk: string) => {
      const records = (rest + chunk).split('\n')
      rest = records.pop() ?? ''
      for (const record of records) {
        const hit = parseGrepRecord(record, query)
        if (!hit) continue
        if (hits.length >= GREP_MAX_HITS) {
          cut = true
          stop()
          return
        }
        hits.push(hit)
      }
    })
    // git grep exits 1 for no match and 128 for a missing pathspec such as an absent biblio/;
    // either way the hits so far stand.
    child.on('error', () => {
      clearTimeout(timer)
      resolve(cut)
    })
    child.on('close', () => {
      clearTimeout(timer)
      resolve(cut)
    })
  })
}

// Lines containing `query` in the project, then in biblio/, up to GREP_MAX_HITS. A newer call
// cancels this one, which then resolves with `cancelled` and no hits.
export async function grepProject(
  root: string,
  query: string,
  git: GrepGit
): Promise<ProjectTextResult> {
  current?.kill?.()
  if (current) current.cancelled = true
  const search: Search = { cancelled: false }
  current = search
  const hits: ProjectTextHit[] = []
  let truncated = false
  if (query.trim().length >= GREP_MIN_QUERY) {
    const first: GrepPass = (await isRepo(root, git)) ? 'repo' : 'plain'
    if (!search.cancelled) truncated = await runPass(root, query, first, hits, search, git)
    if (!truncated && !search.cancelled) {
      truncated = await runPass(root, query, 'biblio', hits, search, git)
    }
  }
  if (current === search) current = null
  return search.cancelled ? { hits: [], truncated: false, cancelled: true } : { hits, truncated }
}
