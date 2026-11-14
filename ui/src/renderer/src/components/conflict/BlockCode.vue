<!-- SPDX-License-Identifier: MPL-2.0 -->
<!-- Copyright (C) 2026 The Lemongrass Authors -->

<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { Compartment, EditorState, type Extension, type Range } from '@codemirror/state'
import { Decoration, EditorView, lineNumbers } from '@codemirror/view'
import { loadLanguage } from '../../languageFor'
import { readerLook } from '../../readerHighlight'
import { commentGutter } from '../../conflictCommentGutter'
import type { ProjectRef } from '../../workspace'
import type { ConflictCommentPane } from '../../../../preload/types'

// A few lines of code, read only and colored by the grammar of the conflicted file's own path, with
// the lines that belong to the block tinted by what became of it. Plain context lines keep only
// the syntax colors.
const props = defineProps<{
  // The file's path, whose extension picks the grammar; none for plain text.
  path: string
  lines: string[]
  // The lines (0-based) the block itself covers, among the clean lines around it.
  from: number
  to: number
  mark: 'taken' | 'rejected' | 'open' | 'edited'
  // The number of the first line, in the file the lines were read from; none shows no numbers.
  start: number | null
  // With these, the box takes comments on its lines, scoped to the block.
  comment?: { project: ProjectRef; pane: ConflictCommentPane; block: number }
}>()

const host = ref<HTMLDivElement>()
const language = new Compartment()
let view: EditorView | undefined

// Tighter than the reader's: no gutter, small type, and the lines carry the block's tint.
const look = EditorView.theme({
  '&': { fontSize: 'var(--text-xs)' },
  '.cm-scroller': { lineHeight: '1.6' },
  '.cm-content': { padding: 'var(--space-1) 0' },
  '.cm-line': { padding: '0 var(--space-2)' }
})

// The lines of a box are lines of a file from `start`, so a comment's lines are the file's own;
// without line numbers a box cannot say which lines they are, and takes no comments.
function commentExtension(): Extension[] {
  const target = props.comment
  const start = props.start
  if (!target || start === null) return []
  const count = props.lines.length
  return [
    commentGutter({
      project: target.project,
      path: props.path,
      pane: target.pane,
      toFile: (docLine) => start + docLine - 1,
      toDoc: (fileLine) =>
        fileLine >= start && fileLine < start + count ? fileLine - start + 1 : null,
      blockOf: () => target.block,
      shows: (block) => block === target.block
    })
  ]
}

function stateOf(): EditorState {
  const lines = props.lines.map((line) => line.replace(/\r$/, ''))
  const mark = Decoration.line({ class: `block-line ${props.mark}` })
  const ranges: Range<Decoration>[] = []
  let at = 0
  lines.forEach((line, index) => {
    if (index >= props.from && index < props.to) ranges.push(mark.range(at))
    at += line.length + 1
  })
  const marks: Extension = EditorView.decorations.of(Decoration.set(ranges))
  return EditorState.create({
    doc: lines.join('\n'),
    extensions: [
      EditorState.readOnly.of(true),
      EditorView.editable.of(false),
      EditorState.tabSize.of(4),
      readerLook,
      look,
      ...commentExtension(),
      ...(props.start === null
        ? []
        : [lineNumbers({ formatNumber: (n: number) => String(n + props.start! - 1) })]),
      language.of([]),
      marks
    ]
  })
}

// A grammar arrives after the text is shown. An answer for a path the box has since left is
// dropped, so a slow import never colors another file.
async function applyLanguage(path: string): Promise<void> {
  const extension = await loadLanguage(path).catch(() => null)
  if (!view || path !== props.path) return
  view.dispatch({ effects: language.reconfigure(extension ?? []) })
}

onMounted(() => {
  view = new EditorView({ parent: host.value!, state: stateOf() })
  void applyLanguage(props.path)
})

watch(
  () => [
    props.lines,
    props.from,
    props.to,
    props.mark,
    props.path,
    props.start,
    props.comment?.block
  ],
  () => {
    view?.setState(stateOf())
    void applyLanguage(props.path)
  }
)

onBeforeUnmount(() => view?.destroy())
</script>

<template>
  <div ref="host" class="block-code" />
</template>

<style scoped>
.block-code :deep(.cm-editor) {
  height: auto;
}

.block-code :deep(.cm-scroller) {
  overflow-x: auto;
}

/* Muted text reads as secondary on a tint, keeping it above its contrast floor. */
.block-code :deep(.block-line) {
  --color-fg-muted: var(--color-fg-secondary);
}

.block-code :deep(.block-line.taken) {
  background: var(--color-jade-muted);
}

.block-code :deep(.block-line.rejected) {
  background: var(--color-error-muted);
}

.block-code :deep(.block-line.open) {
  background: var(--color-sapphire-muted);
}

.block-code :deep(.block-line.edited) {
  background: var(--color-violet-muted);
}
</style>
