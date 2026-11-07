// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

import { computed, ref, shallowRef, watch, type Ref } from 'vue'
import {
  buildSearchTree,
  normalizeQuery,
  type SearchCandidate,
  type SearchResult,
  type SearchRow
} from './treeSearch'
import { useSearchField, type SearchField } from './useSearchField'

export interface CandidateSet {
  items: SearchCandidate[]
  // The source stopped early, so better matches may exist beyond what is shown.
  truncated?: boolean
}

export interface TreeSearchOptions {
  candidates: (query: string) => CandidateSet | Promise<CandidateSet>
  open: (row: SearchRow) => void
  // Where focus returns when the search closes, the panel itself.
  panel: Ref<HTMLElement | null>
  debounceMs?: number
}

export interface TreeSearch extends SearchField {
  result: Ref<SearchResult | null>
  truncated: Ref<boolean>
  cursor: Ref<number>
  status: Ref<string>
  refresh: () => void
  pick: (index: number) => void
}

// The Files view's search: the field from useSearchField, and the panel shows the ranked fuzzy
// matches as a pruned tree in place of the listing.
export function useTreeSearch(options: TreeSearchOptions): TreeSearch {
  const result = shallowRef<SearchResult | null>(null)
  const truncated = ref(false)
  const cursor = ref(0)

  let sequence = 0
  let timer: ReturnType<typeof setTimeout> | undefined

  const field = useSearchField({
    panel: options.panel,
    move,
    enter: () => {
      const row = result.value?.rows[cursor.value]
      if (row) options.open(row)
    },
    onClose: () => {
      clearTimeout(timer)
      sequence++
      result.value = null
      cursor.value = 0
    }
  })

  async function run(): Promise<void> {
    const mine = ++sequence
    const text = field.query.value
    if (!normalizeQuery(text)) {
      result.value = null
      truncated.value = false
      return
    }
    const set = await options.candidates(text)
    // A slower reply for an older query never replaces a newer one.
    if (mine !== sequence) return
    result.value = buildSearchTree(set.items, text)
    truncated.value = !!set.truncated
    cursor.value = Math.min(cursor.value, Math.max(0, result.value.rows.length - 1))
  }

  function schedule(): void {
    clearTimeout(timer)
    cursor.value = 0
    timer = setTimeout(() => void run(), options.debounceMs ?? 0)
  }

  watch(field.query, schedule)

  function refresh(): void {
    if (field.active.value) void run()
  }

  function move(delta: number): void {
    const rows = result.value?.rows.length ?? 0
    if (!rows) return
    cursor.value = (cursor.value + delta + rows) % rows
  }

  function pick(index: number): void {
    const row = result.value?.rows[index]
    if (!row) return
    cursor.value = index
    options.open(row)
  }

  const status = computed((): string => {
    if (!result.value) return ''
    const matches = result.value.rows.filter((row) => row.node.match).length
    if (result.value.hidden > 0 || truncated.value) return `Top ${matches} matches`
    return matches === 1 ? '1 match' : `${matches} matches`
  })

  return { ...field, result, truncated, cursor, status, refresh, pick }
}
