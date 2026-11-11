// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

import { reactive } from 'vue'
import type { UiState } from '../../preload/types'

export type SidebarView = 'biblio' | 'files'
// The view the sidebar column shows, one at a time, picked from the side toolbar's upper group.
export type SidebarPane = NonNullable<UiState['sidebarPane']>
export type SidebarPanel = 'git'
export type ExplorerView = NonNullable<UiState['explorer']>
export type ExplorerColumn = keyof ExplorerView['columns']

const DEFAULT_VIEW: ExplorerView = {
  columns: { size: false, modified: false, git: true },
  density: 'compact',
  hideIgnored: false
}

export const sidebarSettings = reactive<{
  view: SidebarView
  pane: SidebarPane
  // Hidden for the session only, as the column always comes back on a restart.
  hidden: boolean
  panel: SidebarPanel | null
  // Sizes the human dragged, kept across a remount and a restart; null leaves the default.
  foldHeight: number
  commitHeight: number | null
  explorer: ExplorerView
}>({
  view: 'biblio',
  pane: 'files',
  hidden: false,
  panel: null,
  foldHeight: 300,
  commitHeight: null,
  explorer: structuredClone(DEFAULT_VIEW)
})

let loaded = false

export async function loadSidebarSettings(): Promise<void> {
  if (loaded) return
  loaded = true
  const saved = await window.api.uiState.load()
  if (saved.sidebarView) sidebarSettings.view = saved.sidebarView
  if (saved.sidebarPane) sidebarSettings.pane = saved.sidebarPane
  // An older file may name 'workgroup', which is now a view and no longer a fold.
  if (saved.sidebarPanel === 'git') sidebarSettings.panel = 'git'
  if (saved.sidebarFoldHeight) sidebarSettings.foldHeight = saved.sidebarFoldHeight
  if (saved.gitCommitHeight) sidebarSettings.commitHeight = saved.gitCommitHeight
  if (saved.explorer) {
    sidebarSettings.explorer = {
      ...DEFAULT_VIEW,
      ...saved.explorer,
      columns: { ...DEFAULT_VIEW.columns, ...saved.explorer.columns }
    }
  }
}

export function setSidebarView(view: SidebarView): void {
  sidebarSettings.view = view
  window.api.uiState.save({ sidebarView: view })
}

// Clicking the shown view's icon hides the column, and any view's icon shows it again.
export function selectSidebarPane(pane: SidebarPane): void {
  if (pane === sidebarSettings.pane && !sidebarSettings.hidden) {
    sidebarSettings.hidden = true
    return
  }
  sidebarSettings.hidden = false
  sidebarSettings.pane = pane
  window.api.uiState.save({ sidebarPane: pane })
}

// Clicking the open panel's icon folds it shut, and while the column is hidden it shows the column
// with the panel open; 'none' is stored because the file merges by field and an absent field would
// keep the old panel.
export function toggleSidebarPanel(panel: SidebarPanel): void {
  if (sidebarSettings.hidden) {
    sidebarSettings.hidden = false
    sidebarSettings.panel = panel
  } else {
    sidebarSettings.panel = sidebarSettings.panel === panel ? null : panel
  }
  window.api.uiState.save({ sidebarPanel: sidebarSettings.panel ?? 'none' })
}

export function setFoldHeight(height: number): void {
  sidebarSettings.foldHeight = height
  window.api.uiState.save({ sidebarFoldHeight: height })
}

export function setCommitHeight(height: number | null): void {
  sidebarSettings.commitHeight = height
  if (height) window.api.uiState.save({ gitCommitHeight: height })
}

// ui-state.json merges per top-level field, so the whole explorer object is sent every time.
function saveExplorer(): void {
  window.api.uiState.save({ explorer: JSON.parse(JSON.stringify(sidebarSettings.explorer)) })
}

export function toggleColumn(column: ExplorerColumn): void {
  sidebarSettings.explorer.columns[column] = !sidebarSettings.explorer.columns[column]
  saveExplorer()
}

export function setDensity(density: ExplorerView['density']): void {
  sidebarSettings.explorer.density = density
  saveExplorer()
}

export function toggleHideIgnored(): void {
  sidebarSettings.explorer.hideIgnored = !sidebarSettings.explorer.hideIgnored
  saveExplorer()
}
