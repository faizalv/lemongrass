// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

import { computed, nextTick, ref, shallowRef, watch, type Ref } from 'vue'
import {
  ancestorsOf,
  searchBiblio,
  type BiblioScope,
  type BiblioSearchNode,
  type BiblioSearchResult
} from './biblioSearch'
import { useSearchField, type SearchField } from './useSearchField'
import type { BiblioNode } from '../../preload/types'

// Module level so the choice lasts the session, across project switches and the Files view.
const scope = ref<BiblioScope>('all')
const includeArchive = ref(false)

const DEBOUNCE_MS = 80

export interface BiblioSearchOptions {
  panel: Ref<HTMLElement | null>
  nodes: () => BiblioNode[]
  // The user's own expanded folders, which the search never writes to.
  expanded: () => Record<string, true>
  openFile: (path: string) => void
  // Toggles a folder in the user's own set, for when no search is active.
  toggleOwn: (path: string) => void
}

export interface BiblioSearch extends SearchField {
  scope: Ref<BiblioScope>
  includeArchive: Ref<boolean>
  result: Ref<BiblioSearchResult<BiblioNode> | null>
  cursorPath: Ref<string | null>
  status: Ref<string>
  // The folders the tree shows open: the user's, plus the search's while it is active.
  expandedPaths: Ref<Record<string, true>>
  toggle: (path: string) => void
  refresh: () => void
}

// The Biblio view's search shows the tree cut down to the rows whose name contains the query,
// closest first, highlights them and opens the folders above them. The opening is its own layer
// over the user's expanded folders, so leaving the search restores the tree as the user had it.
export function useBiblioSearch(options: BiblioSearchOptions): BiblioSearch {
  const result = shallowRef<BiblioSearchResult<BiblioNode> | null>(null)
  const cursor = ref(-1)
  // Folders opened or closed while the search is active, on top of the user's set.
  const opened = ref<Record<string, true>>({})
  const closed = ref<Record<string, true>>({})
  let timer: ReturnType<typeof setTimeout> | undefined

  const field = useSearchField({
    panel: options.panel,
    move,
    enter: () => {
      const path = cursorPath.value
      // Enter before any arrow goes to the first match, as Down would.
      if (!path) return move(1)
      if (isDir(path)) toggle(path)
      else options.openFile(path)
    },
    onClose: () => {
      clearTimeout(timer)
      reset(null)
    }
  })

  const dirs = computed((): Set<string> => {
    const out = new Set<string>()
    const walk = (list: BiblioSearchNode[]): void => {
      for (const node of list) {
        if (node.type !== 'dir') continue
        out.add(node.path)
        if (node.children) walk(node.children)
      }
    }
    walk(options.nodes())
    return out
  })

  function isDir(path: string): boolean {
    return dirs.value.has(path)
  }

  function reset(next: BiblioSearchResult<BiblioNode> | null): void {
    result.value = next
    opened.value = next ? { ...next.expand } : {}
    closed.value = {}
    cursor.value = -1
  }

  function run(): void {
    if (!field.active.value) {
      reset(null)
      return
    }
    reset(searchBiblio(options.nodes(), field.query.value, scope.value, includeArchive.value))
  }

  watch(field.query, () => {
    clearTimeout(timer)
    timer = setTimeout(run, DEBOUNCE_MS)
  })
  watch([scope, includeArchive], run)

  const cursorPath = computed((): string | null => {
    const order = result.value?.order ?? []
    return cursor.value >= 0 && cursor.value < order.length ? order[cursor.value] : null
  })

  // Moving onto a match opens the folders above it, which matters past the expand limit.
  function move(delta: number): void {
    const order = result.value?.order ?? []
    if (!order.length) return
    if (cursor.value < 0) cursor.value = delta > 0 ? 0 : order.length - 1
    else cursor.value = (cursor.value + delta + order.length) % order.length
    const nextOpened = { ...opened.value }
    const nextClosed = { ...closed.value }
    for (const dir of ancestorsOf(order[cursor.value])) {
      nextOpened[dir] = true
      delete nextClosed[dir]
    }
    opened.value = nextOpened
    closed.value = nextClosed
    void nextTick(() =>
      options.panel.value?.querySelector('.search-cursor')?.scrollIntoView({ block: 'nearest' })
    )
  }

  const expandedPaths = computed((): Record<string, true> => {
    const own = options.expanded()
    if (!result.value) return own
    const out: Record<string, true> = { ...own, ...opened.value }
    for (const path of Object.keys(closed.value)) delete out[path]
    return out
  })

  // While the search is active a toggle changes only its own layer.
  function toggle(path: string): void {
    if (!result.value) {
      options.toggleOwn(path)
      return
    }
    const nextOpened = { ...opened.value }
    const nextClosed = { ...closed.value }
    if (expandedPaths.value[path]) {
      delete nextOpened[path]
      nextClosed[path] = true
    } else {
      delete nextClosed[path]
      nextOpened[path] = true
    }
    opened.value = nextOpened
    closed.value = nextClosed
  }

  const status = computed((): string => {
    const current = result.value
    if (!current) return ''
    const count = current.order.length
    if (!count) return 'No matches'
    const text = count === 1 ? '1 match' : `${count} matches`
    return current.capped ? `${text}, type more to expand` : text
  })

  // The tree refetches when biblio/ changes on disk, which an autosave does often, so an open
  // search follows it without undoing the folders the user opened or closed meanwhile.
  function refresh(): void {
    if (!result.value || !field.active.value) return
    const keep = cursorPath.value
    const next = searchBiblio(options.nodes(), field.query.value, scope.value, includeArchive.value)
    result.value = next
    opened.value = { ...next.expand, ...opened.value }
    cursor.value = keep ? next.order.indexOf(keep) : -1
  }

  return {
    ...field,
    scope,
    includeArchive,
    result,
    cursorPath,
    status,
    expandedPaths,
    toggle,
    refresh
  }
}
