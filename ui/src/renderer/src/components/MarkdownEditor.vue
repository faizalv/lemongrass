<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref, shallowRef, watch } from 'vue'
import { Editor, EditorContent, mergeAttributes } from '@tiptap/vue-3'
import StarterKit from '@tiptap/starter-kit'
import Image from '@tiptap/extension-image'
import { Markdown } from 'tiptap-markdown'

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
  const name = path.split('/').pop() ?? 'image'
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
  const paths = lines.map((line) => {
    if (line.startsWith('file://')) return decodeURIComponent(new URL(line).pathname)
    return line.startsWith('/') ? line : ''
  })
  return paths.length && paths.every((path) => IMAGE_PATH.test(path)) ? paths : []
}

// tiptap-markdown's storage isn't declared on Tiptap's own Storage type;
// typed loosely on purpose so it accepts both @tiptap/core's and
// @tiptap/vue-3's (slightly different) Editor types.
function getMarkdown(e: { storage: unknown }): string {
  return (e.storage as unknown as { markdown: { getMarkdown(): string } }).markdown.getMarkdown()
}

onMounted(() => {
  editor.value = new Editor({
    content: props.modelValue,
    extensions: [StarterKit, ScratchpadImage, Markdown],
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
    onUpdate: ({ editor }) => {
      emit('update:modelValue', getMarkdown(editor))
    }
  })
})

onBeforeUnmount(() => {
  clearTimeout(noticeTimer)
  editor.value?.destroy()
})

// Only fires on a real external change (switching files) -- not on every
// keystroke, since onUpdate above already reflects those back out.
watch(
  () => props.modelValue,
  (value) => {
    if (editor.value && value !== getMarkdown(editor.value)) {
      editor.value.commands.setContent(value, { emitUpdate: false })
    }
  }
)
</script>

<template>
  <div class="markdown-editor">
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
    </div>
    <p v-if="imageNotice" class="notice">{{ imageNotice }}</p>
    <EditorContent v-if="editor" class="content" :editor="editor" />
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

.content {
  flex: 1;
  min-height: 0;
  overflow-y: auto;
  padding: var(--space-4);
  color: var(--color-fg-primary);
  font-family: var(--font-body);
  font-size: var(--text-sm);
  line-height: var(--leading-relaxed);
}

.content :deep(.ProseMirror) {
  outline: none;
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
</style>
