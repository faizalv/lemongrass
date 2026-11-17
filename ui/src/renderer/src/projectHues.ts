// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

import { capsuleHue, isCapsuleHue, leastUsedHue, type CapsuleHue } from '@sankasten'
import type { Project } from '../../preload/types'

// A project's color everywhere it shows: its monogram, the switcher row and the header gradient.
export function projectHue(project: Pick<Project, 'name' | 'hue'>): CapsuleHue {
  return isCapsuleHue(project.hue) ? project.hue : capsuleHue(project.name)
}

// Projects with no kept hue, including one added by the CLI, get the least used hue in list order,
// so the oldest ones keep the color their name already gave them. Returns the list with the hues
// saved, or the same list when nothing was missing or the save failed.
export async function withAssignedHues(projects: Project[]): Promise<Project[]> {
  const used: CapsuleHue[] = projects.map((p) => p.hue).filter(isCapsuleHue)
  const missing: Record<string, string> = {}
  for (const project of projects) {
    if (isCapsuleHue(project.hue)) continue
    const hue = leastUsedHue(project.name, used)
    used.push(hue)
    missing[project.id] = hue
  }
  if (Object.keys(missing).length === 0) return projects
  try {
    return await window.api.projects.setHues(missing)
  } catch {
    return projects.map((p) => (missing[p.id] ? { ...p, hue: missing[p.id] } : p))
  }
}
