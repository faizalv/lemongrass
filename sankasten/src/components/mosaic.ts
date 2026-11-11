// SPDX-License-Identifier: Apache-2.0
// Copyright (C) 2026 The Sankasten Authors

import type { InjectionKey, Ref } from 'vue'

// What an AppMosaic hands its capsules: which one is expanded, one at a time.
export interface MosaicContext {
  expanded: Ref<string | null>
  // Expands the capsule, or collapses it when it is the expanded one.
  toggle: (value: string) => void
}

export const MOSAIC_KEY: InjectionKey<MosaicContext> = Symbol('mosaic')
