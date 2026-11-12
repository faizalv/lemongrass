// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

import {
  Facet,
  StateEffect,
  StateField,
  type EditorState,
  type Extension,
  type Transaction
} from '@codemirror/state'
import { Decoration, EditorView, WidgetType, type DecorationSet } from '@codemirror/view'
import { createApp, type App } from 'vue'
import NoteCard, { type NoteReply } from './components/conflict/NoteCard.vue'

// A model's note on lines of the editor it sits in, already mapped to that editor's lines, with
// the replies to it. A question is a note on a block the model left open for the human.
export interface NoteMark {
  id: number
  fromLine: number
  toLine: number
  author: string
  body: string
  question: boolean
  replies: NoteReply[]
}

type NotesOf = (state: EditorState) => NoteMark[]
// Posts a reply to the note with this id and returns the refusal, or null.
export type SendReply = (noteId: number, body: string) => Promise<string | null>

interface NotesConfig {
  notes: NotesOf
  send: SendReply
}

// The notes live in the conflict thread outside the editor, so they are read through a function
// of the state being drawn, and redrawn when refreshNotes is dispatched or the document changes.
const notesConfig = Facet.define<NotesConfig, NotesConfig>({
  combine: (values) => values[0] ?? { notes: () => [], send: async () => 'No thread to reply in' }
})

const refresh = StateEffect.define<null>()

function linesLabel(mark: NoteMark): string {
  return mark.fromLine === mark.toLine
    ? `line ${mark.fromLine}`
    : `lines ${mark.fromLine} to ${mark.toLine}`
}

const mountedCards = new WeakMap<HTMLElement, App>()

// The card is a Vue component, so the reply box uses the design system's own controls.
class NoteWidget extends WidgetType {
  readonly mark: NoteMark
  readonly send: SendReply

  constructor(mark: NoteMark, send: SendReply) {
    super()
    this.mark = mark
    this.send = send
  }

  eq(other: NoteWidget): boolean {
    return other.send === this.send && JSON.stringify(other.mark) === JSON.stringify(this.mark)
  }

  toDOM(): HTMLElement {
    const dom = document.createElement('div')
    dom.className = 'cm-note-card'
    dom.contentEditable = 'false'
    const app = createApp(NoteCard, {
      author: this.mark.author,
      where: linesLabel(this.mark),
      body: this.mark.body,
      question: this.mark.question,
      replies: this.mark.replies,
      send: (body: string) => this.send(this.mark.id, body)
    })
    app.mount(dom)
    mountedCards.set(dom, app)
    return dom
  }

  destroy(dom: HTMLElement): void {
    mountedCards.get(dom)?.unmount()
    mountedCards.delete(dom)
  }

  // The reply box takes its own input, which the editor must not take.
  ignoreEvent(): boolean {
    return true
  }
}

function visibleMarks(state: EditorState): NoteMark[] {
  return state
    .facet(notesConfig)
    .notes(state)
    .filter((mark) => mark.fromLine >= 1 && mark.fromLine <= state.doc.lines)
    .map((mark) => ({
      ...mark,
      toLine: Math.min(Math.max(mark.toLine, mark.fromLine), state.doc.lines)
    }))
}

function build(state: EditorState): DecorationSet {
  const { send } = state.facet(notesConfig)
  const ranges = visibleMarks(state)
    .sort((a, b) => a.toLine - b.toLine || a.id - b.id)
    .map((mark) =>
      Decoration.widget({ widget: new NoteWidget(mark, send), block: true, side: 1 }).range(
        state.doc.line(mark.toLine).to
      )
    )
  return Decoration.set(ranges, true)
}

const notesField = StateField.define<DecorationSet>({
  create: build,
  update(value, tr) {
    if (tr.docChanged || isNoteChange(tr)) return build(tr.state)
    return value
  },
  provide: (field) => EditorView.decorations.from(field)
})

// Same width and sticky placement as the comment cards, measured by the comment gutter. The card
// draws itself (NoteCard.vue).
const theme = EditorView.theme({
  '.cm-note-card': {
    position: 'sticky',
    left: 'var(--cm-annotation-left, 0)',
    boxSizing: 'border-box',
    width: 'var(--cm-annotation-width, 100%)',
    margin: '2px 0 4px'
  }
})

export function noteCards(notes: NotesOf, send: SendReply): Extension {
  return [notesConfig.of({ notes, send }), notesField, theme]
}

// Redraws the cards after the notes changed outside the editor.
export function refreshNotes(view: EditorView): void {
  view.dispatch({ effects: refresh.of(null) })
}

export function isNoteChange(tr: Transaction): boolean {
  return tr.effects.some((effect) => effect.is(refresh))
}

// Lines with a note, which Focus keeps visible.
export function notedLineRanges(state: EditorState): { fromLine: number; toLine: number }[] {
  return visibleMarks(state).map(({ fromLine, toLine }) => ({ fromLine, toLine }))
}
