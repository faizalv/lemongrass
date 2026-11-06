<!-- SPDX-License-Identifier: AGPL-3.0-only -->
<!-- Copyright (C) 2026 The Lemongrass Authors -->

<script setup lang="ts">
import { computed } from 'vue'
import MarkdownEditor from './MarkdownEditor.vue'
import TocViewer from './TocViewer.vue'
import ReadOnlyFile from './ReadOnlyFile.vue'
import MarkdownReader from './MarkdownReader.vue'
import { editDoc, isMarkdownPath, openFile, workspaceOf, type ProjectRef } from '../workspace'
import type { WorkspaceTab } from '../../../preload/types'
import AppEmptyState from './AppEmptyState.vue'

type DocTab = Extract<WorkspaceTab, { kind: 'doc' }>

const props = defineProps<{
  project: ProjectRef
  tab: DocTab
}>()

const doc = computed(() => workspaceOf(props.project.id)?.docs[props.tab.path])

const imageSrc = computed(() => {
  const query = new URLSearchParams({
    p: props.project.path,
    f: props.tab.path,
    v: String(doc.value?.imageVersion ?? 0)
  })
  return `biblio-image://project/?${query}`
})

function edit(value: string): void {
  editDoc(props.project, props.tab.path, value)
}

// Only markdown and scratchpad files have a reader; any other biblio path would show as missing.
function openLinked(path: string): void {
  if (isMarkdownPath(path) || path.startsWith('scratchpad/')) openFile(props.project, path)
}
</script>

<template>
  <div class="doc-view">
    <AppEmptyState
      v-if="!doc || doc.status === 'loading'"
      layout="fill"
      loading
      title="Loading..."
    />
    <AppEmptyState
      v-else-if="doc.status === 'missing'"
      layout="fill"
      title="This file no longer exists."
      detail="Close the tab to remove it."
    />
    <div v-else-if="doc.editable" class="editor-wrap">
      <MarkdownEditor
        :key="tab.id"
        :model-value="doc.content"
        :project-path="project.path"
        :note-path="tab.path"
        @update:model-value="edit"
      />
    </div>
    <TocViewer v-else-if="tab.path === 'books/toc.md'" :project="project" />
    <ReadOnlyFile
      v-else-if="doc.kind !== 'markdown'"
      :entry="doc"
      :path="tab.path"
      :image-src="imageSrc"
      :alt="tab.path"
    />
    <MarkdownReader v-else :html="doc.html" :doc-path="tab.path" :open="openLinked" />
  </div>
</template>

<style scoped>
.doc-view {
  flex: 1;
  min-height: 0;
  display: flex;
  flex-direction: column;
}

.editor-wrap {
  flex: 1;
  min-height: 0;
  display: flex;
  flex-direction: column;
  padding: var(--space-3) var(--space-4) var(--space-4);
}

.editor-wrap :deep(.markdown-editor) {
  flex: 1;
  max-width: none;
}
</style>
