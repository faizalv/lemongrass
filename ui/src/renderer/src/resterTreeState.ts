// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

import { reactive } from 'vue'

// What the human set up in a request's response box, per request. A project switch unmounts the
// Rester Client tab, so this module is what keeps it. Only an explicit choice is stored: a node
// never touched keeps its default depth.

// Which tree nodes the human opened or closed by hand, as path id to open.
const toggles = reactive<Record<string, Record<string, boolean>>>({})

export function treeToggleOf(requestId: string, nodeId: string): boolean | undefined {
  return toggles[requestId]?.[nodeId]
}

export function setTreeToggle(requestId: string, nodeId: string, open: boolean): void {
  toggles[requestId] ??= {}
  toggles[requestId][nodeId] = open
}

export interface ResponseView {
  view: 'tree' | 'raw' | 'headers'
  // The search text, and whether the tree shows only its matches.
  query: string
  filter: boolean
}

const DEFAULT_VIEW: Readonly<ResponseView> = { view: 'tree', query: '', filter: false }
const views = reactive<Record<string, ResponseView>>({})

export function responseViewOf(requestId: string): Readonly<ResponseView> {
  return views[requestId] ?? DEFAULT_VIEW
}

export function setResponseView(requestId: string, patch: Partial<ResponseView>): void {
  views[requestId] = { ...responseViewOf(requestId), ...patch }
}

export function dropResponseState(requestId: string): void {
  delete toggles[requestId]
  delete views[requestId]
}
