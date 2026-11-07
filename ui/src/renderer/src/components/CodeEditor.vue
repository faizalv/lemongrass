<!-- SPDX-License-Identifier: MPL-2.0 -->
<!-- Copyright (C) 2026 The Lemongrass Authors -->

<script lang="ts">
import { defaultKeymap, history, historyKeymap, indentWithTab } from '@codemirror/commands'
import { Compartment, EditorState } from '@codemirror/state'
import { drawSelection, EditorView, keymap, lineNumbers } from '@codemirror/view'
import { readerLook } from '../readerHighlight'

// A kept state outlives the component that made it, so its extensions are module-level: the
// language compartment is shared, and the keymap and listener reach whichever mounted editor
// now shows the state through its view.
interface Handlers {
  change: (state: EditorState) => void
  save: () => void
}

const handlers = new WeakMap<EditorView, Handlers>()
const language = new Compartment()

// readerLook is made for reading; an editor also needs a caret and a selection that show on the
// dark surface, since CodeMirror's base theme paints a black caret. Amber as in the conflict
// editor, since it marks focus.
const editLook = EditorView.theme({
  '.cm-content': { caretColor: 'var(--color-amber)' },
  '.cm-cursor, .cm-dropCursor': { borderLeftColor: 'var(--color-amber)' },
  '.cm-selectionBackground, &.cm-focused .cm-selectionBackground': {
    backgroundColor: 'var(--color-amber-muted) !important'
  }
})

function createState(doc: string): EditorState {
  return EditorState.create({
    doc,
    extensions: [
      history(),
      keymap.of([
        {
          key: 'Mod-s',
          preventDefault: true,
          run: (view) => {
            handlers.get(view)?.save()
            return true
          }
        },
        indentWithTab,
        ...defaultKeymap,
        ...historyKeymap
      ]),
      EditorState.tabSize.of(4),
      lineNumbers(),
      drawSelection(),
      readerLook,
      editLook,
      language.of([]),
      EditorView.updateListener.of((update) => {
        if (update.docChanged) handlers.get(update.view)?.change(update.state)
      })
    ]
  })
}
</script>

<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { loadLanguage } from '../languageFor'

// Editable text in CodeMirror with the reader's look and grammars (CodeReader.vue), plus undo
// and Ctrl+S. It knows nothing about where the text lives: the caller saves on `save` and may
// pass back the last emitted state through `restore`, so a remount keeps unsaved edits and undo
// history.
const props = defineProps<{
  content: string
  path: string
  // Raised by the caller to load content again, discarding edits and undo history.
  revision: number
  restore?: EditorState
  // Takes the keyboard when it mounts, for an editor that has just been put in front.
  autofocus?: boolean
}>()

const emit = defineEmits<{
  change: [state: EditorState]
  save: []
}>()

const host = ref<HTMLDivElement>()
let view: EditorView | undefined

async function applyLanguage(path: string): Promise<void> {
  const extension = await loadLanguage(path).catch(() => null)
  if (!view || path !== props.path) return
  view.dispatch({ effects: language.reconfigure(extension ?? []) })
}

onMounted(() => {
  view = new EditorView({ parent: host.value!, state: props.restore ?? createState(props.content) })
  handlers.set(view, { change: (state) => emit('change', state), save: () => emit('save') })
  if (!props.restore) emit('change', view.state)
  void applyLanguage(props.path)
  if (props.autofocus) view.focus()
})

watch(
  () => props.revision,
  () => {
    if (!view) return
    view.setState(createState(props.content))
    emit('change', view.state)
    void applyLanguage(props.path)
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

defineExpose({ focus: () => view?.focus() })
</script>

<template>
  <div ref="host" class="code-editor" />
</template>

<style scoped>
.code-editor {
  flex: 1;
  min-height: 0;
  display: flex;
}

.code-editor :deep(.cm-editor) {
  height: 100%;
}
</style>
