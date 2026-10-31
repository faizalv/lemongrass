// SPDX-License-Identifier: AGPL-3.0-only
// Copyright (C) 2026 The Lemongrass Authors

import { reactive } from 'vue'

// The tab a picker row is pointing at while hovered. Its pane draws a glow on the pane body when
// the tab is the one shown, or on its label in the tab strip when it is not, so the human can see
// which tab a row means among many. Tab ids are unique across projects.
export const tabGlow = reactive<{ tabId: string | null }>({ tabId: null })

export function glowTab(tabId: string | null): void {
  tabGlow.tabId = tabId
}
