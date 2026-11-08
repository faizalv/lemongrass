// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

import { computed, ref } from 'vue'
import type { UpdateState } from '../../preload/types'

// The updater's state for every view that shows it, subscribed once for the app's lifetime. A push that lands
// before the first read wins, since the main process sends every change.
export const updateState = ref<UpdateState | null>(null)

window.api.update.onState((next) => (updateState.value = next))
void window.api.update.state().then((first) => {
  if (!updateState.value) updateState.value = first
})

// Marked on the Settings button and the Updates entry while a newer version is available, or downloaded and
// waiting for a restart.
export const updatePending = computed(
  () => !!updateState.value?.available || updateState.value?.phase === 'installed'
)
