// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

import { reactive } from 'vue'
import type { Ask, Project } from '../../../../preload/types'
import { capitalizeParts, plainText, tabParts, type NamePart } from '../../tabNames'
import { lastSegment, pathBelow } from '../../pathForm'
import { isWindows } from '../../platform'
import { shellTabById } from '../../workspace'

// Who asked and where, in words a person reads without the ask id: the tab by its quoted title or its
// workgroup role, and the project the asking tab belongs to.

const projects = reactive<{ list: Project[] }>({ list: [] })

export function loadAskProjects(): void {
  window.api.projects
    .list()
    .then((list) => (projects.list = list))
    .catch(() => undefined)
}

export interface AskOrigin {
  project: string
  // The cwd below the project root, '/'-separated, empty at the root or when outside.
  folder: string
  // True when the asking tab's cwd has left its project, so a command would run elsewhere.
  outside: boolean
}

// The cwd below whichever spelling of the project root holds it: the registered path or its real path,
// since an agent's cwd arrives resolved after a cd while the project may be registered through a symlink.
function folderBelow(project: { path: string; realPath?: string }, cwd: string): string | null {
  for (const root of [project.path, project.realPath]) {
    if (!root) continue
    const below = pathBelow(root, cwd, isWindows)
    if (below !== null) return below
  }
  return null
}

// The project of the tab that asked, since the ask's cwd follows the agent's cd anywhere. A closed tab
// falls back to the project holding the cwd, and a cwd outside every project names its whole folder.
export function askProject(ask: Ask): AskOrigin {
  const cwd = ask.cwd
  const found = shellTabById(ask.tab_id)
  if (found) {
    const listed = projects.list.find((p) => p.id === found.project.id)
    const name = listed?.name ?? lastSegment(found.project.path, isWindows)
    const folder = folderBelow(listed ?? found.project, cwd)
    return { project: name, folder: folder ?? '', outside: folder === null }
  }
  const owner = projects.list
    .map((p) => ({ p, folder: folderBelow(p, cwd) }))
    .filter((o) => o.folder !== null)
    .sort((a, b) => b.p.path.length - a.p.path.length)[0]
  if (!owner) return { project: cwd, folder: '', outside: false }
  return { project: owner.p.name, folder: owner.folder ?? '', outside: false }
}

// "lemongrass", or "the ui folder of lemongrass" below the project root.
export function askWhere(ask: Ask): string {
  const { project, folder } = askProject(ask)
  return folder ? `the ${folder} folder of ${project}` : project
}

function wants(ask: Ask): string {
  // The tables show as capsules in the grant view, so the sentence stays short however many there are.
  if (ask.type === 'db') return ' wants a database channel'
  if (ask.type === 'rester') return ' wants a rester channel'
  return ' wants to run a command'
}

// The headline an ask opens with, such as: Claude in "Fix login" tab wants to run a command.
export function askParts(ask: Ask): NamePart[] {
  return capitalizeParts([...tabParts(ask.tab_id), { text: `${wants(ask)}.` }])
}

// The same sentence as plain text with the project, for lists that span projects.
export function askSentence(ask: Ask): string {
  const parts = capitalizeParts([...tabParts(ask.tab_id), { text: wants(ask) }])
  return `${plainText(parts)} in ${askWhere(ask)}.`
}
