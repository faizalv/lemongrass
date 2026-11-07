// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

import { computed, nextTick, ref, type Ref } from 'vue'
import { type AppInputHandle } from '@sankasten'

export interface SearchFieldOptions {
  // Where focus returns when the field closes, the panel itself.
  panel: Ref<HTMLElement | null>
  // Arrow and Enter keys typed in the field; the view owns what they move over.
  move: (delta: number) => void
  enter: () => void
  // Runs after the field closes and its query is cleared.
  onClose?: () => void
}

export interface SearchField {
  open: Ref<boolean>
  query: Ref<string>
  input: Ref<AppInputHandle | null>
  // The field is open with a non-blank query.
  active: Ref<boolean>
  onPanelKeydown: (event: KeyboardEvent) => void
  onInputKeydown: (event: KeyboardEvent) => void
  onInputBlur: (event?: FocusEvent) => void
  show: () => void
  close: () => void
  setQuery: (text: string) => void
  bindInput: (handle: unknown) => void
}

function typedIntoField(event: KeyboardEvent): boolean {
  const target = event.target as HTMLElement | null
  return !!target?.closest('input, textarea, [contenteditable="true"]')
}

// The sidebar's type-to-search field, shared by the Biblio and Files views: a printable key while
// the panel has focus opens the field with that key in it, arrows and Enter go to the view, Escape
// clears and closes.
export function useSearchField(options: SearchFieldOptions): SearchField {
  const open = ref(false)
  const query = ref('')
  const input = ref<AppInputHandle | null>(null)
  const active = computed(() => open.value && query.value.trim() !== '')

  function close(): void {
    open.value = false
    query.value = ''
    options.onClose?.()
    options.panel.value?.focus()
  }

  function onPanelKeydown(event: KeyboardEvent): void {
    if (event.defaultPrevented || typedIntoField(event)) return
    if (event.key.length !== 1 || event.ctrlKey || event.metaKey || event.altKey) return
    if (event.key === ' ' && !open.value) return
    event.preventDefault()
    open.value = true
    query.value += event.key
    void nextTick(() => input.value?.focus())
  }

  // The explicit way in, for a search button: opens the field as typing does, without a first key.
  function show(): void {
    open.value = true
    void nextTick(() => input.value?.focus())
  }

  function onInputKeydown(event: KeyboardEvent): void {
    if (event.key === 'ArrowDown') options.move(1)
    else if (event.key === 'ArrowUp') options.move(-1)
    else if (event.key === 'Enter') options.enter()
    else if (event.key === 'Escape') close()
    else return
    event.preventDefault()
    event.stopPropagation()
  }

  // An empty field closes when focus leaves it, but not for a control in the search bar itself,
  // such as the Biblio scope picker, which would otherwise unmount under the click.
  function onInputBlur(event?: FocusEvent): void {
    const next = event?.relatedTarget as HTMLElement | null | undefined
    if (next?.closest('.tree-search-bar')) return
    if (!query.value) open.value = false
  }

  function setQuery(text: string): void {
    query.value = text
  }

  function bindInput(handle: unknown): void {
    input.value = (handle as AppInputHandle | null) ?? null
  }

  return {
    open,
    query,
    input,
    active,
    onPanelKeydown,
    onInputKeydown,
    onInputBlur,
    show,
    close,
    setQuery,
    bindInput
  }
}
