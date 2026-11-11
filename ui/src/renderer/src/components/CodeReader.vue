<!-- SPDX-License-Identifier: MPL-2.0 -->
<!-- Copyright (C) 2026 The Lemongrass Authors -->

<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { Compartment, EditorState, StateEffect, StateField } from '@codemirror/state'
import { Decoration, EditorView, lineNumbers, type DecorationSet } from '@codemirror/view'
import { loadLanguage } from '../languageFor'
import { readerLook } from '../readerHighlight'
import type { LineReveal } from '../lineReveal'

// Text shown read-only in CodeMirror with syntax colors. CodeMirror draws only the lines in view,
// so a file near the 2 MB read limit costs no more to show than a short one.
const props = defineProps<{
  content: string
  path: string
  // Scrolls to this line and marks it until the next click.
  reveal?: LineReveal
}>()

const emit = defineEmits<{
  revealed: [seq: number]
}>()

const setMark = StateEffect.define<number | null>()
const markLine = Decoration.line({ class: 'cm-reveal-line' })
const markField = StateField.define<DecorationSet>({
  create: () => Decoration.none,
  update(marks, tr) {
    let next = marks.map(tr.changes)
    for (const effect of tr.effects) {
      if (effect.is(setMark)) {
        next =
          effect.value === null ? Decoration.none : Decoration.set([markLine.range(effect.value)])
      }
    }
    return next
  },
  provide: (field) => EditorView.decorations.from(field)
})

const host = ref<HTMLDivElement>()
const language = new Compartment()
let view: EditorView | undefined

// A grammar arrives after the text is already shown. An answer for a path the reader has since
// left is dropped, so a slow import never colors another file.
async function applyLanguage(path: string): Promise<void> {
  const extension = await loadLanguage(path).catch(() => null)
  if (!view || path !== props.path) return
  view.dispatch({ effects: language.reconfigure(extension ?? []) })
}

onMounted(() => {
  view = new EditorView({
    parent: host.value!,
    state: EditorState.create({
      doc: props.content,
      extensions: [
        EditorState.readOnly.of(true),
        EditorView.editable.of(false),
        EditorState.tabSize.of(4),
        lineNumbers(),
        readerLook,
        language.of([]),
        markField,
        EditorView.domEventHandlers({
          mousedown: (_event, target) => {
            target.dispatch({ effects: setMark.of(null) })
            return false
          }
        })
      ]
    })
  })
  void applyLanguage(props.path)
  if (props.reveal) showReveal(props.reveal)
})

function showReveal(reveal: LineReveal): void {
  if (!view) return
  const doc = view.state.doc
  const line = doc.line(Math.min(Math.max(reveal.line, 1), doc.lines))
  view.dispatch({
    effects: [setMark.of(line.from), EditorView.scrollIntoView(line.from, { y: 'center' })]
  })
  emit('revealed', reveal.seq)
}

watch(
  () => props.reveal?.seq,
  () => props.reveal && showReveal(props.reveal)
)

// The file is reread every time its tab is shown, so the same text comes back often; replacing
// the document only on a real change keeps the scroll position.
watch(
  () => props.content,
  (content) => {
    if (!view || view.state.doc.toString() === content) return
    const top = view.scrollDOM.scrollTop
    view.dispatch({ changes: { from: 0, to: view.state.doc.length, insert: content } })
    view.requestMeasure({ read: () => {}, write: () => view && (view.scrollDOM.scrollTop = top) })
  }
)

watch(
  () => props.path,
  (path) => {
    view?.dispatch({ effects: language.reconfigure([]) })
    void applyLanguage(path)
  }
)

onBeforeUnmount(() => {
  view?.destroy()
  view = undefined
})
</script>

<template>
  <div ref="host" class="code-reader" />
</template>

<style scoped>
.code-reader {
  flex: 1;
  min-height: 0;
  display: flex;
}

.code-reader :deep(.cm-editor) {
  height: 100%;
}

.code-reader :deep(.cm-reveal-line) {
  background: var(--color-amber-muted);
}
</style>
