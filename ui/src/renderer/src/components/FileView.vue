<!-- SPDX-License-Identifier: AGPL-3.0-only -->
<!-- Copyright (C) 2026 The Lemongrass Authors -->

<script lang="ts">
import { reactive } from 'vue'

type MarkdownMode = 'formatted' | 'source'

// Kept outside the component because the body unmounts while its tab is in the background.
const markdownModes = reactive(new Map<string, MarkdownMode>())
</script>

<script setup lang="ts">
import { computed, onMounted } from 'vue'
import { AppEmptyState, AppSegmented } from '@sankasten'
import MarkdownReader from './MarkdownReader.vue'
import ReadOnlyFile from './ReadOnlyFile.vue'
import {
  isMarkdownPath,
  loadProjectFile,
  openProjectFile,
  workspaceOf,
  type ProjectRef
} from '../workspace'
import { projectImageUrl, renderReaderMarkdown } from '../readerMarkdown'
import type { WorkspaceTab } from '../../../preload/types'

type FileTab = Extract<WorkspaceTab, { kind: 'file' }>

const props = defineProps<{
  project: ProjectRef
  tab: FileTab
}>()

const entry = computed(() => workspaceOf(props.project.id)?.files[props.tab.path])

const imageSrc = computed(() => {
  const query = new URLSearchParams({
    p: props.project.path,
    f: props.tab.path,
    v: String(entry.value?.imageVersion ?? 0)
  })
  return `project-file://project/?${query}`
})

const isMarkdown = computed(() => isMarkdownPath(props.tab.path) && entry.value?.kind === 'text')

const mode = computed<MarkdownMode>({
  get: () => markdownModes.get(props.tab.id) ?? 'formatted',
  set: (value) => markdownModes.set(props.tab.id, value)
})

const modeOptions: { value: MarkdownMode; label: string }[] = [
  { value: 'formatted', label: 'Formatted' },
  { value: 'source', label: 'Source' }
]

const html = computed(() =>
  isMarkdown.value && mode.value === 'formatted'
    ? renderReaderMarkdown(
        entry.value?.content ?? '',
        props.tab.path,
        projectImageUrl(props.project.path)
      )
    : ''
)

function openLinked(path: string): void {
  openProjectFile(props.project, path)
}

// The body unmounts in the background, so every show rereads the file: a file whose folder the
// explorer does not watch is still current when looked at.
onMounted(() => void loadProjectFile(props.project, props.tab.path, true))
</script>

<template>
  <div class="file-view">
    <AppEmptyState
      v-if="!entry || entry.status === 'loading'"
      layout="fill"
      loading
      title="Loading..."
    />
    <AppEmptyState
      v-else-if="entry.status === 'missing'"
      layout="fill"
      title="This file no longer exists."
      detail="Close the tab to remove it."
    />
    <template v-else-if="isMarkdown">
      <div class="mode-bar">
        <AppSegmented v-model="mode" :options="modeOptions" aria-label="Markdown view" />
      </div>
      <MarkdownReader
        v-if="mode === 'formatted'"
        :html="html"
        :doc-path="tab.path"
        :open="openLinked"
      />
      <ReadOnlyFile v-else :entry="entry" :path="tab.path" :image-src="imageSrc" :alt="tab.path" />
    </template>
    <ReadOnlyFile v-else :entry="entry" :path="tab.path" :image-src="imageSrc" :alt="tab.path" />
  </div>
</template>

<style scoped>
.file-view {
  flex: 1;
  min-height: 0;
  display: flex;
  flex-direction: column;
}

.mode-bar {
  flex-shrink: 0;
  display: flex;
  justify-content: flex-end;
  padding: var(--space-2) var(--space-3);
  border-bottom: 1px solid var(--color-border-subtle);
}
</style>
