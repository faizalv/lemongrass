<!-- SPDX-License-Identifier: AGPL-3.0-only -->
<!-- Copyright (C) 2026 The Lemongrass Authors -->

<script setup lang="ts">
import { nextTick, onBeforeUnmount, onMounted, ref, shallowRef, watch } from 'vue'
import { Editor, EditorContent, Extension, Node, mergeAttributes } from '@tiptap/vue-3'
import StarterKit from '@tiptap/starter-kit'
import Image from '@tiptap/extension-image'
import { Table, TableCell, TableHeader, TableRow } from '@tiptap/extension-table'
import { TaskItem, TaskList } from '@tiptap/extension-list'
import { Markdown } from 'tiptap-markdown'
import { Find, findState, setFindQuery, stepFind } from '../editorFind'
import { AppInput, type AppInputHandle } from '@sankasten'
import { isAbsolutePath, lastSegment } from '../pathForm'
import { isWindows } from '../platform'

const props = defineProps<{
  modelValue: string
  projectPath?: string
  notePath?: string
}>()

const emit = defineEmits<{
  'update:modelValue': [value: string]
  'image-pending': [url: string, file: File]
}>()

const editor = shallowRef<Editor>()
const findOpen = ref(false)
const findQuery = ref('')
const findCount = ref(0)
const findCurrent = ref(0)
const findInput = ref<AppInputHandle>()
let lastMarkdown = ''
const imageNotice = ref('')
let noticeTimer: ReturnType<typeof setTimeout> | undefined

function showImageNotice(message: string): void {
  imageNotice.value = message
  clearTimeout(noticeTimer)
  noticeTimer = setTimeout(() => (imageNotice.value = ''), 4000)
}

function resolveImageSrc(src: string): string {
  if (!props.projectPath || !props.notePath || /^[a-z][a-z0-9+.-]*:/i.test(src)) return src
  const parts = props.notePath.split('/').slice(0, -1)
  for (const segment of src.split('/')) {
    if (segment === '..') parts.pop()
    else if (segment !== '.') parts.push(segment)
  }
  const query = `p=${encodeURIComponent(props.projectPath)}&f=${encodeURIComponent(parts.join('/'))}`
  return `biblio-image://project/?${query}`
}

const ScratchpadImage = Image.extend({
  renderHTML({ HTMLAttributes }) {
    return [
      'img',
      mergeAttributes(this.options.HTMLAttributes, {
        ...HTMLAttributes,
        src: resolveImageSrc(String(HTMLAttributes.src ?? ''))
      })
    ]
  }
})

const LiteralText = Node.create({
  name: 'text',
  group: 'inline',
  addStorage() {
    return {
      markdown: {
        // A bare pipe inside a table cell would end the cell, so only there it is escaped.
        serialize(
          state: { text(text: string, escape: boolean): void; inTable?: boolean },
          node: { text?: string }
        ) {
          const text = node.text ?? ''
          state.text(state.inTable ? text.replace(/\|/g, '\\|') : text, false)
        }
      }
    }
  }
})

// tiptap-markdown can write a table as markdown only while every cell is one paragraph; otherwise,
// with html off, the whole table is saved as the text "[table]". So a cell holds one paragraph
// and the keys that would add a paragraph or a hard break do nothing inside it.
const SingleParagraphCell = TableCell.extend({ content: 'paragraph' })
const SingleParagraphHeader = TableHeader.extend({ content: 'paragraph' })

const CellLineGuard = Extension.create({
  name: 'cellLineGuard',
  priority: 1000,
  addKeyboardShortcuts() {
    const inCell = (): boolean =>
      this.editor.isActive('tableCell') || this.editor.isActive('tableHeader')
    return {
      Enter: inCell,
      'Shift-Enter': inCell,
      'Mod-Enter': inCell
    }
  }
})

// tiptap-markdown gives only bullet and ordered lists a `tight` attribute; without one a task list
// serializes loose, putting a blank line between every `- [ ]` item on save.
const TightTaskLists = Extension.create({
  name: 'tightTaskLists',
  addGlobalAttributes() {
    return [
      {
        types: ['taskList'],
        attributes: {
          tight: {
            default: true,
            parseHTML: (element) =>
              element.getAttribute('data-tight') === 'true' || !element.querySelector('p'),
            renderHTML: (attributes) => ({ 'data-tight': attributes.tight ? 'true' : null })
          }
        }
      }
    ]
  }
})

const MAX_IMAGE_BYTES = 10 * 1024 * 1024
const IMAGE_MIMES = ['image/png', 'image/jpeg', 'image/gif', 'image/webp']
const IMAGE_PATH = /\.(png|jpe?g|gif|webp)$/i

const errorMessages = {
  'too-large': 'Image is larger than 10 MB.',
  unsupported: 'Only PNG, JPEG, GIF and WebP images are supported.',
  failed: 'Could not save the image.'
}

async function pathToFile(path: string): Promise<File | null> {
  const result = await window.api.biblio.readImageFile(path)
  if ('error' in result) {
    showImageNotice(errorMessages[result.error])
    return null
  }
  const name = lastSegment(path.startsWith('file:') ? decodeURIComponent(path) : path, isWindows)
  return new File([new Uint8Array(result.bytes)], name, { type: result.mime })
}

async function insertImages(sources: (File | string)[], position: number | null): Promise<void> {
  const { projectPath, notePath } = props
  for (const source of sources) {
    if (!editor.value) return
    const file = typeof source === 'string' ? await pathToFile(source) : source
    if (!file) continue
    if (!IMAGE_MIMES.includes(file.type)) {
      showImageNotice(errorMessages.unsupported)
      continue
    }
    if (file.size > MAX_IMAGE_BYTES) {
      showImageNotice(errorMessages['too-large'])
      continue
    }
    let src: string
    if (projectPath && notePath) {
      const bytes = new Uint8Array(await file.arrayBuffer())
      const result = await window.api.biblio.saveScratchpadImage(
        projectPath,
        notePath,
        bytes,
        file.type
      )
      if ('error' in result) {
        showImageNotice(errorMessages[result.error])
        continue
      }
      src = result.path
    } else {
      src = URL.createObjectURL(file)
      emit('image-pending', src, file)
    }
    const image = { type: 'image', attrs: { src } }
    const chain = editor.value.chain().focus()
    if (position === null) chain.insertContent(image).run()
    else chain.insertContentAt(position, image).run()
  }
}

function imageFiles(list: FileList | null | undefined): File[] {
  return Array.from(list ?? []).filter((file) => file.type.startsWith('image/'))
}

function pastedImagePaths(data: DataTransfer | null): string[] {
  const text = data?.getData('text/uri-list') || data?.getData('text/plain') || ''
  const lines = text
    .split(/\r?\n/)
    .map((line) => line.trim())
    .filter((line) => line && !line.startsWith('#'))
  // A file URL goes to main as is, which turns it into a path the host's own way (file:///C:/x.png on Windows).
  const paths = lines.map((line) => {
    if (line.startsWith('file://')) return line
    return isAbsolutePath(line, isWindows) ? line : ''
  })
  return paths.length && paths.every((path) => IMAGE_PATH.test(path)) ? paths : []
}

// tiptap-markdown's storage isn't declared on Tiptap's own Storage type;
// typed loosely on purpose so it accepts both @tiptap/core's and
// @tiptap/vue-3's (slightly different) Editor types.
function getMarkdown(e: { storage: unknown }): string {
  return (e.storage as unknown as { markdown: { getMarkdown(): string } }).markdown.getMarkdown()
}

function syncFind(): void {
  if (!editor.value) return
  const find = findState(editor.value)
  findCount.value = find.matches.length
  findCurrent.value = find.current
}

async function openFind(): Promise<void> {
  if (!editor.value) return
  const { from, to } = editor.value.state.selection
  const selected = editor.value.state.doc.textBetween(from, to)
  if (selected && !selected.includes('\n')) findQuery.value = selected
  findOpen.value = true
  await nextTick()
  findInput.value?.select()
  setFindQuery(editor.value, findQuery.value)
}

function closeFind(): void {
  findOpen.value = false
  if (!editor.value) return
  setFindQuery(editor.value, '')
  editor.value.commands.focus()
}

function onFindInput(): void {
  if (editor.value) setFindQuery(editor.value, findQuery.value)
}

function onFindKeydown(event: KeyboardEvent): void {
  if (!editor.value) return
  if (event.key === 'Enter') {
    event.preventDefault()
    stepFind(editor.value, event.shiftKey ? -1 : 1)
  } else if (event.key === 'Escape') {
    event.preventDefault()
    closeFind()
  }
}

function onKeydown(event: KeyboardEvent): void {
  if ((event.ctrlKey || event.metaKey) && !event.altKey && event.key.toLowerCase() === 'f') {
    event.preventDefault()
    void openFind()
  }
}

function focusFromBlank(event: MouseEvent): void {
  if ((event.target as HTMLElement).closest('.ProseMirror')) return
  editor.value?.commands.focus('end')
}

onMounted(() => {
  editor.value = new Editor({
    content: props.modelValue,
    extensions: [
      StarterKit.configure({ text: false }),
      LiteralText,
      ScratchpadImage,
      Table.configure({ resizable: false }),
      TableRow,
      SingleParagraphHeader,
      SingleParagraphCell,
      CellLineGuard,
      TaskList,
      TightTaskLists,
      TaskItem.configure({ nested: true }),
      Markdown.configure({ html: false }),
      Find
    ],
    editorProps: {
      handlePaste: (_view, event) => {
        const files = imageFiles(event.clipboardData?.files)
        const sources = files.length ? files : pastedImagePaths(event.clipboardData ?? null)
        if (!sources.length) return false
        event.preventDefault()
        void insertImages(sources, null)
        return true
      },
      handleDrop: (view, event) => {
        const files = imageFiles(event.dataTransfer?.files)
        if (!files.length) return false
        event.preventDefault()
        const position = view.posAtCoords({ left: event.clientX, top: event.clientY })?.pos ?? null
        void insertImages(files, position)
        return true
      }
    },
    onCreate: ({ editor }) => {
      lastMarkdown = getMarkdown(editor)
    },
    onTransaction: syncFind,
    onUpdate: ({ editor }) => {
      const markdown = getMarkdown(editor)
      if (markdown.trimEnd() === lastMarkdown.trimEnd()) return
      lastMarkdown = markdown
      emit('update:modelValue', markdown)
    }
  })
})

onBeforeUnmount(() => {
  clearTimeout(noticeTimer)
  editor.value?.destroy()
})

watch(
  () => props.modelValue,
  (value) => {
    const current = editor.value
    if (!current || value === lastMarkdown) return
    const { from, to } = current.state.selection
    current.commands.setContent(value, { emitUpdate: false })
    current.commands.setTextSelection({ from, to })
    lastMarkdown = getMarkdown(current)
  }
)
</script>

<template>
  <div class="markdown-editor" @keydown="onKeydown">
    <div v-if="editor" class="toolbar">
      <button
        class="tool"
        :class="{ active: editor.isActive('bold') }"
        title="Bold"
        @click="editor.chain().focus().toggleBold().run()"
      >
        B
      </button>
      <button
        class="tool italic"
        :class="{ active: editor.isActive('italic') }"
        title="Italic"
        @click="editor.chain().focus().toggleItalic().run()"
      >
        I
      </button>
      <span class="divider" />
      <button
        class="tool"
        :class="{ active: editor.isActive('heading', { level: 1 }) }"
        title="Heading 1"
        @click="editor.chain().focus().toggleHeading({ level: 1 }).run()"
      >
        H1
      </button>
      <button
        class="tool"
        :class="{ active: editor.isActive('heading', { level: 2 }) }"
        title="Heading 2"
        @click="editor.chain().focus().toggleHeading({ level: 2 }).run()"
      >
        H2
      </button>
      <span class="divider" />
      <button
        class="tool"
        :class="{ active: editor.isActive('bulletList') }"
        title="Bullet list"
        @click="editor.chain().focus().toggleBulletList().run()"
      >
        &bull; List
      </button>
      <button
        class="tool"
        :class="{ active: editor.isActive('orderedList') }"
        title="Numbered list"
        @click="editor.chain().focus().toggleOrderedList().run()"
      >
        1. List
      </button>
      <span class="divider" />
      <button
        class="tool mono"
        :class="{ active: editor.isActive('codeBlock') }"
        title="Code block"
        @click="editor.chain().focus().toggleCodeBlock().run()"
      >
        &lt;/&gt;
      </button>
      <button
        class="tool"
        :class="{ active: editor.isActive('blockquote') }"
        title="Quote"
        @click="editor.chain().focus().toggleBlockquote().run()"
      >
        &ldquo;
      </button>
      <button
        class="tool"
        title="Task list"
        :class="{ active: editor.isActive('taskList') }"
        @click="editor.chain().focus().toggleTaskList().run()"
      >
        &check; Tasks
      </button>
      <button
        class="tool"
        title="Insert table"
        :disabled="editor.isActive('table')"
        @click="editor.chain().focus().insertTable({ rows: 3, cols: 3, withHeaderRow: true }).run()"
      >
        Table
      </button>
      <template v-if="editor.isActive('table')">
        <span class="divider" />
        <button
          class="tool"
          title="Add row below"
          @click="editor.chain().focus().addRowAfter().run()"
        >
          + Row
        </button>
        <button
          class="tool"
          title="Add column right"
          @click="editor.chain().focus().addColumnAfter().run()"
        >
          + Col
        </button>
        <button class="tool" title="Delete row" @click="editor.chain().focus().deleteRow().run()">
          &minus; Row
        </button>
        <button
          class="tool"
          title="Delete column"
          @click="editor.chain().focus().deleteColumn().run()"
        >
          &minus; Col
        </button>
        <button
          class="tool danger"
          title="Delete table"
          @click="editor.chain().focus().deleteTable().run()"
        >
          Delete table
        </button>
      </template>
    </div>
    <div v-if="findOpen" class="find">
      <AppInput
        ref="findInput"
        v-model="findQuery"
        size="sm"
        class="find-input"
        placeholder="Find"
        spellcheck="false"
        @input="onFindInput"
        @keydown="onFindKeydown"
      />
      <span class="find-count">
        {{ findCount ? `${findCurrent + 1} of ${findCount}` : findQuery ? 'No matches' : '' }}
      </span>
      <button
        class="tool"
        title="Previous (Shift+Enter)"
        :disabled="!findCount"
        @click="editor && stepFind(editor, -1)"
      >
        &uarr;
      </button>
      <button
        class="tool"
        title="Next (Enter)"
        :disabled="!findCount"
        @click="editor && stepFind(editor, 1)"
      >
        &darr;
      </button>
      <button class="tool" title="Close (Esc)" @click="closeFind">&times;</button>
    </div>
    <p v-if="imageNotice" class="notice">{{ imageNotice }}</p>
    <EditorContent v-if="editor" class="content" :editor="editor" @click="focusFromBlank" />
  </div>
</template>

<style scoped>
.markdown-editor {
  width: 100%;
  max-width: 720px;
  min-height: 0;
  display: flex;
  flex-direction: column;
  border: 1px solid var(--color-border-default);
  border-radius: var(--radius-lg);
  overflow: hidden;
  background: var(--color-surface-2);
  transition: border-color var(--duration-fast) var(--ease-out);
}

.markdown-editor:focus-within {
  border-color: var(--color-amber);
}

.toolbar {
  flex-shrink: 0;
  display: flex;
  align-items: center;
  gap: 2px;
  flex-wrap: wrap;
  padding: var(--space-2);
  background: var(--color-surface-1);
  border-bottom: 1px solid var(--color-border-default);
}

.tool {
  padding: var(--space-1) var(--space-2);
  background: transparent;
  border: none;
  border-radius: var(--radius-sm);
  color: var(--color-fg-secondary);
  font-family: var(--font-body);
  font-size: var(--text-xs);
  font-weight: var(--weight-medium);
  cursor: pointer;
  transition:
    background var(--duration-fast) var(--ease-out),
    color var(--duration-fast) var(--ease-out);
}

.tool.italic {
  font-style: italic;
}

.tool.mono {
  font-family: var(--font-mono);
}

.tool:hover {
  background: var(--color-surface-2);
  color: var(--color-fg-primary);
}

.tool.active {
  background: var(--color-amber-muted);
  color: var(--color-fg-accent);
}

.divider {
  width: 1px;
  height: 16px;
  background: var(--color-border-default);
  margin: 0 var(--space-1);
}

.notice {
  flex-shrink: 0;
  padding: var(--space-2) var(--space-4);
  background: var(--color-surface-1);
  border-bottom: 1px solid var(--color-border-default);
  color: var(--color-fg-secondary);
  font-size: var(--text-xs);
}

.find {
  flex-shrink: 0;
  display: flex;
  align-items: center;
  gap: var(--space-2);
  padding: var(--space-2);
  background: var(--color-surface-1);
  border-bottom: 1px solid var(--color-border-default);
}

.find-input {
  flex: 1;
  min-width: 0;
}

.find-count {
  flex-shrink: 0;
  color: var(--color-fg-secondary);
  font-size: var(--text-xs);
  font-variant-numeric: tabular-nums;
}

.tool:disabled {
  opacity: 0.4;
  cursor: default;
}

.content :deep(.find-match) {
  background: var(--color-amber-muted);
  border-radius: 2px;
}

.content :deep(.find-match.current) {
  background: var(--color-amber);
  color: var(--color-surface-1);
}

.content {
  flex: 1;
  min-height: 0;
  display: flex;
  flex-direction: column;
  overflow-y: auto;
  padding: var(--space-4);
  color: var(--color-fg-primary);
  font-family: var(--font-body);
  font-size: var(--text-sm);
  line-height: var(--leading-relaxed);
}

.content :deep(.ProseMirror) {
  outline: none;
  flex: 1 0 auto;
  min-height: 200px;
}

.content :deep(h1),
.content :deep(h2),
.content :deep(h3) {
  font-family: var(--font-display);
  font-weight: var(--weight-bold);
  line-height: var(--leading-tight);
  margin-top: var(--space-6);
  margin-bottom: var(--space-3);
}

.content :deep(h1:first-child),
.content :deep(h2:first-child),
.content :deep(h3:first-child) {
  margin-top: 0;
}

.content :deep(h1) {
  font-size: var(--text-xl);
}
.content :deep(h2) {
  font-size: var(--text-lg);
}
.content :deep(h3) {
  font-size: var(--text-md);
}

.content :deep(p) {
  margin-bottom: var(--space-3);
}

.content :deep(ul),
.content :deep(ol) {
  margin-bottom: var(--space-3);
  padding-left: var(--space-6);
}

.content :deep(code) {
  font-family: var(--font-mono);
  font-size: 0.9em;
  background: var(--color-surface-2);
  padding: 0.15em 0.4em;
  border-radius: var(--radius-sm);
}

.content :deep(img) {
  display: block;
  max-width: 100%;
  height: auto;
  margin-bottom: var(--space-3);
  border-radius: var(--radius-md);
}

.content :deep(pre) {
  background: var(--color-surface-2);
  border-radius: var(--radius-lg);
  padding: var(--space-4);
  overflow-x: auto;
  margin-bottom: var(--space-3);
}

.content :deep(pre code) {
  background: none;
  padding: 0;
}

.content :deep(blockquote) {
  border-left: 2px solid var(--color-border-default);
  padding-left: var(--space-4);
  color: var(--color-fg-secondary);
  margin-bottom: var(--space-3);
}

.tool.danger:hover {
  color: var(--color-error);
}

.content :deep(h4),
.content :deep(h5),
.content :deep(h6) {
  font-family: var(--font-display);
  font-weight: var(--weight-bold);
  line-height: var(--leading-tight);
  margin-top: var(--space-5);
  margin-bottom: var(--space-2);
}

.content :deep(h4) {
  font-size: var(--text-base);
}

.content :deep(h5),
.content :deep(h6) {
  font-size: var(--text-sm);
}

.content :deep(h6) {
  color: var(--color-fg-secondary);
}

.content :deep(li > ul),
.content :deep(li > ol) {
  margin-bottom: 0;
}

.content :deep(.tableWrapper) {
  overflow-x: auto;
  margin-bottom: var(--space-3);
}

.content :deep(table) {
  border-collapse: collapse;
  table-layout: auto;
}

.content :deep(th),
.content :deep(td) {
  min-width: 60px;
  padding: var(--space-1) var(--space-3);
  border: 1px solid var(--color-border-default);
  text-align: left;
  vertical-align: top;
}

.content :deep(th) {
  background: var(--layer-raised-hover);
  font-weight: var(--weight-semibold);
}

.content :deep(th p),
.content :deep(td p) {
  margin-bottom: 0;
}

.content :deep(.selectedCell) {
  background: var(--color-amber-muted);
}

.content :deep(ul[data-type='taskList']) {
  list-style: none;
  padding-left: var(--space-1);
}

/* TaskItem's node view draws its li without data-type="taskItem", so rules reach it through the list. */
.content :deep(ul[data-type='taskList'] > li) {
  display: flex;
  align-items: flex-start;
  gap: var(--space-2);
}

.content :deep(ul[data-type='taskList'] > li > div) {
  flex: 1;
  min-width: 0;
}

.content :deep(ul[data-type='taskList'] > li > label) {
  flex-shrink: 0;
  display: flex;
  padding-top: 4px;
  cursor: pointer;
}

.content :deep(ul[data-type='taskList'] > li input[type='checkbox']) {
  appearance: none;
  width: 14px;
  height: 14px;
  margin: 0;
  background: var(--layer-input);
  border: 1px solid var(--color-border-strong);
  border-radius: var(--radius-sm);
  cursor: pointer;
  transition:
    background var(--duration-fast) var(--ease-default),
    border-color var(--duration-fast) var(--ease-default);
}

.content :deep(ul[data-type='taskList'] > li input[type='checkbox']:checked) {
  background: var(--color-amber)
    url("data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' viewBox='0 0 24 24' fill='none' stroke='%230A0A0A' stroke-width='3.5' stroke-linecap='round' stroke-linejoin='round'%3E%3Cpolyline points='20 6 9 17 4 12'/%3E%3C/svg%3E")
    center / 10px no-repeat;
  border-color: var(--color-amber);
}

.content :deep(ul[data-type='taskList'] > li input[type='checkbox']:focus-visible) {
  outline: 1px solid var(--color-amber);
  outline-offset: 2px;
}

.content :deep(ul[data-type='taskList'] > li[data-checked='true'] > div) {
  color: var(--color-fg-secondary);
}
</style>
