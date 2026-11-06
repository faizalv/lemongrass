// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

export function userHandle(name: string): string {
  return name.toLowerCase().split(/\s+/).filter(Boolean).join('_')
}

export function collidingUserName(names: string[], index: number): string | null {
  const handle = userHandle(names[index])
  if (handle === '') return null
  const other = names.findIndex((name, i) => i !== index && userHandle(name) === handle)
  return other === -1 ? null : names[other]
}
