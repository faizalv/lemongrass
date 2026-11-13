<!-- SPDX-License-Identifier: MPL-2.0 -->
<!-- Copyright (C) 2026 The Lemongrass Authors -->

<script setup lang="ts">
import { computed } from 'vue'
import { gitStateOf } from '../../git'
import type { ConflictThreadState } from '../../conflictThread'
import { fileStatusText } from '../../conflictDecisions'
import { useFileDecisions } from '../../conflictFiles'
import type { ProjectRef } from '../../workspace'
import { AppSwitcher } from '@sankasten'

// Switches the file the conflict tab shows, among the files git still reports conflicted. While a
// model holds the conflict each file says what waits for the human in it.
const props = defineProps<{
  project: ProjectRef
  path: string
  thread: ConflictThreadState
}>()

const decisions = useFileDecisions(() => props.project)

const emit = defineEmits<{ select: [path: string] }>()

// Past this many files the list gets a filter box.
const SEARCH_FROM = 8

function nameOf(path: string): string {
  return path.slice(path.lastIndexOf('/') + 1)
}

function blocksText(blocks: number): string {
  return blocks === 1 ? '1 conflict' : `${blocks} conflicts`
}

const items = computed(() =>
  (gitStateOf(props.project.id).status?.files ?? [])
    .filter((file) => file.group === 'conflicted')
    .map((file) => {
      const cards = decisions.value.find((d) => d.path === file.path)?.cards ?? null
      const status = props.thread.status ? fileStatusText(cards) : null
      return {
        id: file.path,
        label: nameOf(file.path),
        title: file.path,
        search: file.path,
        hint: status ?? (file.conflictBlocks ? blocksText(file.conflictBlocks) : undefined)
      }
    })
)
</script>

<template>
  <AppSwitcher
    class="file-picker"
    size="sm"
    :items="items"
    :active-id="path"
    :label="nameOf(path)"
    :title="path"
    placeholder="Pick a file"
    empty-text="No conflicted files left."
    :searchable="items.length > SEARCH_FROM"
    search-placeholder="Filter files"
    @select="emit('select', $event)"
  />
</template>

<style scoped>
.file-picker {
  flex-shrink: 0;
}
</style>
