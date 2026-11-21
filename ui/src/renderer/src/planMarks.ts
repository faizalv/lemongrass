// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

const PLANS_HEADING = /^##\s+Plans\b/
const NEXT_HEADING = /^##\s/
// Plain or backslash-escaped brackets, since an editor round trip can escape them.
const UNCHECKED = /^\s*[-*+]\s+\\?\[ \\?\]/m
const CHECKED = /^\s*[-*+]\s+\\?\[[xX]\\?\]/m

// The Plans section of a PRD, or the whole text when it has none.
function plansOf(markdown: string): string {
  const lines = markdown.split('\n')
  const start = lines.findIndex((line) => PLANS_HEADING.test(line))
  if (start === -1) return markdown
  const rest = lines.slice(start + 1)
  const end = rest.findIndex((line) => NEXT_HEADING.test(line))
  return (end === -1 ? rest : rest.slice(0, end)).join('\n')
}

// True when a PRD's Plans hold at least one checked item and no unchecked one, the same bar the
// task close checks before it prints any step.
export function plansAllChecked(markdown: string): boolean {
  const plans = plansOf(markdown)
  return CHECKED.test(plans) && !UNCHECKED.test(plans)
}
