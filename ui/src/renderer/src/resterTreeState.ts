// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

import { reactive } from 'vue'

// Which response tree nodes the human opened or closed by hand, per request, as path id to open.
// A project switch unmounts the Rester Client tab, so this module is what keeps them. Only an
// explicit choice is stored: a node never touched keeps its default depth.
const toggles = reactive<Record<string, Record<string, boolean>>>({})

export function treeToggleOf(requestId: string, nodeId: string): boolean | undefined {
  return toggles[requestId]?.[nodeId]
}

export function setTreeToggle(requestId: string, nodeId: string, open: boolean): void {
  toggles[requestId] ??= {}
  toggles[requestId][nodeId] = open
}

export function dropTreeToggles(requestId: string): void {
  delete toggles[requestId]
}
