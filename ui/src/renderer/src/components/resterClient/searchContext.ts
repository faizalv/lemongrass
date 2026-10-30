// SPDX-License-Identifier: AGPL-3.0-only
// Copyright (C) 2026 The Lemongrass Authors

import type { InjectionKey } from 'vue'
import type { SearchMatch } from '../../responseSearch'

// What ResponsePanel shares with every JsonNode while a search runs. version changes with each
// new search, so a node drops a manual open or close made for the previous one.
export interface TreeSearch {
  active: boolean
  byPath: Map<string, SearchMatch>
  ancestors: Set<string>
  currentId: string | null
  filter: boolean
  version: number
}

export const treeSearchKey: InjectionKey<TreeSearch> = Symbol('treeSearch')
