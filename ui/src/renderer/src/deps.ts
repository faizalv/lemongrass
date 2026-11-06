// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

import { reactive } from 'vue'
import type { DepsState } from '../../preload/types'

// Main pushes every change, so the startup layer and the Settings section read one copy with no poll.
export const depsStore = reactive<{ state: DepsState | null }>({ state: null })

export function startDepsFeed(): void {
  window.api.deps.onState((next) => (depsStore.state = next))
  void window.api.deps.state().then((next) => {
    depsStore.state ??= next
  })
}
