// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

// A sidebar tree row dragged onto a shell types its project-relative path there. It travels in its
// own type beside a tree's internal move type, so a shell never reacts to a tab drag or a move.
export const PATH_DRAG_MIME = 'application/x-lemongrass-shell-path'

export function setPathDrag(event: DragEvent, projectRelativePath: string): void {
  event.dataTransfer?.setData(PATH_DRAG_MIME, projectRelativePath)
}

export function isPathDrag(event: DragEvent): boolean {
  return event.dataTransfer?.types.includes(PATH_DRAG_MIME) ?? false
}

// Single-quoted when it holds anything a POSIX shell would split or expand, so the pasted path is
// one word for a shell and an agent CLI alike.
export function quotePath(path: string): string {
  if (/^[\w./@%+=:,-]+$/.test(path)) return path
  return `'${path.replaceAll("'", `'\\''`)}'`
}
