// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

const PLANS_HEADING = /^##\s+Plans\b/
const NEXT_HEADING = /^##\s/
// Plain or backslash-escaped brackets, since an editor round trip can escape them.
const ITEM = /^\s*[-*+]\s+\\?\[([ xX])\\?\]\s*(.*)$/
const APPROVAL = /^user approval\b/i

// done: every Plans item checked. approval: only the user approval item is open, so the work is
// built and waits for the user. open: anything else, a PRD with no checkbox at all included.
export type PlanStatus = 'done' | 'approval' | 'open'

// The Plans section of a PRD, or the whole text when it has none.
function plansOf(markdown: string): string[] {
  const lines = markdown.split('\n')
  const start = lines.findIndex((line) => PLANS_HEADING.test(line))
  if (start === -1) return lines
  const rest = lines.slice(start + 1)
  const end = rest.findIndex((line) => NEXT_HEADING.test(line))
  return end === -1 ? rest : rest.slice(0, end)
}

export function planStatus(markdown: string): PlanStatus {
  const items = plansOf(markdown).flatMap((line) => {
    const match = ITEM.exec(line)
    return match ? [{ checked: match[1] !== ' ', text: match[2] }] : []
  })
  const open = items.filter((item) => !item.checked)
  if (items.length === 0) return 'open'
  if (open.length === 0) return 'done'
  return open.every((item) => APPROVAL.test(item.text)) ? 'approval' : 'open'
}
