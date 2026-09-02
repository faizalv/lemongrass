<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { marked } from 'marked'
import DOMPurify from 'dompurify'
import KnowledgeTree from './KnowledgeTree.vue'
import type { TreeNode } from '../../../preload'

const props = defineProps<{
  projectPath: string
}>()

const tree = ref<TreeNode[]>([])
const activeFilePath = ref<string | null>(null)
const fileContent = ref<string>('')
const loading = ref(false)

async function loadTree(): Promise<void> {
  tree.value = await window.api.knowledge.tree(props.projectPath)
  activeFilePath.value = null
  fileContent.value = ''
}

async function selectFile(path: string): Promise<void> {
  loading.value = true
  activeFilePath.value = path
  try {
    fileContent.value = await window.api.knowledge.read(props.projectPath, path)
  } catch (err) {
    fileContent.value = `Couldn't read this file: ${(err as Error).message}`
  } finally {
    loading.value = false
  }
}

const isMarkdown = computed(() => activeFilePath.value?.toLowerCase().endsWith('.md') ?? false)

const renderedHtml = computed(() => {
  if (!isMarkdown.value) return ''
  return DOMPurify.sanitize(marked.parse(fileContent.value, { async: false }) as string)
})

watch(() => props.projectPath, loadTree, { immediate: true })
</script>

<template>
  <div class="knowledge-panel">
    <div class="tree-column">
      <KnowledgeTree
        v-if="tree.length > 0"
        :nodes="tree"
        :active-file-path="activeFilePath"
        @select-file="selectFile"
      />
      <p v-else class="empty">No context/ folder in this project.</p>
    </div>
    <div class="content-column">
      <p v-if="loading" class="empty">Loading...</p>
      <div v-else-if="isMarkdown" class="markdown-body" v-html="renderedHtml"></div>
      <pre v-else-if="activeFilePath" class="raw-body">{{ fileContent }}</pre>
      <p v-else class="empty">Select a file to read it.</p>
    </div>
  </div>
</template>

<style scoped>
.knowledge-panel {
  width: 480px;
  flex-shrink: 0;
  display: flex;
  background: var(--color-surface-1);
  border-left: 1px solid var(--color-border-subtle);
}

.tree-column {
  width: 180px;
  flex-shrink: 0;
  overflow-y: auto;
  padding: var(--space-2);
  border-right: 1px solid var(--color-border-subtle);
}

.content-column {
  flex: 1;
  min-width: 0;
  overflow-y: auto;
  padding: var(--space-4);
}

.empty {
  color: var(--color-fg-muted);
  font-size: var(--text-sm);
  padding: var(--space-2);
}

.raw-body {
  font-family: var(--font-mono);
  font-size: var(--text-xs);
  color: var(--color-fg-secondary);
  white-space: pre-wrap;
  word-break: break-word;
}

.markdown-body {
  font-family: var(--font-body);
  font-size: var(--text-sm);
  line-height: var(--leading-relaxed);
  color: var(--color-fg-primary);
}

.markdown-body :deep(h1),
.markdown-body :deep(h2),
.markdown-body :deep(h3) {
  font-family: var(--font-display);
  color: var(--color-fg-primary);
  margin: var(--space-4) 0 var(--space-2);
}

.markdown-body :deep(p) {
  margin: 0 0 var(--space-3);
}

.markdown-body :deep(code) {
  font-family: var(--font-mono);
  background: var(--color-surface-2);
  padding: 2px 4px;
  border-radius: var(--radius-sm);
  font-size: var(--text-xs);
}

.markdown-body :deep(pre) {
  background: var(--color-surface-2);
  padding: var(--space-3);
  border-radius: var(--radius-md);
  overflow-x: auto;
}

.markdown-body :deep(a) {
  color: var(--color-fg-accent);
}

.markdown-body :deep(ul),
.markdown-body :deep(ol) {
  padding-left: var(--space-5);
  margin: 0 0 var(--space-3);
}
</style>
